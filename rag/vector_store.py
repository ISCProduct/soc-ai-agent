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

ALL_COLLECTIONS = (
    COLLECTION_COMPANY_CONTEXT,
    COLLECTION_INTERVIEW_HINTS,
    COLLECTION_ES_REVIEW,
)

DOC_TYPE_TO_COLLECTION = {
    "resume_review": COLLECTION_COMPANY_CONTEXT,
    "company_research": COLLECTION_COMPANY_CONTEXT,
    "interview_hints": COLLECTION_INTERVIEW_HINTS,
    "es_review": COLLECTION_ES_REVIEW,
}

_chroma_client: Any = None
_chroma_lock = threading.Lock()


def reset_chroma_client_for_tests() -> None:
    """テスト用にシングルトンをクリアする。"""
    global _chroma_client
    with _chroma_lock:
        _chroma_client = None


def _collection_names(client: Any) -> set:
    """Chroma 0.6+ では list_collections が名前文字列のみ返す場合がある。"""
    names: set = set()
    for col in client.list_collections():
        if isinstance(col, str):
            names.add(col)
        else:
            name = getattr(col, "name", "") or ""
            if name:
                names.add(name)
    return names


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
    # デフォルトは企業調査。履歴書レビューは呼び出し側で doc_type を上書きする。
    return {
        "collection": COLLECTION_COMPANY_CONTEXT,
        "doc_type": "company_research",
        "company": company or "unknown",
        "role": role or "general",
        "cache_key": key,
    }


def build_cache_key(doc_type: str, company: str, role: str = "general") -> str:
    """doc_type / company / role から互換キャッシュキーを組み立てる。"""
    company = (company or "unknown").strip() or "unknown"
    role = (role or "general").strip() or "general"
    if doc_type == "interview_hints":
        return f"hints::{company}::{role}"
    if doc_type == "es_review":
        return f"{company}::es_review"
    return f"{company}::{role}"


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
        existing = _collection_names(client)
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
            where=_build_where({"company": meta["company"], "role": meta["role"]}),
            n_results=n_results,
            fallback_where=_build_where({"company": meta["company"]}),
        )
        if docs:
            logger.info(
                "chromadb cache hit key=%s collection=%s mode=company_role docs=%d",
                cache_key,
                collection_name,
                len(docs),
            )
            return docs

        # 職種ミス時は同一企業の任意 role を再利用（#510 / Phase 3 横断）
        if allow_company_fallback:
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


def _build_where(filters: Dict[str, str]) -> Dict[str, Any]:
    """ChromaDBはフラットな複数キーのwhereを許可しない(1オペレータのみ)ため、
    2キー以上ある場合は$andでラップする。1キー以下ならそのまま返す。"""
    if len(filters) <= 1:
        return dict(filters)
    return {"$and": [{k: v} for k, v in filters.items()]}


def _query_with_where(
    collection: Any,
    query_embedding: List[float],
    where: Dict[str, Any],
    n_results: int,
    fallback_where: Optional[Dict[str, Any]] = None,
) -> List[str]:
    def _run(w: Dict[str, Any]) -> Any:
        return collection.query(
            query_embeddings=[query_embedding],
            n_results=n_results,
            where=w,
            include=["documents", "metadatas"],
        )

    try:
        results = _run(where)
    except Exception:
        # whereフィルタが失敗した場合でも、他社のデータが混入しないよう
        # fallback_where(通常はcompanyのみ)でスコープを維持したまま再試行する。
        # スコープを維持できない場合は、フィルタ無し検索(他社データ混入の
        # リスクがある)は行わずキャッシュミス扱いにする。
        if not fallback_where:
            return []
        try:
            results = _run(fallback_where)
        except Exception:
            return []

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
    doc_type: Optional[str] = None,
) -> None:
    """ドキュメントを固定コレクションへ upsert する。"""
    if not docs or not embeddings or len(docs) != len(embeddings):
        return
    meta = parse_cache_key(cache_key)
    if doc_type:
        meta["doc_type"] = doc_type
        mapped = DOC_TYPE_TO_COLLECTION.get(doc_type)
        if mapped:
            meta["collection"] = mapped
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
            "chromadb upsert key=%s collection=%s docs=%d source=%s doc_type=%s",
            cache_key,
            collection_name,
            len(docs),
            source,
            meta["doc_type"],
        )
    except Exception as exc:
        logger.exception("chromadb set failed key=%s error=%s", cache_key, exc)


