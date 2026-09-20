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
    return event


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
