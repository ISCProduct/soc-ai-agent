"""埋め込み・類似度・ドキュメント取得。

patch("main.OpenAI") / patch("main.EMBED_MAX_RETRIES") / patch("main.embed_texts")
との互換のため、実行時に main モジュール上のシンボルを参照する。
"""
from __future__ import annotations

import logging
import math
import os
import time
from typing import List

import tiktoken
from fastapi import HTTPException

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
        return enc.decode(tokens[:m.MAX_EMBED_TOKENS])
    return text


def embed_texts(texts: List[str]) -> List[List[float]]:
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise HTTPException(status_code=500, detail="OPENAI_API_KEY is required")

    embedding_model = os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    client = m.OpenAI(api_key=api_key)

    # トークン上限チェック
    texts = [_truncate_text(t, embedding_model) for t in texts]

    last_err: Exception = RuntimeError("embed_texts: no attempts made")
    for attempt in range(1, m.EMBED_MAX_RETRIES + 1):
        try:
            response = client.embeddings.create(model=embedding_model, input=texts)
            return [item.embedding for item in response.data]
        except Exception as exc:
            last_err = exc
            if attempt < m.EMBED_MAX_RETRIES:
                wait = 2 ** (attempt - 1)
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
    import main as m

    if not docs:
        return []
    embeddings = m.embed_texts(docs + [query])
    doc_embeddings = embeddings[:-1]
    query_embedding = embeddings[-1]

    scored = []
    for doc, emb in zip(docs, doc_embeddings):
        scored.append((m.cosine_similarity(query_embedding, emb), doc))
    scored.sort(key=lambda item: item[0], reverse=True)
    top_docs = [doc for _, doc in scored[: min(5, len(scored))]]
    return top_docs
