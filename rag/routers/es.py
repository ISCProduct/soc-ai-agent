"""ES添削エンドポイント。"""
from __future__ import annotations

import logging
from typing import List

from fastapi import APIRouter

from models import ESReviewRequest, ESReviewResponse
from services.sanitize import _sanitize_company_name_for_query

logger = logging.getLogger("main")

router = APIRouter()


@router.post("/es/review", response_model=ESReviewResponse)
def es_review(request: ESReviewRequest) -> ESReviewResponse:
    import main as m

    context_docs: List[str] = []
    safe_company_name = ""
    if request.company_name.strip():
        safe_company_name = _sanitize_company_name_for_query(request.company_name)
        cache_key = "{company}::es_review".format(company=safe_company_name)
        if request.company_context:
            context_docs = [request.company_context]
            m.set_cached_context(
                cache_key,
                context_docs,
                source="company_brief",
                doc_type="es_review",
            )
        else:
            context_docs = m.get_cached_context(
                cache_key, query=f"{safe_company_name} 求める人物像 採用 価値観"
            )
        if not context_docs and m.ALLOW_WEB_SEARCH_FALLBACK:
            logger.info("es review web search start company=%s", safe_company_name)
            try:
                # 段階的に2回の軽量Web Searchパイプラインを実行して、採用情報（一般）とエンジニア向け観点を取得する
                summary_general = m._run_async(m._run_web_search_pipeline, safe_company_name, "")
                summary_engineer = m._run_async(m._run_web_search_pipeline, safe_company_name, "エンジニア")
                parts = [s for s in (summary_general, summary_engineer) if s]
                if parts:
                    combined = "\n\n".join(parts)
                    # 先頭2000文字に制限してコンテキストに注入（プロンプト長を考慮）
                    if len(combined) > 2000:
                        combined = combined[:2000]
                    context_docs = [combined]
                    m.set_cached_context(
                        cache_key,
                        context_docs,
                        source="web_search",
                        doc_type="es_review",
                    )
            except Exception as exc:
                logger.warning("es review web search failed company=%s error=%s", safe_company_name, exc)
        elif not context_docs:
            logger.info(
                "es review cache miss; web_search skipped company=%s",
                safe_company_name,
            )

    return m._run_es_review(
        es_text=request.es_text,
        question_type=request.question_type,
        company_name=safe_company_name,
        context_docs=context_docs,
    )
