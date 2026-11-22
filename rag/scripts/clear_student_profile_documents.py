"""既存の学生プロフィールの本文を Chroma から消す単発スクリプト（SOCAIAGENT-695）。

student_profiles コレクションを、本文を持たない新しいコレクションへ差し替える。
id・埋め込み・メタデータはそのまま写すので、検索結果は変わらない。OpenAI は呼ばない。

update で本文を空にするだけでは足りない。chromadb 0.6.3 は書き込みを SQLite の embeddings_queue に
本文ごと積み、ベクトル側が書き出されるまで消さない（既定で 1000 件ごと）。そこで次の順に処理する。
  1. 本文なしの複製（student_profiles__swap695）を作る。
  2. 元のコレクションの本文を update で空にする。0.6.3 の delete_collection はメタデータの行を
     消さずに残すため、先に本文を空にしておかないと削除後も本文が SQLite に残る。
  3. 元を student_profiles__old695 へ改名 → 複製を student_profiles へ改名 → 元を削除する。
     コレクションを削除するとキューの行も消える。
何度実行してもよい。途中で失敗したら、ログの指示に従ってから再実行する。

実行前後の手順（すべて必須）:
  1. 学生プロフィールの書き込みを止める。/student-search/index と DELETE /student-search/{user_id} の
     両方（rag-review を止め、このスクリプトは別プロセスで動かすのが確実）。
     止めずに動かすと、複製した後の更新は失われ、複製した後の削除（同意撤回・退会）は
     新しいコレクションで復活する。止められなかった場合は、実行中に更新・削除した学生について
     index と削除をやり直す。
  2. Chroma のスナップショットを取る（chroma_data ボリューム、または RAG_CHROMA_DATA_DIR の
     バックアップ）。スナップショット自体に本文が入っているので、確認が済んだら消す。
  3. --dry-run（既定）で件数を確かめる。本文は出力しない。total=0 なら接続先を見直す（異常終了する）。
  4. --apply を実行する。
  5. Chroma を止めて SQLite を VACUUM する（例: chroma のデータディレクトリに対して
     `chroma utils vacuum --path <dir> --force`）。行を消しても、本文のバイトは
     VACUUM するまで chroma.sqlite3 の空き領域に残る。
  6. Chroma と rag-review を起動し、検索の疎通（make rag-smoke）を確かめる。
  staging で 1〜6 を確かめてから本番。

使い方（rag/ で。接続先は rag-review と同じ CHROMA_HOST / CHROMA_PORT / RAG_CHROMA_DATA_DIR）:
  python scripts/clear_student_profile_documents.py            # dry-run: 件数だけ数える
  python scripts/clear_student_profile_documents.py --apply    # 本文のないコレクションへ差し替える
"""
from __future__ import annotations

import argparse
import logging
import os
import sys
import time
from typing import Any, Dict, List

logger = logging.getLogger("clear-student-profile-documents")

BATCH = 500
COLLECTION = "student_profiles"
SWAP_SUFFIX = "__swap695"
OLD_SUFFIX = "__old695"


def _all_ids(collection: Any, batch: int) -> List[str]:
    ids: List[str] = []
    while True:
        page = list(collection.get(include=[], limit=batch, offset=len(ids)).get("ids") or [])
        if not page:
            return ids
        ids.extend(page)


def _rename_into_place(client: Any, collection: Any, name: str) -> None:
    try:
        collection.modify(name=name)
    except Exception:
        # 改名の隙間に検索 API の get_or_create が空の同名コレクションを作った場合だけ、消してやり直す。
        stray = client.get_collection(name)
        if stray.count() != 0:
            raise
        client.delete_collection(name)
        collection.modify(name=name)


def clear_documents(client: Any, name: str = COLLECTION, dry_run: bool = True, batch: int = BATCH) -> Dict[str, int]:
    """name のコレクションを本文のない複製へ差し替え、件数（total / with_text / cleared）を返す。"""
    swap_name, old_name = name + SWAP_SUFFIX, name + OLD_SUFFIX
    existing = set(client.list_collections())
    if old_name in existing:
        raise RuntimeError(
            f"{old_name} が残っている（前回の差し替えが途中で止まった）。{name} の件数を確かめ、"
            f"揃っていれば {old_name} を削除、空なら {old_name} を {name} へ戻してから再実行する"
        )
    src = client.get_collection(name)
    ids = _all_ids(src, batch)

    with_text = 0
    for start in range(0, len(ids), batch):
        docs = src.get(ids=ids[start:start + batch], include=["documents"])["documents"]
        with_text += sum(1 for d in docs if d)
    result = {"total": len(ids), "with_text": with_text, "cleared": 0}
    if dry_run or not ids:
        return result

    if swap_name in existing:
        client.delete_collection(swap_name)  # 前回途中で止まった複製。本文は入っていない
    dst = client.create_collection(swap_name, metadata=src.metadata)
    for start in range(0, len(ids), batch):
        got = src.get(ids=ids[start:start + batch], include=["documents", "embeddings", "metadatas"])
        embeddings = [list(e) for e in got["embeddings"]]
        dst.add(ids=got["ids"], embeddings=embeddings, metadatas=got["metadatas"], documents=[""] * len(got["ids"]))
        # documents だけを渡すと既定の埋め込み関数で計算し直そうとするため、今の埋め込みも渡す。
        src.update(ids=got["ids"], documents=[""] * len(got["ids"]), embeddings=embeddings)
    if dst.count() != len(ids):
        raise RuntimeError(f"複製の件数が合わない: {dst.count()} != {len(ids)}。{name} は差し替えていない")

    src.modify(name=old_name)
    _rename_into_place(client, dst, name)
    client.delete_collection(old_name)
    result["cleared"] = with_text
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--apply", action="store_true", help="本文のないコレクションへ差し替える（省略時は dry-run）")
    parser.add_argument("--dry-run", action="store_true", help="件数だけ数える（既定）")
    args = parser.parse_args()
    if args.apply and args.dry_run:
        parser.error("--apply と --dry-run は同時に指定できない")
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
    from vector_store import describe_backend, get_chroma_client  # noqa: E402

    client = get_chroma_client()
    if COLLECTION not in set(client.list_collections()):
        logger.error("%s が %s に無い。接続先（CHROMA_HOST / RAG_CHROMA_DATA_DIR）を確かめる", COLLECTION, describe_backend())
        return 2

    started = time.monotonic()
    result = clear_documents(client, dry_run=not args.apply)
    logger.info(
        "%s %s total=%d with_text=%d cleared=%d elapsed=%.1fs",
        COLLECTION, "apply" if args.apply else "dry-run",
        result["total"], result["with_text"], result["cleared"], time.monotonic() - started,
    )
    if result["total"] == 0:
        logger.error("%s が空（%s）。接続先が違う可能性がある", COLLECTION, describe_backend())
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
