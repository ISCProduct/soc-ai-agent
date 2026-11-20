"""ES添削エンドポイント。"""
from __future__ import annotations

import logging
from concurrent.futures import ThreadPoolExecutor
from typing import List

from fastapi import APIRouter

from models import ESReviewRequest, ESReviewResponse
from services.sanitize import _sanitize_company_name_for_query
from vector_store import build_cache_key

logger = logging.getLogger("main")

router = APIRouter()


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
            context_docs = m.get_cached_context(
                cache_key, query=f"{safe_company_name} 求める人物像 採用 価値観"
            )
            if context_docs:
                context_source = "cache"
        if not context_docs and m.ALLOW_WEB_SEARCH_FALLBACK:
            logger.info("es review web search start company=%s", safe_company_name)
            try:
                # 軽量Web Searchパイプラインを2回（一般/エンジニア向け）同時に実行する。
                # 検索件数・要約は従来どおり。片方が例外なら従来同様 except で握る。
                # _run_async はスレッドごとに独立したイベントループを使うので並列にできる。
                with ThreadPoolExecutor(max_workers=2) as pool:
                    f_general = pool.submit(m._run_async, m._run_web_search_pipeline, safe_company_name, "")
                    f_engineer = pool.submit(m._run_async, m._run_web_search_pipeline, safe_company_name, "エンジニア")
                    summary_general = f_general.result()
                    summary_engineer = f_engineer.result()
                parts = [s for s in (summary_general, summary_engineer) if s]
                if parts:
                    combined = "\n\n".join(parts)
                    # 先頭2000文字に制限してコンテキストに注入（プロンプト長を考慮）
                    if len(combined) > 2000:
                        combined = combined[:2000]
                    context_docs = [combined]
                    context_source = "web_search"
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
        company_context_source=context_source,
        tech_stack=request.tech_stack,
        char_limit=request.char_limit,
        char_limit_mode=request.char_limit_mode,
        include_feedback_items=request.include_feedback_items,
    )
