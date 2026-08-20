"""埋め込み・類似度・ドキュメント取得。

埋め込みは LangChain（langchain-openai.OpenAIEmbeddings）経由。
patch("main.embed_texts") / patch("main.EMBED_MAX_RETRIES") 互換のため、
実行時に main モジュール上のシンボルも参照する。
"""
from __future__ import annotations

import logging
import math
import os
import time
from typing import List

import tiktoken
from fastapi import HTTPException
from langchain_core.documents import Document

from services.langchain_runtime import get_embeddings

logger = logging.getLogger("main")


def _truncate_text(text: str, model: str) -> str:
    """テキストが埋め込みモデルのトークン上限を超えている場合に切り詰める。"""
    import main as m

    try:
        enc = tiktoken.encoding_for_model(model)
    except KeyError:
        enc = tiktoken.get_encoding("cl100k_base")
    tokens = enc.encode(text)
    if len(tokens) > m.MAX_EMBED_TOKENS:
        logger.warning(
            "truncating text from %d to %d tokens for model=%s",
            len(tokens),
            m.MAX_EMBED_TOKENS,
            model,
        )
        return enc.decode(tokens[: m.MAX_EMBED_TOKENS])
    return text


def embed_texts(texts: List[str]) -> List[List[float]]:
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise HTTPException(status_code=500, detail="OPENAI_API_KEY is required")

    embedding_model = os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    texts = [_truncate_text(t, embedding_model) for t in texts]

    # get_embeddings は api_key 欠如で ValueError。上で既にチェック済み。
    embeddings = get_embeddings(embedding_model)

    # #994: FastAPIの同期routeはStarletteのスレッドプール(既定サイズに上限あり)で
    # 実行されるため、ここのtime.sleepはイベントループ自体はブロックしないが、
    # OpenAI側が不安定な間に同時リクエストが多いとプールを専有し得る。
    # 暫定対応として、最大待機時間を明示的に上限で頭打ちにして
    # 1リクエストあたりの専有時間を抑える運用でしのぐ。根本対応(非同期化)が必要に
    # なったら、embed_texts自体をasync defにしてasyncio.sleepへ切り替え、
    # 呼び出し元(company.py/cache.py等)もawaitするよう連鎖的に変更する。
    max_retry_wait_seconds = 5
    last_err: Exception = RuntimeError("embed_texts: no attempts made")
    for attempt in range(1, m.EMBED_MAX_RETRIES + 1):
        try:
            return embeddings.embed_documents(texts)
        except Exception as exc:
            last_err = exc
            if attempt < m.EMBED_MAX_RETRIES:
                wait = min(2 ** (attempt - 1), max_retry_wait_seconds)
                logger.warning(
                    "embed_texts failed attempt=%d retrying in %ds error=%s",
                    attempt,
                    wait,
                    exc,
                )
                time.sleep(wait)
    raise last_err


def cosine_similarity(a: List[float], b: List[float]) -> float:
    dot = 0.0
    norm_a = 0.0
    norm_b = 0.0
    for av, bv in zip(a, b):
        dot += av * bv
        norm_a += av * av
        norm_b += bv * bv
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return dot / (math.sqrt(norm_a) * math.sqrt(norm_b))


def retrieve_docs(docs: List[str], query: str) -> List[str]:
    """クエリに近いドキュメント上位を返す（LangChain Document + 埋め込み類似度）。"""
    import main as m

    if not docs:
        return []

    # Document 化して LC 上の単位を揃える（本文は page_content）
    documents = [Document(page_content=doc) for doc in docs]
    embeddings = m.embed_texts([d.page_content for d in documents] + [query])
    doc_embeddings = embeddings[:-1]
    query_embedding = embeddings[-1]

    scored: list[tuple[float, str]] = []
    for doc, emb in zip(documents, doc_embeddings):
        scored.append((m.cosine_similarity(query_embedding, emb), doc.page_content))
    scored.sort(key=lambda item: item[0], reverse=True)
    return [doc for _, doc in scored[: min(5, len(scored))]]
