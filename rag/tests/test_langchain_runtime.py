"""LangChain 導入のスモークテスト。"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest


def test_langchain_packages_importable():
    import langchain
    import langchain_core
    import langchain_openai
    from langchain_core.documents import Document

    assert langchain.__name__ == "langchain"
    assert langchain_core.__name__ == "langchain_core"
    assert langchain_openai.__name__ == "langchain_openai"
    assert Document(page_content="x").page_content == "x"


def test_langchain_runtime_get_embeddings_requires_api_key():
    from services import langchain_runtime as runtime

    runtime.get_embeddings.cache_clear()
    env = {k: v for k, v in __import__("os").environ.items() if k != "OPENAI_API_KEY"}
    with patch.dict("os.environ", env, clear=True):
        with pytest.raises(ValueError, match="OPENAI_API_KEY"):
            runtime.get_embeddings()


def test_langchain_runtime_get_embeddings_cached(monkeypatch):
    from services import langchain_runtime as runtime

    runtime.get_embeddings.cache_clear()
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    with patch("services.langchain_runtime.OpenAIEmbeddings") as mock_cls:
        mock_cls.return_value = MagicMock(name="embeddings")
        a = runtime.get_embeddings("text-embedding-3-small")
        b = runtime.get_embeddings("text-embedding-3-small")
        assert a is b
        assert mock_cls.call_count == 1
    runtime.get_embeddings.cache_clear()
