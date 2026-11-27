"""企業データのコレクションを新しい世代へ作り直す（SOCAIAGENT-658）。

全件の再構築は、今のコレクションを消して作り直すと、その間は検索できず、途中で失敗すると空になる。
新しい世代（接尾辞つきの名前）へ文書・埋め込み・メタデータを写してから、RAG_COLLECTION_SUFFIX を
切り替えて使う。写している間も、今の世代で検索できる。埋め込みは写すだけなので OpenAI は呼ばない。

手順:
  1. python scripts/rebuild_collections.py --to-suffix g2 [--space cosine]
     （--from-suffix を省くと、今の RAG_COLLECTION_SUFFIX の世代から写す）
  2. 件数を確かめ、rag-review の RAG_COLLECTION_SUFFIX=g2 にして入れ替える
  3. 旧世代は 7日 残し（切り戻し用）、そのあと --drop-suffix <旧> で消す。接尾辞なしの世代は --drop-suffix ""

--space は新しい世代の距離関数（hnsw:space）。既定は cosine（SOCAIAGENT-554。vector_store.COLLECTION_METADATA と同じ）。
コレクションは作ったあとで距離関数を変えられないので、cosine にするときはこの再構築で作り直す。
写し先が別の距離関数で既にあるときは止める（Chroma は既存のコレクションへの距離関数の指定を黙って無視するため）。

--status は、企業データ（今の世代）と学生プロフィールのコレクションの距離関数と件数を出す（書き込まない）。

学生プロフィール（student_profiles）は世代の接尾辞を持たないので、名前を入れ替えて cosine にする（SOCAIAGENT-554）。
  1. 学生プロフィールの書き込みを止める（/student-search/index と DELETE /student-search/{user_id}。rag-review を止め、
     このスクリプトは別プロセスで動かすのが確実）。止めずに動かすと、写した後の更新・削除（同意撤回・退会）が失われる。
     rag-review を止めると企業検索・ES 添削なども止まるので、利用の少ない時間帯に行う
  2. python scripts/rebuild_collections.py --student-profiles            # 確認だけ（件数と今の距離関数）
     python scripts/rebuild_collections.py --student-profiles --apply    # 写して名前を入れ替える
     student_profiles__cos554 に cosine で写し、元を student_profiles__l2old554 に、写しを student_profiles に改名する
  3. rag-review を起動し、学生検索の疎通を確かめる
  4. 止めていた間の削除は届いていない（Backend はやり直さない）。DB から今公開してよい学生の user_id を出し、
     --prune-students <ファイル> で確かめてから --apply で消す（今と旧いコレクションの両方）
  5. 旧いコレクションは 7日 残し（切り戻し用）、そのあと --drop-student-old で消す。
     旧いコレクションにも退会・同意撤回した学生の埋め込みが残るので、必ず消す
  切り戻し: student_profiles を消して（または別名にして）、student_profiles__l2old554 を student_profiles に改名する。
  旧いコレクションを残している間の削除（同意撤回・退会）は旧いほうにも反映する（student_profile_store.delete_student）。
  更新（index）は反映しないので、切り替えのあとに保存した学生は、切り戻すと切り替え前の内容で検索される
  （Backend に index し直す経路は無く、本人が次に保存したときに直る）。切り戻したら --prune-students も流す。
  改名の途中で止まったとき（student_profiles が無い・空のまま）は、もう一度 --student-profiles --apply を動かす。
  写しがそろっていれば再開し、そろっていなければ元に戻す（--apply なしなら、どちらをするかだけを出す）。
  埋め込みは写すだけなので OpenAI は呼ばない。関連度は距離関数によらず同じ値になる（student_profile_store.relevance_score）。
"""
from __future__ import annotations

import argparse
import os
import sys
from typing import Any, Dict, List, Optional, Set

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import student_profile_store as sps  # noqa: E402
import vector_store as vs  # noqa: E402

BATCH = 500

