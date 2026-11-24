"""検索クエリの埋め込みのキャッシュ（services/cache.py、SOCAIAGENT-556）。"""
from unittest.mock import MagicMock, patch

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


def test_key_size_does_not_grow_with_query_length(monkeypatch):
    """長い企業名を変えながら送られても、キーは固定長（SHA-256）で、文そのものを持たない。"""
    import main

    embed = MagicMock(return_value=[[0.1]])
    monkeypatch.setattr(main, "embed_texts", embed)
    long_queries = [f"{'長い企業名' * 2000}{i} 求める人物像" for i in range(3)]
    with patch.object(cache, "get_cached_documents", return_value=[]):
        for q in long_queries:
            cache.get_cached_context("key", query=q)
        cache.get_cached_context("key", query=long_queries[0])
    assert embed.call_count == 3  # 同じ文は2回目から呼ばない（ハッシュでも当たる）
    for model, digest in cache._query_embeddings:
        assert len(digest) == 64
        assert all(digest != q for q in long_queries)
