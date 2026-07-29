"""LangChain ランタイム（埋め込み・今後の Chat / Retriever 共通入口）。

FastAPI エンドポイントはそのままに、LLM/埋め込み実装を LangChain へ寄せるための薄いファサード。
"""
from __future__ import annotations

import os
from functools import lru_cache

from langchain_openai import ChatOpenAI, OpenAIEmbeddings


def embedding_model_name() -> str:
    return os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")


def chat_model_name() -> str:
    return os.getenv("OPENAI_CHAT_MODEL", os.getenv("OPENAI_MODEL", "gpt-4o-mini"))


@lru_cache(maxsize=4)
def get_embeddings(model: str | None = None) -> OpenAIEmbeddings:
    """OpenAI Embeddings（LangChain）。API キーは環境変数 OPENAI_API_KEY。"""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY is required")
    return OpenAIEmbeddings(
        model=model or embedding_model_name(),
        api_key=api_key,
    )


def get_chat_model(model: str | None = None, temperature: float = 0.2) -> ChatOpenAI:
    """Chat モデル（LangChain）。段階的に research / review へ適用する。"""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY is required")
    return ChatOpenAI(
        model=model or chat_model_name(),
        api_key=api_key,
        temperature=temperature,
    )
