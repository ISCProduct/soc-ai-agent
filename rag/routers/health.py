"""ヘルスチェックエンドポイント。"""
from __future__ import annotations

import os

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from vector_store import ping_chroma

router = APIRouter()


def _chat_model() -> str:
    """ES添削などチャット系で実際に使うモデル名。

    評価ハーネス(Backend/cmd/aibench)がここから読む。プロンプトが Python 側に
    あるため ES添削は HTTP 経由で評価するしかなく、モデル名をハーネス側で
    推測すると結果に残るモデル名が嘘になる。OpenAI 側のモデル更新で同じ名前でも
    出力が変わるため、どのモデルで測ったかが分からない結果は比較に使えない。
    """
    return os.getenv("OPENAI_CHAT_MODEL", "gpt-4o")


@router.get("/health")
def health() -> dict:
    ok, detail = ping_chroma()
    return {
        "status": "ok" if ok else "degraded",
        "vector_store": {"ok": ok, "detail": detail},
        "chat_model": _chat_model(),
    }


# /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
# /health は後方互換のため維持
@router.get("/healthz")
def healthz() -> dict:
    ok, detail = ping_chroma()
    payload = {
        "status": "ok" if ok else "degraded",
        "vector_store": {"ok": ok, "detail": detail},
        "chat_model": _chat_model(),
    }
    if not ok:
        return JSONResponse(status_code=503, content=payload)
    return payload
