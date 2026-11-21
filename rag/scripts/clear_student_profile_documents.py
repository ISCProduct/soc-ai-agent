"""既存の学生プロフィールの本文だけを空にする単発スクリプト（SOCAIAGENT-695）。

student_profiles コレクションの documents を空文字にする。埋め込みとメタデータは変えない。
chromadb 0.6 の update は documents だけを渡すと既定の埋め込み関数で埋め込みを計算し直そうとする
（次元が違えばエラー、同じなら黙って置き換わる）。そのため今の埋め込みを読み出して一緒に渡す。
OpenAI は呼ばない。何度実行しても同じ結果になり、途中で失敗したらそのまま再実行してよい。

実行前の手順（必須）:
  1. Chroma のスナップショットを取る（chroma_data ボリューム、または CHROMA_DATA_DIR のバックアップ）。
     本文は消すと戻らない。戻す手段はこのスナップショットか、学生プロフィールの再 index だけ。
  2. --dry-run（既定）で件数を確かめる。本文は出力しない。
  3. staging で --apply → 検索の疎通（make rag-smoke）を確かめてから本番。

使い方（rag/ で。接続先は rag-review と同じ CHROMA_HOST / CHROMA_PORT / CHROMA_DATA_DIR）:
  python scripts/clear_student_profile_documents.py            # dry-run: 件数だけ数える
  python scripts/clear_student_profile_documents.py --apply    # 本文を空にする
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


def clear_documents(collection: Any, dry_run: bool = True, batch: int = BATCH) -> Dict[str, int]:
    """本文のある行だけ documents を空にし、件数（total / with_text / cleared）を返す。"""
    # 更新中に offset の並びが変わっても取りこぼさないよう、先に ID を全部集めてから処理する。
    ids: List[str] = []
    while True:
        got = collection.get(include=[], limit=batch, offset=len(ids))
        page = list(got.get("ids") or [])
        if not page:
            break
        ids.extend(page)

    with_text = 0
    cleared = 0
    for start in range(0, len(ids), batch):
        chunk = ids[start:start + batch]
        got = collection.get(ids=chunk, include=["documents", "embeddings"])
        targets = [
            (i, emb)
            for i, doc, emb in zip(got["ids"], got["documents"], got["embeddings"])
            if doc
        ]
        with_text += len(targets)
        if dry_run or not targets:
            continue
        collection.update(
            ids=[i for i, _ in targets],
            documents=[""] * len(targets),
            embeddings=[list(emb) for _, emb in targets],
        )
        cleared += len(targets)
    return {"total": len(ids), "with_text": with_text, "cleared": cleared}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--apply", action="store_true", help="本文を実際に空にする（省略時は dry-run）")
    parser.add_argument("--dry-run", action="store_true", help="件数だけ数える（既定）")
    args = parser.parse_args()
    if args.apply and args.dry_run:
        parser.error("--apply と --dry-run は同時に指定できない")
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
    from student_profile_store import _collection  # noqa: E402

    started = time.monotonic()
    result = clear_documents(_collection(), dry_run=not args.apply)
    logger.info(
        "student_profiles %s total=%d with_text=%d cleared=%d elapsed=%.1fs",
        "apply" if args.apply else "dry-run",
        result["total"], result["with_text"], result["cleared"], time.monotonic() - started,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
