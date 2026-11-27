"""Web Search パイプライン全体の上限時間（services/deadline.py、SOCAIAGENT-558）のテスト。"""
import asyncio
import threading
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
    """面接ヒントの Web Search も同じ上限で打ち切る。検索の同期呼び出しでもループを止めない。
    検索が上限までに終わらなければ、返せる結果が無いので空文字（要約だけが遅いときは別のテスト）。"""
    import main
    from services.hints import _run_hints_web_search_pipeline

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "0.3")
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setattr(main, "_web_search_openai", _slow(2.0))
    client = MagicMock()
    monkeypatch.setattr(main, "OpenAI", lambda **_kw: client)

    started = time.monotonic()
    result = main._run_async(_run_hints_web_search_pipeline, "テスト株式会社", "エンジニア", ["q1"])
    elapsed = time.monotonic() - started

    assert result == ""
    assert elapsed < 1.5, elapsed


@pytest.mark.parametrize("value", ["nan", "inf", "-inf", "abc"])
def test_deadline_sec_rejects_non_finite(monkeypatch, value):
    # nan は float() を通るが、wait_for に渡すと例外になって Web Search がすべて止まる
    from services import deadline

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", value)
    assert deadline.web_search_deadline_sec() == deadline.DEFAULT_WEB_SEARCH_DEADLINE_SEC


def test_hints_pipeline_falls_back_to_search_results_when_summary_is_late(monkeypatch):
    # 検索は終わったが要約が上限に間に合わないときは、検索結果の連結を返す（要約が失敗したときと同じ）
    import main
    from services import hints

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "0.3")
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setattr(main, "_web_search_openai", lambda q: f"結果:{q}")

    class SlowCompletions:
        def create(self, **kwargs):
            time.sleep(2)

    class SlowClient:
        def __init__(self, **kwargs):
            self.chat = type("Chat", (), {"completions": SlowCompletions()})()

    monkeypatch.setattr(main, "OpenAI", SlowClient)
    started = time.monotonic()
    out = asyncio.run(hints._run_hints_web_search_pipeline("A社", "一般職", ["q1", "q2"]))
    assert out == "結果:q1\n\n結果:q2"
    assert time.monotonic() - started < 1.5


def test_search_log_is_saved_outside_deadline(monkeypatch):
    # ログの保存が遅くても、完成した要約は上限内に返る（保存は待たない）
    import main
    from services import research

    monkeypatch.setenv("RAG_WEB_SEARCH_DEADLINE_SEC", "0.5")
    monkeypatch.setattr(main, "_generate_search_queries", lambda c, j: ["q1"])
    monkeypatch.setattr(main, "_web_search_openai", lambda q: "結果")
    monkeypatch.setattr(main, "rank_results_by_domain_trust", lambda r, c: r)
    monkeypatch.setattr(main, "_summarize_for_hiring", lambda c, j, r: "要約")
    saved = threading.Event()

    def slow_save(*args):
        time.sleep(1)
        saved.set()

    monkeypatch.setattr(main, "_save_search_log", slow_save)
    started = time.monotonic()
    assert asyncio.run(research._run_web_search_pipeline("A社", "")) == "要約"
    assert time.monotonic() - started < 0.5
    assert saved.wait(3), "ログは後から保存される"
