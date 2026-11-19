"""履歴書レビューの SSE は、失敗時に例外文を利用者へ返さず固定文言にする（SOCAIAGENT-664）。"""
from __future__ import annotations

import asyncio
import json
from unittest.mock import MagicMock, patch

from models import ReviewRequest
from routers.resume import REVIEW_STREAM_ERROR_MESSAGE, review_resume_stream


def _collect(streaming_response) -> list[dict]:
    async def _consume() -> list[dict]:
        events = []
        async for raw in streaming_response.body_iterator:
            text = raw if isinstance(raw, str) else raw.decode()
            for line in text.splitlines():
                if line.startswith("data: "):
                    events.append(json.loads(line[6:]))
        return events

    return asyncio.run(_consume())


def test_stream_error_uses_fixed_message(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.side_effect = RuntimeError("RateLimitError: key sk-secret org-xyz")

    request = ReviewRequest(resume_text="経歴: エンジニア", company_name="テスト株式会社")
    with patch("main._gather_context", return_value=([], "web_search")), \
         patch("main.RESUME_REVIEW_INPUT_CHAR_LIMIT", 10000), \
         patch("routers.resume.OpenAI", return_value=client):
        events = _collect(review_resume_stream(request))

    errors = [e for e in events if e["type"] == "error"]
    assert errors == [{"type": "error", "message": REVIEW_STREAM_ERROR_MESSAGE}]
    assert "sk-secret" not in json.dumps(events, ensure_ascii=False)


def test_stream_without_api_key_uses_fixed_message(monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    request = ReviewRequest(resume_text="経歴: エンジニア", company_name="テスト株式会社")
    with patch("main._gather_context", return_value=([], "web_search")), \
         patch("main.RESUME_REVIEW_INPUT_CHAR_LIMIT", 10000):
        events = _collect(review_resume_stream(request))

    assert events == [{"type": "error", "message": REVIEW_STREAM_ERROR_MESSAGE}]
