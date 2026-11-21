"""検索クエリの埋め込みのキャッシュ（services/cache.py、SOCAIAGENT-556）。"""
from unittest.mock import MagicMock, patch

import pytest

from services import cache


def test_same_query_is_embedded_once(monkeypatch):
    import main

    embed = MagicMock(return_value=[[0.1, 0.2]])
    monkeypatch.setattr(main, "embed_texts", embed)
    with patch.object(cache, "get_cached_documents", return_value=["doc"]):
        for _ in range(3):
            assert cache.get_cached_context("key") == ["doc"]
    assert embed.call_count == 1


def test_different_query_or_model_is_embedded_again(monkeypatch):
    import main

    embed = MagicMock(return_value=[[0.1]])
    monkeypatch.setattr(main, "embed_texts", embed)
    with patch.object(cache, "get_cached_documents", return_value=[]):
        cache.get_cached_context("key", query="A社 求める人物像")
        cache.get_cached_context("key", query="B社 求める人物像")
        monkeypatch.setenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-large")
        cache.get_cached_context("key", query="A社 求める人物像")
    assert embed.call_count == 3


def test_failure_is_not_cached(monkeypatch):
    import main

    embed = MagicMock(side_effect=[RuntimeError("down"), [[0.1]]])
    monkeypatch.setattr(main, "embed_texts", embed)
    with patch.object(cache, "get_cached_documents", return_value=["doc"]):
        assert cache.get_cached_context("key") == []  # 失敗はキャッシュミス扱い（今までどおり）
        assert cache.get_cached_context("key") == ["doc"]  # 次は API を呼び直す
    assert embed.call_count == 2


def test_cache_is_bounded(monkeypatch):
    import main

    monkeypatch.setattr(cache, "_QUERY_EMBEDDING_CACHE_MAX", 2)
    embed = MagicMock(return_value=[[0.1]])
    monkeypatch.setattr(main, "embed_texts", embed)
    with patch.object(cache, "get_cached_documents", return_value=[]):
        for q in ["q1", "q2", "q3", "q1"]:
            cache.get_cached_context("key", query=q)
    # q1 は q3 を入れたときに押し出されているので、4回目は呼び直す
    assert embed.call_count == 4
    assert len(cache._query_embeddings) == 2
