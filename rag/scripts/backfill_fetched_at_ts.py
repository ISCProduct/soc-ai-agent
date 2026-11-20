"""既存の文書に fetched_at_ts（取得時刻の数値）を補う（SOCAIAGENT-659）。

検索では TTL を where の fetched_at_ts で絞るようになったので、fetched_at_ts を持たない既存の文書は
古い扱いになり、検索に当たらない。デプロイの前にこのスクリプトで補っておく。

- fetched_at（ISO 8601）が読めて、ありえない未来でない文書だけに fetched_at_ts を足す
- fetched_at が無い・読めない文書には足さない（古い扱いのまま。次の検索で取り直されて上書きされる。U1）
- 既に数値の fetched_at_ts がある文書には触らない（何度流しても同じ結果になる）
- 既定は dry-run（件数を出すだけ）。書き込むときは --apply を付ける

手順:
  1. python scripts/backfill_fetched_at_ts.py            # 件数を確かめる
  2. python scripts/backfill_fetched_at_ts.py --apply    # 書き込む
  3. rag-review をデプロイする（切り戻しは RAG_TTL_WHERE=0）
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import vector_store as vs  # noqa: E402

BATCH = 500


def fetched_at_to_ts(fetched_at: Any, now: Optional[datetime] = None) -> Optional[int]:
    """fetched_at を UNIX 秒にする。読めない・ありえない未来のときは None（補わない）。"""
    if not fetched_at:
        return None
    try:
        ts = datetime.fromisoformat(str(fetched_at).replace("Z", "+00:00"))
        if ts.tzinfo is None:
            ts = ts.replace(tzinfo=timezone.utc)
        value = ts.timestamp()
    except (ValueError, TypeError, OverflowError, OSError):
        return None
    now = now or datetime.now(timezone.utc)
    if value - now.timestamp() > vs._FETCHED_AT_FUTURE_TOLERANCE_SECONDS:
        return None
    return int(value)


def _has_ts(meta: Dict[str, Any]) -> bool:
    value = meta.get("fetched_at_ts")
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def backfill_collection(collection: Any, apply: bool = False) -> Dict[str, int]:
    """1つのコレクションを補い、件数（scanned / already / filled / unreadable）を返す。"""
    stats = {"scanned": 0, "already": 0, "filled": 0, "unreadable": 0}
    offset = 0
    while True:
        got = collection.get(include=["metadatas"], limit=BATCH, offset=offset)
        ids: List[str] = list(got.get("ids") or [])
        if not ids:
            break
        metadatas = got.get("metadatas") or [{} for _ in ids]
        update_ids: List[str] = []
        update_metas: List[Dict[str, Any]] = []
        for doc_id, meta in zip(ids, metadatas):
            meta = dict(meta or {})
            stats["scanned"] += 1
            if _has_ts(meta):
                stats["already"] += 1
                continue
            ts = fetched_at_to_ts(meta.get("fetched_at"))
            if ts is None:
                stats["unreadable"] += 1
                continue
            meta["fetched_at_ts"] = ts
            update_ids.append(doc_id)
            # update はメタデータを丸ごと置き換える実装もあるので、既存の項目ごと渡す
            update_metas.append(meta)
        if update_ids:
            stats["filled"] += len(update_ids)
            if apply:
                collection.update(ids=update_ids, metadatas=update_metas)
        # 件数は増減しない（update だけ）ので、offset で順に読み進めてよい
        offset += len(ids)
    return stats


def backfill(client: Any, apply: bool = False) -> Dict[str, Dict[str, int]]:
    """今の世代（RAG_COLLECTION_SUFFIX）の企業データのコレクションをすべて補う。"""
    existing = vs._collection_names(client)
    result: Dict[str, Dict[str, int]] = {}
    for logical in vs.ALL_COLLECTIONS:
        physical = vs.physical_collection_name(logical)
        if physical not in existing:
            result[logical] = {"scanned": 0, "already": 0, "filled": 0, "unreadable": 0}
            continue
        result[logical] = backfill_collection(client.get_collection(physical), apply=apply)
    return result


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description="既存の文書に fetched_at_ts を補う（既定は dry-run）")
    parser.add_argument("--apply", action="store_true", help="実際に書き込む（付けなければ件数を出すだけ）")
    args = parser.parse_args(argv)
    result = backfill(vs.get_chroma_client(), apply=args.apply)
    print(json.dumps({"apply": args.apply, "collections": result}, ensure_ascii=False, indent=2))
    if not args.apply:
        print("dry-run です。書き込むときは --apply を付けてください。", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
