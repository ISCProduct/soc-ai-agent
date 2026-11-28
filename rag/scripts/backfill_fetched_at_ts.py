"""既存の文書に fetched_at_ts（取得時刻の数値）を補う、手動用の CLI（SOCAIAGENT-659）。

ふだんは rag-review が起動のたびにバックグラウンドで補う（main.start_fetched_at_ts_backfill）ので、
デプロイで手順は要らない。このスクリプトは、件数を確かめたいときや、起動時の backfill を
止めている（RAG_BACKFILL_ON_STARTUP=0）ときに使う。補い方は vector_store.backfill_fetched_at_ts を参照。

- 既定は dry-run（件数を出すだけ）。書き込むときは --apply を付ける
- unreadable は fetched_at が無い・読めない・ありえない未来の文書の数（補わず、古い扱いのまま）

  python scripts/backfill_fetched_at_ts.py            # 件数を確かめる
  python scripts/backfill_fetched_at_ts.py --apply    # 書き込む
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from typing import List, Optional

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import vector_store as vs  # noqa: E402


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description="既存の文書に fetched_at_ts を補う（既定は dry-run）")
    parser.add_argument("--apply", action="store_true", help="実際に書き込む（付けなければ件数を出すだけ）")
    args = parser.parse_args(argv)
    result = vs.backfill_fetched_at_ts(vs.get_chroma_client(), apply=args.apply)
    print(json.dumps({"apply": args.apply, "collections": result}, ensure_ascii=False, indent=2))
    if not args.apply:
        print("dry-run です。書き込むときは --apply を付けてください。", file=sys.stderr)
    return 1 if vs.backfill_failed_collections(result) else 0


if __name__ == "__main__":
    raise SystemExit(main())
