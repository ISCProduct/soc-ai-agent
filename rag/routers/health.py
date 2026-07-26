"""ヘルスチェックエンドポイント。"""
from __future__ import annotations

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from vector_store import ping_chroma

router = APIRouter()


@router.get("/health")
def health() -> dict:
    ok, detail = ping_chroma()
    return {
        "status": "ok" if ok else "degraded",
        "vector_store": {"ok": ok, "detail": detail},
    }


# /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
# /health は後方互換のため維持
@router.get("/healthz")
def healthz() -> dict:
    ok, detail = ping_chroma()
    payload = {
        "status": "ok" if ok else "degraded",
        "vector_store": {"ok": ok, "detail": detail},
    }
    if not ok:
        return JSONResponse(status_code=503, content=payload)
    return payload
