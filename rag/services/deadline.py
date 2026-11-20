"""Web Search パイプライン全体の上限時間（SOCAIAGENT-558）。

パイプライン（クエリ生成＋最大4検索＋要約）の各呼び出しには OPENAI_TIMEOUT_SEC（60秒）があるが、
全体の上限が無かった。検索と要約が順に遅れると全体で数分になり、手前の CloudFront / ALB（90秒）や
Backend（85秒、#1556）で切られて、結果が届かないまま費用だけがかかる。

上限を過ぎたら空文字を返し、呼び出し側は「Web Search の結果なし」として先へ進む
（ES 添削なら企業情報なしで添削する）。費用のかかる途中の呼び出しは止められないが（スレッドは
中断できない）、待たずに返すので、利用者には時間内に結果が届く。
"""
from __future__ import annotations

import asyncio
import logging
import os
from concurrent.futures import ThreadPoolExecutor
from typing import Awaitable, Callable

logger = logging.getLogger("rag-review")

# 既定は40秒。経路で最小の90秒（CloudFront / ALB）から、Web Search の後に続く処理
# （ES 添削の評価と改善文で 20〜40 秒）と余裕を引いた値。
DEFAULT_WEB_SEARCH_DEADLINE_SEC = 40.0


def web_search_deadline_sec() -> float:
    """RAG_WEB_SEARCH_DEADLINE_SEC。0 以下なら上限なし。読めなければ既定。"""
    raw = os.getenv("RAG_WEB_SEARCH_DEADLINE_SEC", "").strip()
    if not raw:
        return DEFAULT_WEB_SEARCH_DEADLINE_SEC
    try:
        return float(raw)
    except ValueError:
        logger.warning("invalid RAG_WEB_SEARCH_DEADLINE_SEC=%r; using %.0f", raw, DEFAULT_WEB_SEARCH_DEADLINE_SEC)
        return DEFAULT_WEB_SEARCH_DEADLINE_SEC


async def run_with_deadline(
        steps: Callable[[ThreadPoolExecutor], Awaitable[str]], label: str, company_name: str
) -> str:
    """steps を上限時間つきで走らせる。上限を過ぎたら空文字を返す。

    steps には専用のスレッドプールを渡す。終わったら（上限切れでも）待たずに手放すので、
    `with ThreadPoolExecutor()` の出口や asyncio.run の終了処理（既定のプールの join）で
    上限を過ぎてもスレッドを待ち続ける、ということが起きない。
    """
    deadline = web_search_deadline_sec()
    executor = ThreadPoolExecutor(thread_name_prefix=f"rag-{label}")
    try:
        if deadline <= 0:
            return await steps(executor)
        return await asyncio.wait_for(steps(executor), timeout=deadline)
    except asyncio.TimeoutError:
        logger.warning("%s pipeline exceeded deadline %.0fs company=%s; continuing without web search",
                       label, deadline, company_name)
        return ""
    finally:
        executor.shutdown(wait=False, cancel_futures=True)
