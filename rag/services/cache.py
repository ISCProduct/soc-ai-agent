"""企業コンテキストのキャッシュ get/set ラッパー。

patch("main.embed_texts") 互換のため実行時に main を参照する。
"""
from __future__ import annotations

import logging
from typing import List, Optional

from vector_store import get_cached_documents, set_cached_documents

logger = logging.getLogger("main")


def get_cached_context(
        cache_key: str, query: str = "採用 価値観 求める人物像"
) -> List[str]:
    """ベクトルDBからキャッシュ済みドキュメントを類似度順で最大 5 件取得する。"""
    import main as m

    try:
        query_emb = m.embed_texts([query])[0]
        return get_cached_documents(cache_key, query_emb, n_results=5)
    except Exception as exc:
        logger.exception("chromadb get failed key=%s error=%s", cache_key, exc)
        return []


def set_cached_context(
        cache_key: str,
        docs: List[str],
        source: str = "unknown",
        doc_type: Optional[str] = None,
) -> None:
    """ドキュメントと埋め込みをベクトルDBに永続保存する。"""
    import main as m

    if not docs:
        return
    try:
        embeddings = m.embed_texts(docs)
        set_cached_documents(
            cache_key, docs, embeddings, source=source, doc_type=doc_type
        )
    except Exception as exc:
        logger.exception("chromadb set failed key=%s error=%s", cache_key, exc)
