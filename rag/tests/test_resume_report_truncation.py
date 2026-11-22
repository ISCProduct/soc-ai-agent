"""履歴書 RAG レポートが出力上限で切れたときの扱い（SOCAIAGENT-702）。

非stream は上限を倍にして1回だけ作り直し、それでも切れたら「途中で切れた」旨を本文の末尾に出す
（Go の requestReviewJSON と同じ方針）。stream は作り直さず、表示だけ足す。
"""
import asyncio
import json
from unittest.mock import MagicMock, patch

import pytest

from models import ReviewRequest
from routers.resume import review_resume_stream
from services.resume_report import (
    RESUME_REPORT_MAX_TOKENS,
    RESUME_REPORT_TRUNCATED_NOTICE,
    generate_resume_report,
)


def _resp(text: str, finish_reason: str) -> MagicMock:
    return MagicMock(choices=[MagicMock(message=MagicMock(content=text), finish_reason=finish_reason)])


@pytest.mark.parametrize(
    "finish_reasons, want_calls, want_notice",
    [
        (["stop"], 1, False),
        (["length", "stop"], 2, False),
        (["length", "length"], 2, True),
    ],
)
def test_generate_resume_report_retries_once_with_doubled_limit(monkeypatch, finish_reasons, want_calls, want_notice):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.side_effect = [_resp(f"レポート{i}", r) for i, r in enumerate(finish_reasons)]
    with patch("services.resume_report.OpenAI", return_value=client):
        report = generate_resume_report(ReviewRequest(resume_text="履歴書", company_name="テスト株式会社"), [], "none", 10000)

    calls = [c.kwargs for c in client.chat.completions.create.call_args_list]
    assert len(calls) == want_calls
    assert calls[0]["max_tokens"] == RESUME_REPORT_MAX_TOKENS
    if want_calls == 2:
        assert calls[1]["max_tokens"] == RESUME_REPORT_MAX_TOKENS * 2
    assert report.startswith(f"レポート{want_calls - 1}")
    assert report.endswith(RESUME_REPORT_TRUNCATED_NOTICE) is want_notice


def _chunk(text: str | None, finish_reason: str | None) -> MagicMock:
    return MagicMock(choices=[MagicMock(delta=MagicMock(content=text), finish_reason=finish_reason)])


@pytest.mark.parametrize("finish_reason, want_notice", [("stop", False), ("length", True)])
def test_stream_shows_notice_only_when_truncated(monkeypatch, finish_reason, want_notice):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.return_value = iter([_chunk("本文", None), _chunk(None, finish_reason)])
    with patch("main._gather_context", return_value=([], "none")), \
         patch("main.RESUME_REVIEW_INPUT_CHAR_LIMIT", 10000), \
         patch("routers.resume.OpenAI", return_value=client):
        response = review_resume_stream(ReviewRequest(resume_text="履歴書", company_name="テスト株式会社"))

        async def _collect() -> list[dict]:
            events = []
            async for raw in response.body_iterator:
                line = raw.decode() if isinstance(raw, bytes) else raw
                events.append(json.loads(line.removeprefix("data: ").strip()))
            return events

        events = asyncio.run(_collect())

    # 作り直さない
    assert client.chat.completions.create.call_count == 1
    texts = [e["text"] for e in events if e["type"] == "chunk"]
    assert texts[0] == "本文"
    assert (RESUME_REPORT_TRUNCATED_NOTICE in texts) is want_notice
    assert events[-1]["type"] == "done"
