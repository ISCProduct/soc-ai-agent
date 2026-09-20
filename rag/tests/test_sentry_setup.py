"""Sentry スクラブの単体テスト（#1185）。"""
from __future__ import annotations

from services.sentry_setup import _scrub_event


def test_scrub_event_removes_secrets():
    event = {
        "request": {
            "data": {"resume_text": "秘密"},
            "cookies": {"session": "abc"},
            "query_string": "email=a@b.com",
            "headers": {
                "Authorization": "Bearer secret",
                "Cookie": "a=1",
                "X-Internal-Token": "rag-secret",
                "X-Request-ID": "keep-me",
                "Content-Type": "application/json",
            },
        }
    }
    out = _scrub_event(event, None)
    assert out is not None
    req = out["request"]
    assert "data" not in req
    assert "cookies" not in req
    assert req["query_string"] == ""
    assert "Authorization" not in req["headers"]
    assert "Cookie" not in req["headers"]
    assert "X-Internal-Token" not in req["headers"]
    assert req["headers"]["X-Request-ID"] == "keep-me"


def test_scrub_event_without_request():
    event = {"message": "x"}
    assert _scrub_event(event, None) == event
