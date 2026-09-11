#!/usr/bin/env python3
"""旧 PersistentClient (/app/chroma_db) → 独立 Chroma HttpClient への移行 (#585)。

使い方（ホストから）:
  # 1) 旧データを退避済みであること（例: /tmp/soc-ai-chroma-backup/chroma_db）
  # 2) chroma + rag-review が起動していること（make rag-up）
  cd rag && .venv/bin/python scripts/migrate_persistent_to_http.py \\
    --source /tmp/soc-ai-chroma-backup/chroma_db \\
    --host 127.0.0.1 --port 8000 --dry-run

  # 本番投入
  ... --dry-run を外す

ロールバック:
  rag-review の CHROMA_HOST を空にし、RAG_CHROMA_DATA_DIR に旧 path をマウントして再起動。
"""
from __future__ import annotations

import argparse
import re
import sys
from typing import Any, Dict, List

import chromadb


NEW_COLLECTIONS = ("company_context", "interview_hints", "es_review")


def _names(client: Any) -> List[str]:
    raw = client.list_collections()
    out: List[str] = []
    for c in raw:
        if isinstance(c, str):
            out.append(c)
        else:
            name = getattr(c, "name", "") or ""
            if name:
                out.append(name)
    return out


def map_target_collection(src_name: str) -> str:
    n = src_name.lower()
    if "hint" in n:
        return "interview_hints"
    if "es_review" in n or n.endswith("es_review"):
        return "es_review"
    if src_name in NEW_COLLECTIONS:
        return src_name
    return "company_context"


def sanitize_company(company_name: str) -> str:
    """rag/main.py の _sanitize_company_name_for_query と同等。"""
    sanitized = re.sub(r"[^0-9A-Za-zぁ-んァ-ン一-龥ー々〆ヵヶ・\s]", "", company_name)
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    return sanitized or "unknown"


def infer_company(src_name: str, metadata: Dict[str, Any] | None) -> str:
    if metadata and metadata.get("company"):
        return sanitize_company(str(metadata["company"]))
    # 旧命名: hints_______39722 / 39722_______es_review など
    digits = re.findall(r"\d{4,}", src_name)
    if digits:
        return sanitize_company(f"legacy_id_{digits[-1]}")
    cleaned = re.sub(r"[^0-9A-Za-z一-龥ぁ-んァ-ヶー]+", "_", src_name).strip("_")
    return sanitize_company(cleaned or "unknown")


def migrate(source: str, host: str, port: int, dry_run: bool, limit_per_col: int) -> int:
    src = chromadb.PersistentClient(path=source)
    dst = chromadb.HttpClient(host=host, port=port)
    src_names = _names(src)
    print(f"source collections={len(src_names)} path={source}")
    print(f"dest http://{host}:{port}")

    moved = 0
    mapped: Dict[str, int] = {k: 0 for k in NEW_COLLECTIONS}
    skipped_no_emb = 0
    for name in src_names:
        target = map_target_collection(name)
        scol = src.get_collection(name)
        try:
            count = scol.count()
        except Exception as exc:
            print(f"  skip {name}: count failed {exc}")
            continue
        if count == 0:
            continue
        print(f"  {name} ({count}) -> {target}")
        if dry_run:
            moved += count
            mapped[target] = mapped.get(target, 0) + count
            continue
        # 全件取得（中規模想定）。巨大な場合は limit で分割。
        got = scol.get(include=["documents", "metadatas", "embeddings"])
        ids = got.get("ids") or []
        docs = got.get("documents") or []
        metas = got.get("metadatas") or []
        embs = got.get("embeddings")
        if embs is None:
            print(f"  skip {name}: no embeddings")
            skipped_no_emb += 1
            continue
        dcol = dst.get_or_create_collection(target)
        batch_ids: List[str] = []
        batch_docs: List[str] = []
        batch_metas: List[Dict[str, Any]] = []
        batch_embs: List[List[float]] = []
        for i, doc_id in enumerate(ids):
            if limit_per_col > 0 and i >= limit_per_col:
                break
            doc = docs[i] if i < len(docs) else None
            if not doc:
                continue
            md = dict(metas[i] or {}) if i < len(metas) else {}
            md.setdefault("company", infer_company(name, md))
            md.setdefault("role", md.get("role") or "general")
            md.setdefault("doc_type", target if target != "company_context" else "company_research")
            md.setdefault("source", md.get("source") or "migrate_persistent")
            md["migrated_from"] = name
            emb = embs[i] if i < len(embs) else None
            if emb is None:
                continue
            new_id = f"mig_{name}_{doc_id}"[:64]
            batch_ids.append(new_id)
            batch_docs.append(doc)
            batch_metas.append(md)
            batch_embs.append(list(emb))
        if not batch_ids:
            continue
        # chroma upsert は大きすぎると失敗しうるので分割
        chunk = 50
        for start in range(0, len(batch_ids), chunk):
            dcol.upsert(
                ids=batch_ids[start : start + chunk],
                documents=batch_docs[start : start + chunk],
                metadatas=batch_metas[start : start + chunk],
                embeddings=batch_embs[start : start + chunk],
            )
        moved += len(batch_ids)
        mapped[target] = mapped.get(target, 0) + len(batch_ids)
        print(f"    upserted {len(batch_ids)}")
    print(f"done moved_docs={moved} dry_run={dry_run} mapped={mapped} skipped_no_emb={skipped_no_emb}")
    return 0


def main(argv: List[str]) -> int:
    p = argparse.ArgumentParser(description="Migrate Persistent Chroma to Http Chroma (#585)")
    p.add_argument("--source", required=True, help="PersistentClient path")
    p.add_argument("--host", default="127.0.0.1")
    p.add_argument("--port", type=int, default=8000)
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--limit-per-collection", type=int, default=0, help="0=all")
    args = p.parse_args(argv)
    return migrate(args.source, args.host, args.port, args.dry_run, args.limit_per_collection)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
