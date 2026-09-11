"""LangChain ランタイム（埋め込み・今後の Chat / Retriever 共通入口）。

FastAPI エンドポイントはそのままに、LLM/埋め込み実装を LangChain へ寄せるための薄いファサード。
再試行は外側（embed_texts の EMBED_MAX_RETRIES）に集約し、LangChain 側は max_retries=0。
"""
from __future__ import annotations

import hashlib
import os
from functools import lru_cache

from langchain_openai import ChatOpenAI, OpenAIEmbeddings

# 外側リトライと二重にしない。タイムアウトはハング防止用。
_DEFAULT_REQUEST_TIMEOUT_SEC = float(os.getenv("RAG_OPENAI_TIMEOUT_SEC", "60"))
_last_embeddings_key_fp: str | None = None


def embedding_model_name() -> str:
    return os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")


def chat_model_name() -> str:
    return os.getenv("OPENAI_CHAT_MODEL", os.getenv("OPENAI_MODEL", "gpt-4o-mini"))


def _api_key_fingerprint(api_key: str) -> str:
    return hashlib.sha256(api_key.encode("utf-8")).hexdigest()


@lru_cache(maxsize=4)
def _get_embeddings_cached(model: str, api_key_fp: str) -> OpenAIEmbeddings:
    # api_key_fp は lru_cache のキー専用。実キーは環境変数から読む。
    _ = api_key_fp
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY is required")
    return OpenAIEmbeddings(
        model=model,
        api_key=api_key,
        max_retries=0,
        request_timeout=_DEFAULT_REQUEST_TIMEOUT_SEC,
    )


def get_embeddings(model: str | None = None) -> OpenAIEmbeddings:
    """OpenAI Embeddings（LangChain）。API キーは環境変数 OPENAI_API_KEY。

    キャッシュキーには API キーの指紋だけを使い、ローテーション時は旧インスタンスを破棄する。
    """
    global _last_embeddings_key_fp
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY is required")
    fp = _api_key_fingerprint(api_key)
    if _last_embeddings_key_fp is not None and _last_embeddings_key_fp != fp:
        _get_embeddings_cached.cache_clear()
    _last_embeddings_key_fp = fp
    return _get_embeddings_cached(model or embedding_model_name(), fp)


def get_chat_model(model: str | None = None, temperature: float = 0.2) -> ChatOpenAI:
    """Chat モデル（LangChain）。段階的に research / review へ適用する。"""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY is required")
    return ChatOpenAI(
        model=model or chat_model_name(),
        api_key=api_key,
        temperature=temperature,
        max_retries=0,
        request_timeout=_DEFAULT_REQUEST_TIMEOUT_SEC,
    )
