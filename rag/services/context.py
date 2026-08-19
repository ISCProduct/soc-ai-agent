"""履歴書レビュー用のコンテキスト収集。"""
from __future__ import annotations

import logging
from typing import List, Tuple

from fastapi import HTTPException

from models import ReviewRequest
from services.sanitize import _sanitize_company_name_for_query
from vector_store import build_cache_key

logger = logging.getLogger("main")


def _gather_context(request: ReviewRequest) -> Tuple[List[str], str]:
    """RAGコンテキストを収集し (docs, context_source) を返す。

    優先順: company_context(brief) → Chroma キャッシュ →（オプトイン時）Deep Research / Web Search
    """
    import main as m

    safe_company_name = _sanitize_company_name_for_query(request.company_name)
    role_label = request.job_title or "指定なし"
    # #938: キャッシュキーはサニタイズ前の企業名のハッシュで衝突回避する
    cache_key = build_cache_key(
        "resume_review", safe_company_name, role_label, company_original=request.company_name
    )

    if request.company_context:
        docs = [request.company_context]
        m.set_cached_context(
            cache_key, docs, source="company_brief", doc_type="resume_review"
        )
        return docs, "company_brief"

    # キャッシュヒット: 即時返却
    try:
        retrieved = m.get_cached_context(cache_key)
    except Exception as exc:
        logger.error("get_cached_context failed error=%s", exc, exc_info=True)
        retrieved = None
    if retrieved:
        return retrieved, "cache"

    # Deep Research (o3-deep-research)
    if m.USE_DEEP_RESEARCH and safe_company_name:
        try:
            report = m.run_deep_research(safe_company_name, role_label)
            if report:
                retrieved = [report]
                m.set_cached_context(
                    cache_key,
                    retrieved,
                    source="deep_research",
                    doc_type="resume_review",
                )
                return retrieved, "deep_research"
            logger.warning("deep research returned empty result")
        except Exception as exc:
            logger.error("deep research failed error=%s", exc, exc_info=True)
            if m.STRICT_DEEP_RESEARCH:
                raise HTTPException(status_code=502, detail="Deep Research failed")

    # OpenAI Web Searchパイプライン（クエリ生成→並列検索→LLM要約→キャッシュ保存）
    if not retrieved and m.ALLOW_WEB_SEARCH_FALLBACK and safe_company_name:
        logger.info("web search pipeline start company=%s role=%s", safe_company_name, role_label)
        try:
            summary = m._run_async(m._run_web_search_pipeline, safe_company_name, role_label)
            if summary:
                retrieved = [summary]
                m.set_cached_context(
                    cache_key,
                    retrieved,
                    source="web_search",
                    doc_type="resume_review",
                )
                return retrieved, "web_search"
            logger.warning("web search pipeline returned empty result company=%s", safe_company_name)
        except Exception as exc:
            logger.error("web search pipeline failed error=%s", exc, exc_info=True)

    return retrieved or [], "none"
