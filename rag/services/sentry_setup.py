"""Sentry 初期化（#619 / #1185）。

SENTRY_DSN 未設定時は no-op。送信前に Authorization / Cookie / リクエストデータを落とす。
"""
from __future__ import annotations

import logging
import os
from typing import Any

logger = logging.getLogger(__name__)


def _scrub_event(event: dict[str, Any], _hint: dict[str, Any] | None) -> dict[str, Any] | None:
    request = event.get("request")
    if isinstance(request, dict):
        request.pop("data", None)
        request.pop("cookies", None)
        request["query_string"] = ""
        headers = request.get("headers")
        if isinstance(headers, dict):
            for key in list(headers.keys()):
                lower = key.lower()
                if lower in {
                    "authorization",
                    "cookie",
                    "x-admin-token",
                    "x-user-token",
                    "x-company-user-token",
                    "x-internal-token",
                }:
                    headers.pop(key, None)

    # logger.error() 経由のイベントは LoggingIntegration が本文をそのまま載せる。
    # crew.py の "output=%s"（LLM 出力を1000字）のような経路があるので頭だけ残す。
    for entry in event.get("exception", {}).get("values", []) or []:
        if isinstance(entry, dict) and isinstance(entry.get("value"), str):
            entry["value"] = _truncate(entry["value"])
    if isinstance(event.get("message"), str):
        event["message"] = _truncate(event["message"])
    log_entry = event.get("logentry")
    if isinstance(log_entry, dict) and isinstance(log_entry.get("message"), str):
        log_entry["message"] = _truncate(log_entry["message"])
    return event


# 原因の特定には先頭で足り、それ以上は本文の持ち出しになりやすい。
MAX_SENTRY_MESSAGE_CHARS = 300


def _truncate(message: str) -> str:
    if len(message) <= MAX_SENTRY_MESSAGE_CHARS:
        return message
    return message[:MAX_SENTRY_MESSAGE_CHARS] + "…(truncated)"


def init_sentry() -> bool:
    dsn = os.getenv("SENTRY_DSN", "").strip()
    if not dsn:
        return False

    import sentry_sdk
    from sentry_sdk.integrations.fastapi import FastApiIntegration
    from sentry_sdk.integrations.starlette import StarletteIntegration

    env = os.getenv("APP_ENV", "").strip() or "development"
    release = os.getenv("SENTRY_RELEASE", "").strip() or None

    sentry_sdk.init(
        dsn=dsn,
        environment=env,
        release=release,
        send_default_pii=False,
        traces_sample_rate=0.0,
        before_send=_scrub_event,
        integrations=[
            StarletteIntegration(transaction_style="endpoint"),
            FastApiIntegration(transaction_style="endpoint"),
        ],
    )
    logger.info("sentry enabled environment=%s release=%s", env, release or "-")
    return True
