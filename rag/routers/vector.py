"""ベクトルストア管理エンドポイント。"""
from __future__ import annotations

import logging
from typing import Optional

from fastapi import APIRouter, HTTPException

from models import VectorReembedRequest, VectorReembedResponse, VectorStatusResponse
from services.sanitize import _sanitize_company_name_for_query
from vector_store import build_cache_key, delete_company_documents, get_index_status

logger = logging.getLogger("main")

router = APIRouter()


@router.get("/vector/status", response_model=VectorStatusResponse)
def vector_status(company: Optional[str] = None) -> dict:
    """ベクトルDBのインデックス状況を返す（管理・監視用）。"""
    company_filter = None
    if company and company.strip():
        company_filter = _sanitize_company_name_for_query(company)
    try:
        return get_index_status(company_filter)
    except Exception as exc:
        logger.exception("vector status failed error=%s", exc)
        raise HTTPException(status_code=503, detail=f"vector status failed: {exc}") from exc


@router.post("/vector/reembed", response_model=VectorReembedResponse)
def vector_reembed(request: VectorReembedRequest) -> VectorReembedResponse:
    """企業ベクトルを削除し、必要なら WebSearch で再埋め込みする。"""
    import main as m

    safe_company = _sanitize_company_name_for_query(request.company_name)
    doc_type = (request.doc_type or "").strip() or None
    if doc_type and doc_type not in {
        "resume_review",
        "company_research",
        "interview_hints",
        "es_review",
    }:
        raise HTTPException(status_code=400, detail="invalid doc_type")

    try:
        deleted_info = delete_company_documents(safe_company, doc_type=doc_type)
    except Exception as exc:
        logger.exception("vector delete failed company=%s error=%s", safe_company, exc)
        raise HTTPException(status_code=503, detail=f"vector delete failed: {exc}") from exc

    sources: list[str] = []
    refreshed = False
    if request.refresh:
        try:
            if doc_type in (None, "company_research", "resume_review"):
                summary = m._run_async(m._run_web_search_pipeline, safe_company, "指定なし")
                if summary:
                    m.set_cached_context(
                        build_cache_key("company_research", safe_company, "指定なし"),
                        [summary],
                        source="web_search",
                        doc_type="company_research",
                    )
                    sources.append("company_research")
                    refreshed = True
            if doc_type in (None, "interview_hints"):
                hints_text = m._run_hints_web_search(safe_company, "一般職")
                if hints_text:
                    m.set_cached_context(
                        build_cache_key("interview_hints", safe_company, "一般職"),
                        [hints_text],
                        source="web_search",
                        doc_type="interview_hints",
                    )
                    sources.append("interview_hints")
                    refreshed = True
            if doc_type in (None, "es_review"):
                summary = m._run_async(m._run_web_search_pipeline, safe_company, "")
                if summary:
                    m.set_cached_context(
                        build_cache_key("es_review", safe_company),
                        [summary],
                        source="web_search",
                        doc_type="es_review",
                    )
                    sources.append("es_review")
                    refreshed = True
        except Exception as exc:
            logger.exception("vector reembed refresh failed company=%s error=%s", safe_company, exc)
            raise HTTPException(status_code=502, detail=f"reembed refresh failed: {exc}") from exc

    return VectorReembedResponse(
        status="ok",
        company=safe_company,
        deleted=int(deleted_info.get("deleted") or 0),
        collections=deleted_info.get("collections") or {},
        refreshed=refreshed,
        sources=sources,
    )
