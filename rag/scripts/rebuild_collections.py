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
     このスクリプトは別プロセスで動かすのが確実）。止めずに動かすと、写した後の更新・削除（同意撤回・退会）が失われる
  2. python scripts/rebuild_collections.py --student-profiles            # 確認だけ（件数と今の距離関数）
     python scripts/rebuild_collections.py --student-profiles --apply    # 写して名前を入れ替える
     student_profiles__cos554 に cosine で写し、元を student_profiles__l2old554 に、写しを student_profiles に改名する
  3. rag-review を起動し、学生検索の疎通を確かめる。旧いコレクションは 7日 残し（切り戻し用）、
     そのあと --drop-student-old で消す。旧いコレクションにも退会・同意撤回した学生の埋め込みが残るので、必ず消す
  切り戻し: student_profiles を消して（または別名にして）、student_profiles__l2old554 を student_profiles に改名する。
  埋め込みは写すだけなので OpenAI は呼ばない。関連度は距離関数によらず同じ値になる（student_profile_store.relevance_score）。
"""
from __future__ import annotations

import argparse
import os
import sys
from typing import Any, Dict, List, Optional

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import vector_store as vs  # noqa: E402

BATCH = 500

STUDENT_PROFILES = "student_profiles"
STUDENT_SWAP = "student_profiles__cos554"
STUDENT_OLD = "student_profiles__l2old554"


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
    """
    existing = vs._collection_names(client)
    if STUDENT_PROFILES not in existing:
        return {"status": "missing"}
    src = client.get_collection(STUDENT_PROFILES)
    info: Dict[str, Any] = {"space": vs.collection_space(src), "count": src.count()}
    if info["space"] == "cosine":
        return {**info, "status": "already_cosine"}
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
    parser.add_argument("--apply", action="store_true", help="--student-profiles で実際に写して入れ替える（省くと確認だけ）")
    parser.add_argument("--drop-student-old", action="store_true", help="切り替え前の学生プロフィールのコレクションを消す")
    args = parser.parse_args(argv)

    client = vs.get_chroma_client()
    if args.status:
        for name, info in collection_status(client).items():
            print(f"{name}: space={info['space']} count={info['count']}")
        return 0
    if args.student_profiles:
        print("student_profiles:", rebuild_student_profiles(client, apply=args.apply))
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