STUDENT_PROFILES = sps.COLLECTION_STUDENT_PROFILES
STUDENT_SWAP = "student_profiles__cos554"
STUDENT_OLD = sps.COLLECTION_STUDENT_PROFILES_OLD


def _copy_all(src: Any, dst: Any) -> int:
    """src の全件（id・本文・埋め込み・メタデータ）を dst へ写し、件数を返す。"""
    total = 0
    offset = 0
    while True:
        got = src.get(include=["documents", "embeddings", "metadatas"], limit=BATCH, offset=offset)
        ids: List[str] = list(got.get("ids") or [])
        if not ids:
            break
        dst.upsert(ids=ids, documents=got["documents"], embeddings=got["embeddings"], metadatas=got["metadatas"])
        total += len(ids)
        offset += len(ids)
    return total


def _open_with_space(client: Any, name: str, space: str) -> Any:
    """name のコレクションを space で開く（無ければ作る）。既に別の距離関数であれば止める。"""
    col = client.get_or_create_collection(name, metadata={"hnsw:space": space})
    actual = vs.collection_space(col)
    if actual != space:
        raise RuntimeError(f"{name} は既に {actual} で存在する（{space} にできない）。消してからやり直す")
    return col


def copy_collections(client: Any, from_suffix: str, to_suffix: str, space: str = "cosine") -> Dict[str, int]:
    """3つの企業データのコレクションを from の世代から to の世代へ写し、論理名ごとの件数を返す。"""
    if from_suffix == to_suffix:
        raise ValueError("写し元と写し先の世代が同じ")
    existing = vs._collection_names(client)
    copied: Dict[str, int] = {}
    for logical in vs.ALL_COLLECTIONS:
        src_name = vs.physical_collection_name(logical, from_suffix)
        dst_name = vs.physical_collection_name(logical, to_suffix)
        dst = _open_with_space(client, dst_name, space)
        if src_name not in existing:
            copied[logical] = 0
            continue
        total = _copy_all(client.get_collection(src_name), dst)
        if dst.count() < total:
            raise RuntimeError(f"{dst_name}: 写した件数が足りない（{dst.count()} < {total}）")
        copied[logical] = total
    return copied


def drop_generation(client: Any, suffix: str) -> List[str]:
    """ある世代の企業データのコレクションを消す。今使っている世代は消さない。"""
    if suffix == vs.collection_suffix():
        raise ValueError("今使っている世代（RAG_COLLECTION_SUFFIX）は消せない")
    existing = vs._collection_names(client)
    dropped = []
    for logical in vs.ALL_COLLECTIONS:
        name = vs.physical_collection_name(logical, suffix)
        if name in existing:
            client.delete_collection(name)
            dropped.append(name)
    return dropped


def collection_status(client: Any) -> Dict[str, Dict[str, Any]]:
    """企業データ（今の世代）と学生プロフィールのコレクションの距離関数と件数。無いものは載せない。"""
    existing = vs._collection_names(client)
    names = [vs.physical_collection_name(logical) for logical in vs.ALL_COLLECTIONS] + [STUDENT_PROFILES, STUDENT_OLD]
    return {
        name: {"space": vs.collection_space(col), "count": col.count()}
        for name in names
        if name in existing
        for col in [client.get_collection(name)]
    }


