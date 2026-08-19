"""LangChain ランタイム（埋め込み・今後の Chat / Retriever 共通入口）。

FastAPI エンドポイントはそのままに、LLM/埋め込み実装を LangChain へ寄せるための薄いファサード。
再試行は外側（embed_texts の EMBED_MAX_RETRIES）に集約し、LangChain 側は max_retries=0。
"""
from __future__ import annotations

import os
from functools import lru_cache

from langchain_openai import ChatOpenAI, OpenAIEmbeddings

# 外側リトライと二重にしない。タイムアウトはハング防止用。
_DEFAULT_REQUEST_TIMEOUT_SEC = float(os.getenv("RAG_OPENAI_TIMEOUT_SEC", "60"))


def embedding_model_name() -> str:
    return os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")


def chat_model_name() -> str:
    return os.getenv("OPENAI_CHAT_MODEL", os.getenv("OPENAI_MODEL", "gpt-4o-mini"))


@lru_cache(maxsize=4)
def _get_embeddings_cached(model: str, api_key: str) -> OpenAIEmbeddings:
    return OpenAIEmbeddings(
        model=model,
        api_key=api_key,
        max_retries=0,
        request_timeout=_DEFAULT_REQUEST_TIMEOUT_SEC,
    )


def get_embeddings(model: str | None = None) -> OpenAIEmbeddings:
    """OpenAI Embeddings（LangChain）。API キーは環境変数 OPENAI_API_KEY。

    api_key をキャッシュキーに含めることで、Secrets Manager 等によるローテーション後も
    プロセス再起動なしで新しいキーが使われる（古いキーのエントリは maxsize=4 で自然に押し出される）。
    """
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY is required")
    return _get_embeddings_cached(model or embedding_model_name(), api_key)


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
