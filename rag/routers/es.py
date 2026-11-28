"""ES添削エンドポイント。"""
from __future__ import annotations

import logging
from concurrent.futures import ThreadPoolExecutor
from typing import Dict, List, Optional

from fastapi import APIRouter

from models import ESReviewRequest, ESReviewResponse
from services.sanitize import _sanitize_company_name_for_query
from vector_store import build_cache_key

logger = logging.getLogger("main")

router = APIRouter()


# 観点ごとの Web Search の職種（パイプラインの job_title）と、キャッシュキーの役割。
# 一般は今までのキー（役割なし）。エンジニア向けは別のキーにし、技術スタックの有無で
# キャッシュの中身が来た順に変わらないようにする（スタック無しの要約が先に入ると、後から来た
# エンジニア向けの ES も TTL の間は一般だけになる、という食い違いを防ぐ）。
_ES_VIEWS = {
    "general": ("", "general"),
    "engineer": ("エンジニア", "engineer"),
}


def _es_web_search(m, company_name: str, views: List[str]) -> Dict[str, Optional[str]]:
    """ES 添削の企業情報を、観点ごとに Web Search で取る（SOCAIAGENT-553）。

    以前は「一般」と「エンジニア」の2本を、職種に関係なく毎回走らせていた。1企業の初回で
    Web Search が倍になり、エンジニア以外の ES でもエンジニア向けの検索を払っていた。
    ここでは呼び出し側が決めた観点（キャッシュに無いものだけ）を走らせる。

    2本のときは #2440 と同じく、スレッドごとに _run_async（独立したイベントループ）で並列にする。
    片方が例外なら例外を投げ、呼び出し側が企業情報なしで続ける（#2440 と同じ扱い）。
    """
    if len(views) == 1:
        job, _ = _ES_VIEWS[views[0]]
        return {views[0]: m._run_async(m._run_web_search_pipeline, company_name, job)}
    with ThreadPoolExecutor(max_workers=len(views)) as pool:
        futures = {
            v: pool.submit(m._run_async, m._run_web_search_pipeline, company_name, _ES_VIEWS[v][0])
            for v in views
        }
        return {v: f.result() for v, f in futures.items()}


@router.post("/es/review", response_model=ESReviewResponse)
def es_review(request: ESReviewRequest) -> ESReviewResponse:
    import main as m

    context_docs: List[str] = []
    safe_company_name = ""
    # #1524: 企業コンテキストの取得元をレスポンスへ返す（0件なら "none"）
    context_source = "none"
    if request.company_name.strip():
        safe_company_name = _sanitize_company_name_for_query(request.company_name)
        # #938: キャッシュキーはサニタイズ前の企業名のハッシュで衝突回避する
        cache_key = build_cache_key(
            "es_review", safe_company_name, company_original=request.company_name
        )
        if request.company_context:
            context_docs = [request.company_context]
            context_source = "company_brief"
            m.set_cached_context(
                cache_key,
                context_docs,
                source="company_brief",
                doc_type="es_review",
            )
        else:
            # 一般の観点は常に、エンジニア向けは技術スタックがあるときだけ（観点ごとに別のキャッシュ）
            views = ["general"] + (["engineer"] if request.tech_stack.strip() else [])
            keys = {
                v: build_cache_key("es_review", safe_company_name, _ES_VIEWS[v][1], company_original=request.company_name)
                for v in views
            }
            query = f"{safe_company_name} 求める人物像 採用 価値観"
            parts: Dict[str, List[str]] = {}
            for v in views:
                # 両方の観点とも、キーが無いときに同じ企業の別の役割の文書で代用しない。一般の観点が
                # エンジニア向けの要約を拾うと、スタック無しの ES がそれを一般の企業情報として使うため。
                # 無ければ Web Search に進む
                docs = m.get_cached_context(keys[v], query=query, allow_company_fallback=False)
                if docs:
                    parts[v] = docs
            missing = [v for v in views if v not in parts]
            searched = False
            if missing and m.ALLOW_WEB_SEARCH_FALLBACK:
                logger.info("es review web search start company=%s views=%s", safe_company_name, missing)
                try:
                    summaries = _es_web_search(m, safe_company_name, missing)
                    for v, summary in summaries.items():
                        if summary:
                            parts[v] = [summary]
                            m.set_cached_context(keys[v], [summary], source="web_search", doc_type="es_review")
                    searched = any(summaries.get(v) for v in missing)
                except Exception as exc:
                    logger.warning("es review web search failed company=%s error=%s", safe_company_name, exc)
            elif missing:
                logger.info(
                    "es review cache miss; web_search skipped company=%s views=%s",
                    safe_company_name, missing,
                )
            combined = "\n\n".join("\n\n".join(parts[v]) for v in views if v in parts)
            if combined:
                # 先頭2000文字に制限してコンテキストに注入（プロンプト長を考慮）
                context_docs = [combined[:2000]]
                context_source = "web_search" if searched else "cache"

    return m._run_es_review(
        es_text=request.es_text,
        question_type=request.question_type,
        company_name=safe_company_name,
        context_docs=context_docs,
        company_context_source=context_source,
        tech_stack=request.tech_stack,
        char_limit=request.char_limit,
        char_limit_mode=request.char_limit_mode,
        include_feedback_items=request.include_feedback_items,
    )
