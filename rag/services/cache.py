"""企業コンテキストのキャッシュ get/set ラッパー。

patch("main.embed_texts") 互換のため実行時に main を参照する。
"""
from __future__ import annotations

import logging
import os
import threading
from collections import OrderedDict
from typing import List, Optional, Tuple

from vector_store import get_cached_documents, set_cached_documents

logger = logging.getLogger("main")


# 検索クエリの埋め込みのキャッシュ（SOCAIAGENT-556）。
#
# キャッシュの当たり判定のたびに、クエリを埋め込み API へ投げていた。既定のクエリ
# 「採用 価値観 求める人物像」は定数で、ES 添削のクエリも「企業名 + 定型文」なので、同じ文を
# 何度も課金していた。埋め込みはモデルと文で決まるので、(モデル, 文) で覚えておく。
# 企業名入りのクエリで増えすぎないよう、件数に上限を掛けて古いものから捨てる。
_QUERY_EMBEDDING_CACHE_MAX = 256
_query_embeddings: "OrderedDict[Tuple[str, str], List[float]]" = OrderedDict()
_query_embeddings_lock = threading.Lock()


def _query_embedding(query: str) -> List[float]:
    """クエリの埋め込み。同じモデル・同じ文は2回目から API を呼ばない。失敗は覚えない。"""
    import main as m

    key = (os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"), query)
    with _query_embeddings_lock:
        cached = _query_embeddings.get(key)
        if cached is not None:
            _query_embeddings.move_to_end(key)
            return cached
    embedding = m.embed_texts([query])[0]
    with _query_embeddings_lock:
        _query_embeddings[key] = embedding
        _query_embeddings.move_to_end(key)
        while len(_query_embeddings) > _QUERY_EMBEDDING_CACHE_MAX:
            _query_embeddings.popitem(last=False)
    return embedding


def clear_query_embedding_cache() -> None:
    """テスト用。埋め込みを差し替えるテストの間で持ち越さない。"""
    with _query_embeddings_lock:
        _query_embeddings.clear()


def get_cached_context(
        cache_key: str, query: str = "採用 価値観 求める人物像"
) -> List[str]:
    """ベクトルDBからキャッシュ済みドキュメントを類似度順で最大 5 件取得する。"""
    try:
        query_emb = _query_embedding(query)
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
