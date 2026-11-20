"""ES添削のWeb検索2回(一般/エンジニア)が同時実行されることのテスト(SOCAIAGENT-682)。"""
import asyncio
import time
from unittest.mock import MagicMock, patch

import pytest

from models import ESReviewRequest
from routers.es import es_review

_DELAY = 0.2


def _call(pipeline):
    run_review = MagicMock(return_value="ok")
    with patch("main._run_web_search_pipeline", pipeline), \
         patch("main.get_cached_context", return_value=[]), \
         patch("main.set_cached_context") as set_cache, \
         patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
         patch("main._run_es_review", run_review):
        es_review(ESReviewRequest(es_text="本文", company_name="テスト株式会社"))
    return run_review.call_args.kwargs, set_cache


def test_両方呼ばれ結合順序と内容が維持される():
    calls = []

    async def pipeline(company, job):
        calls.append(job)
        return f"summary[{job}]"

    kwargs, set_cache = _call(pipeline)
    assert sorted(calls) == ["", "エンジニア"]
    assert kwargs["context_docs"] == ["summary[]\n\nsummary[エンジニア]"]
    assert kwargs["company_context_source"] == "web_search"
    set_cache.assert_called_once()


def test_片方が空なら残りだけ使う():
    async def pipeline(company, job):
        return "" if job == "" else "eng"

    kwargs, _ = _call(pipeline)
    assert kwargs["context_docs"] == ["eng"]


def test_結合結果は2000文字で切られる():
    async def pipeline(company, job):
        return "a" * 1500

    kwargs, _ = _call(pipeline)
    assert len(kwargs["context_docs"][0]) == 2000


@pytest.mark.parametrize("failing_job", ["", "エンジニア"])
def test_片方が例外ならコンテキストなしで続行する(failing_job):
    async def pipeline(company, job):
        if job == failing_job:
            raise RuntimeError("boom")
        return "ok"

    kwargs, set_cache = _call(pipeline)
    assert kwargs["context_docs"] == []
    assert kwargs["company_context_source"] == "none"
    set_cache.assert_not_called()


def test_2回が重なって実行される():
    async def pipeline(company, job):
        await asyncio.sleep(_DELAY)
        return "x"

    t = time.monotonic()
    _call(pipeline)
    assert time.monotonic() - t < _DELAY * 1.7
