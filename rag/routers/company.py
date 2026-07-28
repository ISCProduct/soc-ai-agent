"""企業ヒント・企業コンテキストエンドポイント。"""
from __future__ import annotations

import logging

from fastapi import APIRouter, HTTPException

from models import (
    CompanyContextRequest,
    CompanyContextResponse,
    CompanyHintsRequest,
    CompanyHintsResponse,
)
from services.sanitize import _sanitize_company_name_for_query
from vector_store import upsert_by_doc_type

logger = logging.getLogger("main")

router = APIRouter()


@router.post("/company/hints", response_model=CompanyHintsResponse)
def company_hints(request: CompanyHintsRequest) -> CompanyHintsResponse:
    import main as m

    # 入力値サニタイズ（#331: クエリインジェクション対策）
    safe_company_name = _sanitize_company_name_for_query(request.company_name)
    role_label = request.position or "一般職"
    cache_key = "hints::{company}::{role}".format(
        company=safe_company_name, role=role_label
    )

    # Backend 共有 brief があれば Search せず構造化（Phase 1B）
    if request.company_context:
        m.set_cached_context(
            cache_key,
            [request.company_context],
            source="company_brief",
            doc_type="interview_hints",
        )
        return m._parse_hints_from_text(safe_company_name, role_label, request.company_context)

    # キャッシュヒット: そのまま構造化して返す
    retrieved = m.get_cached_context(
        cache_key, query=f"{safe_company_name} 面接 よく聞かれる質問"
    )
    if retrieved:
        result = m._parse_hints_from_text(safe_company_name, role_label, "\n\n".join(retrieved))
        result.cached = True
        return result

    # 既定はキャッシュ強制。明示オプトイン時のみ Web Search
    research_text = ""
    if m.ALLOW_WEB_SEARCH_FALLBACK:
        research_text = m._run_hints_web_search(safe_company_name, role_label) or ""
        if research_text:
            m.set_cached_context(
                cache_key, [research_text], source="web_search", doc_type="interview_hints"
            )
    else:
        logger.info(
            "hints cache miss; web_search skipped company=%s (RAG_ALLOW_WEB_SEARCH_FALLBACK=false)",
            safe_company_name,
        )

    return m._parse_hints_from_text(safe_company_name, role_label, research_text)


@router.post("/company/context", response_model=CompanyContextResponse)
def upsert_company_context(request: CompanyContextRequest) -> CompanyContextResponse:
    """バックエンドが取得した求人情報・人物像をベクトルDBへ横断書き込みする。"""
    import main as m

    safe_company = _sanitize_company_name_for_query(request.company_name)
    docs = [request.content]
    # jobs / persona は設計上の source=job_fetch に正規化
    raw_type = (request.context_type or "general").strip().lower()
    source = "job_fetch" if raw_type in {"jobs", "persona", "general", "job_fetch"} else raw_type

    try:
        embeddings = m.embed_texts(docs)
    except Exception as exc:
        logger.exception("company context embed failed company=%s error=%s", safe_company, exc)
        raise HTTPException(status_code=500, detail="embedding failed") from exc

    # 用途横断: 企業単位（role=general）で各コレクションへ1回ずつ書き込み
    # 職種指定クエリは company fallback で再利用できる
    targets = [
        ("company_research", "指定なし"),
        ("es_review", "general"),
        ("interview_hints", "一般職"),
    ]
    keys_updated = 0
    for doc_type, role in targets:
        upsert_by_doc_type(
            company=safe_company,
            role=role,
            doc_type=doc_type,
            docs=docs,
            embeddings=embeddings,
            source=source,
        )
        keys_updated += 1

    logger.info(
        "company context upserted company=%s type=%s source=%s keys=%d chars=%d",
        safe_company, request.context_type, source, keys_updated, len(request.content),
    )
    return CompanyContextResponse(
        status="ok",
        company=safe_company,
        context_type=request.context_type,
        keys_updated=keys_updated,
    )
