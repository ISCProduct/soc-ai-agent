"""ベクトルストア（Chroma）クライアントと企業単位コレクション操作。

#573: 本番では CHROMA_HOST 経由の HttpClient、未設定時は PersistentClient にフォールバックする。
"""
from __future__ import annotations

import hashlib
import logging
import os
import re
import threading
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple

import chromadb

logger = logging.getLogger("rag-review")

DEFAULT_CHROMA_DATA_DIR = "/app/chroma_db"
CHROMA_DATA_DIR = os.getenv("RAG_CHROMA_DATA_DIR", DEFAULT_CHROMA_DATA_DIR)
CHROMA_HOST = os.getenv("CHROMA_HOST", "").strip()
CHROMA_PORT = int(os.getenv("CHROMA_PORT", "8000"))
CACHE_TTL_SECONDS = int(os.getenv("RAG_SEARCH_CACHE_TTL_SECONDS", "86400"))

COLLECTION_COMPANY_CONTEXT = "company_context"
COLLECTION_INTERVIEW_HINTS = "interview_hints"
COLLECTION_ES_REVIEW = "es_review"

_chroma_client: Any = None
_chroma_lock = threading.Lock()


def reset_chroma_client_for_tests() -> None:
    """テスト用にシングルトンをクリアする。"""
    global _chroma_client
    with _chroma_lock:
        _chroma_client = None


def get_chroma_client() -> Any:
    """Chroma クライアントを返す。CHROMA_HOST があれば HttpClient。"""
    global _chroma_client
    if _chroma_client is None:
        with _chroma_lock:
            if _chroma_client is None:
                if CHROMA_HOST:
                    logger.info(
                        "chroma mode=http host=%s port=%d", CHROMA_HOST, CHROMA_PORT
                    )
                    _chroma_client = chromadb.HttpClient(host=CHROMA_HOST, port=CHROMA_PORT)
                else:
                    logger.info("chroma mode=persistent path=%s", CHROMA_DATA_DIR)
                    _chroma_client = chromadb.PersistentClient(path=CHROMA_DATA_DIR)
    return _chroma_client


def _sanitize_collection_name(name: str) -> str:
    """chromadb のコレクション名制約に合わせてサニタイズする (3-63文字, 英数字/_/-)。"""
    cleaned = re.sub(r"[^a-zA-Z0-9_-]", "_", name)
    cleaned = re.sub(r"^[^a-zA-Z0-9]+", "", cleaned)
    cleaned = re.sub(r"[^a-zA-Z0-9]+$", "", cleaned)
    if len(cleaned) < 3:
        cleaned = cleaned.ljust(3, "x")
    return cleaned[:63]


def parse_cache_key(cache_key: str) -> Dict[str, str]:
    """旧キャッシュキーをメタデータに分解する。

    例:
      hints::Acme::エンジニア → interview_hints / company=Acme / role=エンジニア
      Acme::es_review → es_review
      Acme::一般職 → company_context
    """
    key = (cache_key or "").strip()
    if key.startswith("hints::"):
        parts = key.split("::", 2)
        company = parts[1] if len(parts) > 1 else ""
        role = parts[2] if len(parts) > 2 else "general"
        return {
            "collection": COLLECTION_INTERVIEW_HINTS,
            "doc_type": "interview_hints",
            "company": company or "unknown",
            "role": role or "general",
            "cache_key": key,
        }

    parts = key.split("::", 1)
    company = parts[0] if parts else "unknown"
    role = parts[1] if len(parts) > 1 else "general"
    if role == "es_review":
        return {
            "collection": COLLECTION_ES_REVIEW,
            "doc_type": "es_review",
            "company": company or "unknown",
            "role": "general",
            "cache_key": key,
        }
    return {
        "collection": COLLECTION_COMPANY_CONTEXT,
        "doc_type": "company_research",
        "company": company or "unknown",
        "role": role or "general",
        "cache_key": key,
    }


def _is_fresh(fetched_at: Optional[str], ttl_seconds: int = CACHE_TTL_SECONDS) -> bool:
    if not fetched_at or ttl_seconds <= 0:
        return True
    try:
        ts = datetime.fromisoformat(fetched_at.replace("Z", "+00:00"))
        age = (datetime.now(timezone.utc) - ts.astimezone(timezone.utc)).total_seconds()
        return age <= ttl_seconds
    except ValueError:
        return True