def upsert_by_doc_type(
    company: str,
    role: str,
    doc_type: str,
    docs: List[str],
    embeddings: List[List[float]],
    source: str = "unknown",
) -> str:
    """company / role / doc_type で統一書き込みし、使用したキャッシュキーを返す。"""
    cache_key = build_cache_key(doc_type, company, role)
    set_cached_documents(
        cache_key, docs, embeddings, source=source, doc_type=doc_type
    )
    return cache_key


def delete_company_documents(
    company: str,
    doc_type: Optional[str] = None,
) -> Dict[str, Any]:
    """企業に紐づくベクトルを削除する。doc_type 指定時はその用途のみ。"""
    company = (company or "").strip()
    if not company:
        return {"deleted": 0, "collections": {}}

    collections = ALL_COLLECTIONS
    if doc_type:
        mapped = DOC_TYPE_TO_COLLECTION.get(doc_type)
        collections = (mapped,) if mapped else ()

    client = get_chroma_client()
    existing = _collection_names(client)
    deleted_total = 0
    per_collection: Dict[str, int] = {}

    for name in collections:
        if name not in existing:
            per_collection[name] = 0
            continue
        collection = client.get_collection(name)
        try:
            # Chroma の where 削除。未対応環境では get→delete ids にフォールバック。
            before = collection.count()
            try:
                collection.delete(where={"company": company})
            except Exception:
                got = collection.get(where={"company": company}, include=[])
                ids = got.get("ids") or []
                if ids:
                    collection.delete(ids=ids)
            after = collection.count()
            removed = max(0, before - after)
        except Exception as exc:
            logger.warning("chromadb delete failed collection=%s company=%s err=%s", name, company, exc)
            removed = 0
        per_collection[name] = removed
        deleted_total += removed

    return {"deleted": deleted_total, "collections": per_collection}


def get_index_status(company: Optional[str] = None) -> Dict[str, Any]:
    """インデックス状況（コレクション件数・任意で企業フィルタ概要）を返す。"""
    client = get_chroma_client()
    existing = _collection_names(client)
    collections_out: List[Dict[str, Any]] = []

    for name in ALL_COLLECTIONS:
        info: Dict[str, Any] = {"name": name, "exists": name in existing, "count": 0}
        if name not in existing:
            collections_out.append(info)
            continue
        collection = client.get_collection(name)
        info["count"] = collection.count()
        if company:
            try:
                try:
                    got = collection.get(
                        where={"company": company},
                        include=["metadatas"],
                        limit=20,
                    )
                except TypeError:
                    got = collection.get(
                        where={"company": company},
                        include=["metadatas"],
                    )
                metas = (got.get("metadatas") or [])[:20]
                info["company_count"] = len(metas)
                sources = sorted({
                    str((m or {}).get("source") or "")
                    for m in metas
                    if (m or {}).get("source")
                })
                doc_types = sorted({
                    str((m or {}).get("doc_type") or "")
                    for m in metas
                    if (m or {}).get("doc_type")
                })
                fetched = [
                    str((m or {}).get("fetched_at") or "")
                    for m in metas
                    if (m or {}).get("fetched_at")
                ]
                info["sources"] = sources
                info["doc_types"] = doc_types
                info["latest_fetched_at"] = max(fetched) if fetched else None
            except Exception as exc:
                logger.warning("chromadb status filter failed collection=%s err=%s", name, exc)
                info["company_count"] = None
                info["error"] = str(exc)
        collections_out.append(info)

    return {
        "backend": describe_backend(),
        "host": CHROMA_HOST or None,
        "port": CHROMA_PORT if CHROMA_HOST else None,
        "company": company,
        "collections": collections_out,
        "total_documents": sum(int(c.get("count") or 0) for c in collections_out),
    }


def ping_chroma() -> Tuple[bool, str]:
    """Chroma 接続確認。"""
    try:
        client = get_chroma_client()
        client.list_collections()
        return True, describe_backend()
    except Exception as exc:
        return False, str(exc)


def describe_backend() -> str:
    if CHROMA_HOST:
        return f"chromadb http://{CHROMA_HOST}:{CHROMA_PORT}"
    return f"chromadb persistent:{CHROMA_DATA_DIR}"