def rebuild_student_profiles(client: Any, apply: bool = False) -> Dict[str, Any]:
    """学生プロフィールのコレクションを cosine の新しいコレクションへ写し、名前を入れ替える。

    apply=False なら件数と今の距離関数だけを返す。既に cosine なら何もしない。
    学生プロフィールの書き込みを止めてから動かすこと（モジュールの説明を参照）。
    前回の改名の途中で止まっていたら、まずそれを直す（_recover_interrupted）。
    """
    existing = vs._collection_names(client)
    recovered = _recover_interrupted(client, existing, apply)
    if recovered is not None:
        return recovered
    if STUDENT_PROFILES not in existing:
        return {"status": "missing"}
    src = client.get_collection(STUDENT_PROFILES)
    info: Dict[str, Any] = {"space": vs.collection_space(src), "count": src.count()}
    if info["space"] == "cosine":
        return {**info, "status": "already_cosine", "old_left": STUDENT_OLD in existing}
    if STUDENT_OLD in existing:
        raise RuntimeError(f"{STUDENT_OLD} が残っている。前回の切り替えを確かめ、--drop-student-old で消してからやり直す")
    if not apply:
        return {**info, "status": "dry_run"}
    if STUDENT_SWAP in existing:
        # 前回の写しの途中で止まった。写しは元からいつでも作り直せるので消す
        client.delete_collection(STUDENT_SWAP)
    dst = _open_with_space(client, STUDENT_SWAP, "cosine")
    total = _copy_all(src, dst)
    if dst.count() < total or total != info["count"]:
        raise RuntimeError(f"{STUDENT_SWAP}: 写した件数が合わない（{dst.count()} / {total} / 元 {info['count']}）。元はそのまま")
    src.modify(name=STUDENT_OLD)
    try:
        dst.modify(name=STUDENT_PROFILES)
    except Exception:
        # 写しを student_profiles にできなければ、元の名前に戻す（学生検索が空のコレクションを作らないように）
        src.modify(name=STUDENT_PROFILES)
        raise
    return {**info, "status": "swapped", "copied": total, "old": STUDENT_OLD}


def _recover_interrupted(client: Any, existing: set, apply: bool) -> Optional[Dict[str, Any]]:
    """前回の改名の途中で止まった状態を見つけて直す。止まっていなければ None。

    止まった状態: 元を student_profiles__l2old554 に改名した後、写しを student_profiles に改名する前に
    プロセスが終わった・接続が切れた。student_profiles が無いか、その後に rag-review が空の student_profiles を
    作っている（学生検索は空のまま）。
    - 写し（student_profiles__cos554）が cosine で元と同じ件数そろっていれば、写しを student_profiles にする（再開）
    - そろっていなければ、元を student_profiles に戻す（切り戻し。もう一度 --apply すれば写し直す）
    apply=False なら、どちらをするかだけを返す。空の student_profiles は件数 0 のときだけ消す。
    """
    if STUDENT_OLD not in existing:
        return None
    old = client.get_collection(STUDENT_OLD)
    old_count = old.count()
    current = client.get_collection(STUDENT_PROFILES) if STUDENT_PROFILES in existing else None
    if current is not None and not (current.count() == 0 and old_count > 0):
        return None  # 切り替えは済んでいる（旧いコレクションを残している期間）
    swap = client.get_collection(STUDENT_SWAP) if STUDENT_SWAP in existing else None
    resume = swap is not None and vs.collection_space(swap) == "cosine" and swap.count() == old_count
    action = "resume" if resume else "rollback"
    if not apply:
        return {"status": "interrupted", "action": action, "old_count": old_count}
    if current is not None:
        client.delete_collection(STUDENT_PROFILES)  # 中断のあとに作られた空のコレクション
    if resume:
        swap.modify(name=STUDENT_PROFILES)
        return {"status": "resumed", "count": old_count, "old": STUDENT_OLD}
    old.modify(name=STUDENT_PROFILES)
    return {"status": "rolled_back", "count": old_count}


def _all_ids(col: Any) -> List[str]:
    """コレクションの全件の id。"""
    ids: List[str] = []
    offset = 0
    while True:
        got = list(col.get(include=[], limit=BATCH, offset=offset).get("ids") or [])
        if not got:
            return ids
        ids.extend(got)
        offset += len(got)


