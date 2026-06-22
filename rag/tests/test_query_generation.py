from unittest.mock import patch, MagicMock
import os
import json
import tempfile

import main


def test_generate_search_queries_fallback_on_llm_failure(tmp_path, monkeypatch):
    # prepare env to force LLM path but simulate failure
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    # ensure cache dir
    tmpdir = tmp_path / "search_logs"
    tmpdir.mkdir()
    monkeypatch.setenv("RAG_SEARCH_LOG_DIR", str(tmpdir))
    # patch OpenAI to raise
    class DummyClient:
        def __init__(self, api_key=None):
            pass

        class chat:
            @staticmethod
            def completions():
                raise Exception("forced")

    with patch("main.OpenAI", side_effect=Exception("client init failed")):
        queries = main._generate_search_queries("テスト社", "エンジニア")

    assert isinstance(queries, list)
    assert len(queries) >= 3
    assert any("テスト社" in q for q in queries)


def test_generate_search_queries_uses_cache(tmp_path, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    tmpdir = tmp_path / "search_logs"
    tmpdir.mkdir()
    monkeypatch.setenv("RAG_SEARCH_LOG_DIR", str(tmpdir))
    cache_path = tmpdir / "query_cache.json"
    cache = {"テスト社::エンジニア": {"ts": int(__import__("time").time()), "queries": ["cached q1", "cached q2"]}}
    cache_path.write_text(json.dumps(cache, ensure_ascii=False), encoding="utf-8")
    queries = main._generate_search_queries("テスト社", "エンジニア")
    assert queries == ["cached q1", "cached q2"]