def _doc_ids(cache_key: str, count: int) -> List[str]:
    digest = hashlib.sha256(cache_key.encode("utf-8")).hexdigest()[:16]
    return [f"{digest}_{i}" for i in range(count)]


def get_cached_documents(
    cache_key: str,
    query_embedding: List[float],
    n_results: int = 5,
    allow_company_fallback: bool = True,
) -> List[str]:
    """企業単位コレクションから類似ドキュメントを取得する。"""
    meta = parse_cache_key(cache_key)
    collection_name = _sanitize_collection_name(meta["collection"])
    try:
        client = get_chroma_client()
        existing = {getattr(col, "name", "") for col in client.list_collections()}
        if collection_name not in existing:
            logger.info(
                "chromadb cache miss key=%s collection=%s reason=collection_not_found",
                cache_key,
                collection_name,
            )
            return []

        collection = client.get_collection(collection_name)
        if collection.count() == 0:
            return []

        docs = _query_with_where(
            collection,
            query_embedding,
            where={"company": meta["company"], "role": meta["role"]},
            n_results=n_results,
        )
        if docs:
            logger.info(
                "chromadb cache hit key=%s collection=%s mode=company_role docs=%d",
                cache_key,
                collection_name,
                len(docs),
            )
            return docs

        if allow_company_fallback and meta["doc_type"] == "interview_hints":
            docs = _query_with_where(
                collection,
                query_embedding,
                where={"company": meta["company"]},
                n_results=n_results,
            )
            if docs:
                logger.info(
                    "chromadb cache hit key=%s collection=%s mode=company_fallback docs=%d",
                    cache_key,
                    collection_name,
                    len(docs),
                )
                return docs

        logger.info("chromadb cache miss key=%s collection=%s", cache_key, collection_name)
        return []
    except Exception as exc:
        logger.exception("chromadb get failed key=%s error=%s", cache_key, exc)
        return []


def _query_with_where(
    collection: Any,
    query_embedding: List[float],
    where: Dict[str, str],
    n_results: int,
) -> List[str]:
    try:
        results = collection.query(
            query_embeddings=[query_embedding],
            n_results=n_results,
            where=where,
            include=["documents", "metadatas"],
        )
    except Exception:
        # where フィルタ非対応環境向けフォールバック
        results = collection.query(
            query_embeddings=[query_embedding],
            n_results=n_results,
            include=["documents", "metadatas"],
        )

    documents: List[str] = results.get("documents", [[]])[0] or []
    metadatas: List[Dict[str, Any]] = results.get("metadatas", [[]])[0] or []
    fresh_docs: List[str] = []
    for doc, md in zip(documents, metadatas):
        if not doc:
            continue
        fetched_at = (md or {}).get("fetched_at")
        if _is_fresh(fetched_at):
            fresh_docs.append(doc)
    return fresh_docs


def set_cached_documents(
    cache_key: str,
    docs: List[str],
    embeddings: List[List[float]],
    source: str = "unknown",
) -> None:
    """ドキュメントを固定コレクションへ upsert する。"""
    if not docs or not embeddings or len(docs) != len(embeddings):
        return
    meta = parse_cache_key(cache_key)
    collection_name = _sanitize_collection_name(meta["collection"])
    fetched_at = datetime.now(timezone.utc).isoformat()
    try:
        client = get_chroma_client()
        collection = client.get_or_create_collection(collection_name)
        ids = _doc_ids(cache_key, len(docs))
        metadatas = [
            {
                "company": meta["company"],
                "role": meta["role"],
                "doc_type": meta["doc_type"],
                "source": source,
                "fetched_at": fetched_at,
                "cache_key": meta["cache_key"],
            }
            for _ in docs
        ]
        collection.upsert(
            ids=ids,
            documents=docs,
            embeddings=embeddings,
            metadatas=metadatas,
        )
        logger.info(
            "chromadb upsert key=%s collection=%s docs=%d source=%s",
            cache_key,
            collection_name,
            len(docs),
            source,
        )
    except Exception as exc:
        logger.exception("chromadb set failed key=%s error=%s", cache_key, exc)


def describe_backend() -> str:
    if CHROMA_HOST:
        return f"chromadb http://{CHROMA_HOST}:{CHROMA_PORT}"
    return f"chromadb persistent:{CHROMA_DATA_DIR}"
