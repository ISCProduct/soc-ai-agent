"""RAG Review API エントリポイント。

uvicorn main:app および `import main` / patch("main.*") 互換のため、
サービス層のシンボルをこのモジュール名前空間へ再エクスポートする。
"""
from __future__ import annotations

import datetime
import hmac
import json
import logging
import os
import time
import uuid
from typing import Any, Callable

import openai as openai_module
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from openai import OpenAI

from models import (  # noqa: F401 — re-export for import main / tests
    CompanyContextRequest,
    CompanyContextResponse,
    CompanyHintsRequest,
    CompanyHintsResponse,
    ESReviewRequest,
    ESReviewResponse,
    ReviewRequest,
    ReviewResponse,
    VectorReembedRequest,
    VectorReembedResponse,
    VectorStatusResponse,
)
from services.async_utils import _run_async  # noqa: F401
from services.cache import get_cached_context, set_cached_context  # noqa: F401
from services.context import _gather_context  # noqa: F401
from services.crew import run_crewai  # noqa: F401
from services.embed import (  # noqa: F401
    _truncate_text,
    cosine_similarity,
    embed_texts,
    retrieve_docs,
)
from services.es_review import _run_es_review  # noqa: F401
from services.hints import (  # noqa: F401
    _parse_hints_from_text,
    _run_hints_web_search,
    _run_hints_web_search_pipeline,
)
from services.logging_setup import (
    _JsonFormatter,
    _trace_id_var,
    setup_logging,
)
from services.research import (  # noqa: F401
    _domain_trust_score,
    _extract_domains_from_text,
    _generate_search_queries,
    _run_web_search_pipeline,
    _save_search_log,
    _summarize_for_hiring,
    _web_search_openai,
    extract_output_text,
    rank_results_by_domain_trust,
    run_deep_research,
)
from services.sanitize import (  # noqa: F401
    _sanitize_company_name_for_query,
    _sanitize_job_title,
)
from services.settings import (  # noqa: F401 — re-export; patches target main.*
    ALLOW_WEB_SEARCH_FALLBACK,
    CACHE_TTL_SECONDS,
    CREWAI_VERBOSE,
    DEFAULT_CACHE_TTL_SECONDS,
    DEFAULT_EMBED_MAX_RETRIES,
    DEFAULT_HINTS_PARSE_MAX_TOKENS,
    DEFAULT_MAX_EMBED_TOKENS,
    DEFAULT_RESUME_REVIEW_INPUT_CHAR_LIMIT,
    DEFAULT_SEARCH_LOG_DIR,
    DEFAULT_WEB_SEARCH_MODEL,
    EMBED_MAX_RETRIES,
    HINTS_PARSE_MAX_TOKENS,
    INTERNAL_TOKEN_HEADER,
    MAX_EMBED_TOKENS,
    OPENAI_TIMEOUT_SEC,
    RESUME_REVIEW_INPUT_CHAR_LIMIT,
    SEARCH_LOG_DIR,
    STRICT_DEEP_RESEARCH,
    USE_DEEP_RESEARCH,
    WEB_SEARCH_MODEL,
    _INTERNAL_AUTH_EXEMPT_PATHS,
)
from vector_store import (  # noqa: E402, F401
    _sanitize_collection_name,
    build_cache_key,
    delete_company_documents,
    describe_backend as describe_chroma_backend,
    get_cached_documents,
    get_chroma_client,
    get_index_status,
    ping_chroma,
    set_cached_documents,
    upsert_by_doc_type,
)

logger = setup_logging()

app = FastAPI()

# Training export endpoints (registered from training_api.py)
import training_api  # noqa: E402

training_api.register(app)


@app.middleware("http")
async def _internal_auth_middleware(request: Request, call_next: Callable) -> Any:
    if request.url.path in _INTERNAL_AUTH_EXEMPT_PATHS:
        return await call_next(request)

    expected = os.getenv("RAG_INTERNAL_TOKEN", "").strip()
    if not expected:
        logger.error("RAG_INTERNAL_TOKEN が未設定のためリクエストを拒否します（フェイルクローズ）")
        return JSONResponse(
            status_code=503,
            content={"detail": "Service Unavailable: internal authentication not configured"},
        )

    provided = request.headers.get(INTERNAL_TOKEN_HEADER, "")
    if not hmac.compare_digest(provided.encode("utf-8"), expected.encode("utf-8")):
        logger.warning("内部認証トークンが不正なリクエストを拒否しました path=%s", request.url.path)
        return JSONResponse(status_code=401, content={"detail": "Unauthorized"})

    return await call_next(request)


@app.middleware("http")
async def _trace_id_middleware(request: Request, call_next: Callable) -> Any:
    trace_id = request.headers.get("X-Trace-ID") or str(uuid.uuid4())
    token = _trace_id_var.set(trace_id)
    start = time.time()
    try:
        response = await call_next(request)
        duration_ms = int((time.time() - start) * 1000)
        level = "INFO" if response.status_code < 400 else ("WARN" if response.status_code < 500 else "ERROR")
        payload = {
            "time": datetime.datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
            "level": level,
            "logger": __name__,
            "message": "http request",
            "trace_id": trace_id,
            "method": request.method,
            "path": request.url.path,
            "status": response.status_code,
            "duration_ms": duration_ms,
        }
        print(json.dumps(payload, ensure_ascii=False), flush=True)
        response.headers["X-Trace-ID"] = trace_id
        return response
    finally:
        _trace_id_var.reset(token)


@app.on_event("startup")
def log_openai_version() -> None:
    version = getattr(openai_module, "__version__", "unknown")
    has_responses = hasattr(openai_module.OpenAI, "responses")
    logger.info("openai version=%s responses_api=%s", version, has_responses)
    logger.info("vector_store backend=%s", describe_chroma_backend())


# ルーター登録
from routers import company, es, health, resume, student_search, vector  # noqa: E402

app.include_router(health.router)
app.include_router(resume.router)
app.include_router(company.router)
app.include_router(vector.router)
app.include_router(es.router)
app.include_router(student_search.router)

# テスト互換: logging セットアップ別名
_setup_logging = setup_logging