def prune_students(client: Any, keep_user_ids: Set[int], apply: bool = False) -> Dict[str, Any]:
    """公開してよい学生（keep_user_ids）以外の埋め込みを、学生プロフィールの今と旧いコレクションから消す。

    rag-review を止めている間に Backend から来た削除（公開の同意を外した・退会）は届かず、Backend も
    やり直さない（StudentIndexSyncer.Sync は WARN のログだけ。日次の EnsureDeleted は退会者だけ）。
    そのため切り替えのあとと切り戻しのあとに、DB の「今公開してよい学生」に合わせて消す。
    apply=False なら消す件数だけを返す。keep_user_ids が空なら全件消すことになるので止める。
    """
    if not keep_user_ids:
        raise ValueError("公開してよい学生の一覧が空。全件消すことになるので止める（一覧の出力を確かめる）")
    keep = {sps._doc_id(u) for u in keep_user_ids}
    existing = vs._collection_names(client)
    out: Dict[str, Any] = {"apply": apply}
    for name in (STUDENT_PROFILES, STUDENT_OLD):
        if name not in existing:
            continue
        col = client.get_collection(name)
        stale = [i for i in _all_ids(col) if i not in keep]
        if apply:
            for start in range(0, len(stale), BATCH):
                col.delete(ids=stale[start:start + BATCH])
        out[name] = len(stale)
    return out


def _read_user_ids(path: str) -> Set[int]:
    """1行に1つの user_id（空行と # で始まる行は飛ばす）。"""
    ids: Set[int] = set()
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#"):
                ids.add(int(line))
    return ids


def drop_student_old(client: Any) -> List[str]:
    """切り替え前の学生プロフィールのコレクション（l2）を消す。"""
    if STUDENT_OLD not in vs._collection_names(client):
        return []
    client.delete_collection(STUDENT_OLD)
    return [STUDENT_OLD]


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--to-suffix", help="写し先の世代の接尾辞（英数字・_・-）")
    parser.add_argument("--from-suffix", default=None, help="写し元の世代。省略時は今の RAG_COLLECTION_SUFFIX")
    parser.add_argument("--space", choices=["l2", "cosine", "ip"], default="cosine", help="新しい世代の距離関数（既定 cosine）")
    parser.add_argument("--drop-suffix", default=None, help="この世代のコレクションを消す（写しはしない）")
    parser.add_argument("--status", action="store_true", help="コレクションの距離関数と件数を出す（書き込まない）")
    parser.add_argument("--student-profiles", action="store_true", help="学生プロフィールを cosine のコレクションへ入れ替える")
    parser.add_argument("--apply", action="store_true", help="--student-profiles・--prune-students で実際に書き込む（省くと確認だけ）")
    parser.add_argument("--drop-student-old", action="store_true", help="切り替え前の学生プロフィールのコレクションを消す")
    parser.add_argument(
        "--prune-students",
        metavar="FILE",
        help="公開してよい学生の user_id の一覧（1行1つ）に無い学生を、学生プロフィールから消す（--apply を付けないと件数だけ）",
    )
    args = parser.parse_args(argv)

    client = vs.get_chroma_client()
    if args.status:
        for name, info in collection_status(client).items():
            print(f"{name}: space={info['space']} count={info['count']}")
        return 0
    if args.student_profiles:
        print("student_profiles:", rebuild_student_profiles(client, apply=args.apply))
        return 0
    if args.prune_students:
        print("pruned:", prune_students(client, _read_user_ids(args.prune_students), apply=args.apply))
        return 0
    if args.drop_student_old:
        print("dropped:", drop_student_old(client))
        return 0
    if args.drop_suffix is not None:
        print("dropped:", drop_generation(client, args.drop_suffix))
        return 0
    if not args.to_suffix:
        parser.error("--to-suffix か --drop-suffix を指定する")
    from_suffix = vs.collection_suffix() if args.from_suffix is None else args.from_suffix
    copied = copy_collections(client, from_suffix, args.to_suffix, args.space)
    print(f"copied from={from_suffix or '(なし)'} to={args.to_suffix}: {copied}")
    print(f"件数を確かめてから RAG_COLLECTION_SUFFIX={args.to_suffix} に切り替える。旧世代は7日残す。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
