"""Web Search パイプライン全体の上限時間（services/deadline.py、SOCAIAGENT-558）のテスト。"""
import asyncio
import time
from unittest.mock import MagicMock

import pytest

from services import deadline


def test_deadline_sec_from_env(monkeypatch):
    monkeypatch.delenv("RAG_WEB_SEARCH_DEADLINE_SEC", raising=False)
    assert deadline.web_search_deadline_sec() == deadline.DEFAULT_WEB_SEARCH_DEADLINE_SEC
    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "12.5")
    assert deadline.web_search_deadline_sec() == 12.5
    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "abc")
    assert deadline.web_search_deadline_sec() == deadline.DEFAULT_WEB_SEARCH_DEADLINE_SEC


def _slow(seconds):
    def fn(*_args, **_kwargs):
        time.sleep(seconds)
        return "結果"
    return fn


def test_web_search_pipeline_returns_empty_after_deadline(monkeypatch):
    """検索が遅れても上限で空文字を返し、スレッドの終わりを待たない。"""
    import main
    from services.research import _run_web_search_pipeline

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "0.3")
    monkeypatch.setattr(main, "_generate_search_queries", lambda *_: ["q1", "q2"])
    monkeypatch.setattr(main, "_web_search_openai", _slow(2.0))
    summarize = MagicMock(return_value="要約")
    monkeypatch.setattr(main, "_summarize_for_hiring", summarize)

    started = time.monotonic()
    # 本番と同じく asyncio.run（_run_async）で呼ぶ。既定のプールを使うと終了処理で待たされる
    result = main._run_async(_run_web_search_pipeline, "テスト株式会社", "")
    elapsed = time.monotonic() - started

    assert result == ""
    assert elapsed < 1.5, elapsed
    summarize.assert_not_called()


def test_web_search_pipeline_within_deadline_returns_summary(monkeypatch):
    import main
    from services.research import _run_web_search_pipeline

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "5")
    monkeypatch.setattr(main, "_generate_search_queries", lambda *_: ["q1"])
    monkeypatch.setattr(main, "_web_search_openai", lambda q: f"{q}の結果")
    monkeypatch.setattr(main, "rank_results_by_domain_trust", lambda results, _c: results)
    monkeypatch.setattr(main, "_summarize_for_hiring", lambda *_: "要約")
    monkeypatch.setattr(main, "_save_search_log", lambda *_: None)

    assert main._run_async(_run_web_search_pipeline, "テスト株式会社", "") == "要約"


@pytest.mark.parametrize("value", ["0", "-1"])
def test_deadline_can_be_disabled(monkeypatch, value):
    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", value)

    async def steps(_executor):
        await asyncio.sleep(0.05)
        return "ok"

    assert asyncio.run(deadline.run_with_deadline(steps, "test", "A社")) == "ok"


def test_hints_pipeline_returns_empty_after_deadline(monkeypatch):
    """面接ヒントの Web Search も同じ上限で打ち切る。要約の同期呼び出しでもループを止めない。"""
    import main
    from services.hints import _run_hints_web_search_pipeline

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "0.3")
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setattr(main, "_web_search_openai", lambda q: "検索結果")
    client = MagicMock()
    client.chat.completions.create.side_effect = _slow(2.0)
    monkeypatch.setattr(main, "OpenAI", lambda **_kw: client)

    started = time.monotonic()
    result = main._run_async(_run_hints_web_search_pipeline, "テスト株式会社", "エンジニア", ["q1"])
    elapsed = time.monotonic() - started

    assert result == ""
    assert elapsed < 1.5, elapsed
