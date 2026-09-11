"""同期コンテキストから非同期関数を実行するユーティリティ。"""
from __future__ import annotations

import asyncio
from typing import Any, Awaitable, Callable, TypeVar

T = TypeVar("T")


def _run_async(async_func: Callable[..., Awaitable[T]], *args: Any) -> T:
    """同期コンテキストから非同期関数を実行する。"""
    return asyncio.run(async_func(*args))
