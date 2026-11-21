"""検索クエリの短期キャッシュ（services/query_cache.py、SOCAIAGENT-557）のテスト。"""
import json
import os
import threading

from services import query_cache


def test_roundtrip_and_ttl(tmp_path):
    path = str(tmp_path / "query_cache.json")
    query_cache.put(path, "A社::一般職", ["q1", "q2"], ttl_seconds=300, now=1000)
    assert query_cache.get(path, "A社::一般職", ttl_seconds=300, now=1100) == ["q1", "q2"]
    # TTL を過ぎたら返さない
    assert query_cache.get(path, "A社::一般職", ttl_seconds=300, now=1300) is None
    assert query_cache.get(path, "無い::キー", ttl_seconds=300, now=1100) is None


def test_put_prunes_expired_entries(tmp_path):
    """保存のたびに期限切れを消す（ファイルが無制限に大きくならない）。"""
    path = str(tmp_path / "query_cache.json")
    query_cache.put(path, "old", ["q"], ttl_seconds=300, now=1000)
    query_cache.put(path, "new", ["q"], ttl_seconds=300, now=2000)
    with open(path, encoding="utf-8") as f:
        assert set(json.load(f)) == {"new"}


def test_put_caps_entries_keeping_newest(tmp_path, monkeypatch):
    path = str(tmp_path / "query_cache.json")
    monkeypatch.setattr(query_cache, "MAX_ENTRIES", 3)
    for i in range(5):
        query_cache.put(path, f"k{i}", ["q"], ttl_seconds=300, now=1000 + i)
    with open(path, encoding="utf-8") as f:
        assert set(json.load(f)) == {"k2", "k3", "k4"}


def test_corrupt_file_is_treated_as_empty_and_rewritten(tmp_path):
    path = str(tmp_path / "query_cache.json")
    with open(path, "w", encoding="utf-8") as f:
        f.write("{壊れた")
    assert query_cache.get(path, "k", ttl_seconds=300, now=1000) is None
    query_cache.put(path, "k", ["q"], ttl_seconds=300, now=1000)
    assert query_cache.get(path, "k", ttl_seconds=300, now=1000) == ["q"]


def test_malformed_entries_are_ignored(tmp_path):
    path = str(tmp_path / "query_cache.json")
    with open(path, "w", encoding="utf-8") as f:
        json.dump({"k": {"ts": "x", "queries": "not-a-list"}, "list": [1, 2]}, f)
    assert query_cache.get(path, "k", ttl_seconds=300, now=1000) is None
    assert query_cache.get(path, "list", ttl_seconds=300, now=1000) is None


def test_concurrent_puts_do_not_lose_entries(tmp_path):
    """並列に保存しても、ほかのキーの書き込みが失われない（以前はロック無しで取りこぼした）。"""
    path = str(tmp_path / "query_cache.json")
    keys = [f"k{i}" for i in range(40)]
    threads = [
        threading.Thread(target=query_cache.put, args=(path, key, [key], 300, 1000))
        for key in keys
    ]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    with open(path, encoding="utf-8") as f:
        assert set(json.load(f)) == set(keys)
    # 一時ファイルを残さない
    assert not [name for name in os.listdir(tmp_path) if name.endswith(".tmp")]


def test_empty_cached_queries_are_reused(tmp_path, monkeypatch):
    """保存したクエリが空のリストでも、TTL のあいだは作り直さない（LLM を呼び続けない）。"""
    import main
    from services.research import _generate_search_queries

    monkeypatch.setenv("RAG_SEARCH_LOG_DIR", str(tmp_path))
    monkeypatch.setenv("RAG_QUERY_CACHE_TTL_SECONDS", "300")
    query_cache.put(str(tmp_path / "query_cache.json"), "テスト株式会社::一般職", [], ttl_seconds=300)

    def boom(**_kwargs):
        raise AssertionError("TTL 内なのに LLM でクエリを作り直している")

    monkeypatch.setattr(main, "OpenAI", boom)
    assert _generate_search_queries("テスト株式会社", "") == []
