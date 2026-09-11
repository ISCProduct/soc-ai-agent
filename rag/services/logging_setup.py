"""構造化 JSON ログと trace_id ContextVar。"""
from __future__ import annotations

import contextvars
import json
import logging
import os
from typing import Any

_trace_id_var: contextvars.ContextVar[str] = contextvars.ContextVar("trace_id", default="")

_LOG_LEVEL_MAP = {
    "DEBUG": logging.DEBUG,
    "INFO": logging.INFO,
    "WARN": logging.WARNING,
    "WARNING": logging.WARNING,
    "ERROR": logging.ERROR,
}
_log_level = _LOG_LEVEL_MAP.get(os.getenv("LOG_LEVEL", "INFO").upper(), logging.INFO)


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "time": self.formatTime(record, "%Y-%m-%dT%H:%M:%S"),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }
        trace_id = _trace_id_var.get("")
        if trace_id:
            payload["trace_id"] = trace_id
        if record.exc_info:
            payload["exc_info"] = self.formatException(record.exc_info)
        return json.dumps(payload, ensure_ascii=False)


def setup_logging() -> logging.Logger:
    handler = logging.StreamHandler()
    handler.setFormatter(JsonFormatter())
    root = logging.getLogger()
    root.setLevel(_log_level)
    root.handlers = [handler]
    return logging.getLogger("main")


# 後方互換エイリアス（旧 main._JsonFormatter）
_JsonFormatter = JsonFormatter
