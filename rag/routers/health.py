"""ヘルスチェックエンドポイント。"""
from __future__ import annotations

import os
import threading
import time
from typing import Any, Dict

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from services.model_tiers import es_review_model
from vector_store import ping_chroma

router = APIRouter()

# OpenAI への到達確認（?deep=1）の結果を覚える秒数と、確認の上限時間（SOCAIAGENT-562）。
# ヘルスチェックは頻繁に呼ばれるので、毎回 OpenAI へ問い合わせない。
_OPENAI_PROBE_TTL_SEC = 60.0
_OPENAI_PROBE_TIMEOUT_SEC = 3.0
_openai_probe: Dict[str, Any] = {"at": 0.0, "result": None}
_openai_probe_lock = threading.Lock()


def _chat_model() -> str:
    """ES添削などチャット系で実際に使うモデル名。

    評価ハーネス(Backend/cmd/aibench)がここから読む。プロンプトが Python 側に
    あるため ES添削は HTTP 経由で評価するしかなく、モデル名をハーネス側で
    推測すると結果に残るモデル名が嘘になる。OpenAI 側のモデル更新で同じ名前でも
    出力が変わるため、どのモデルで測ったかが分からない結果は比較に使えない。
    """
    # ES添削のモデルの決め方（services/model_tiers.es_review_model）と同じものを返す（#1526）
    return es_review_model()


def _probe_openai(api_key: str) -> Dict[str, Any]:
    """OpenAI へ届くかを確かめる（モデル一覧の取得。課金されない）。結果は一定時間覚える。"""
    now = time.monotonic()
    with _openai_probe_lock:
        cached = _openai_probe["result"]
        if cached is not None and now - _openai_probe["at"] < _OPENAI_PROBE_TTL_SEC:
            return cached
    import main as m

    try:
        m.OpenAI(api_key=api_key, timeout=_OPENAI_PROBE_TIMEOUT_SEC, max_retries=0).models.list()
        result: Dict[str, Any] = {"reachable": True}
    except Exception as exc:  # 鍵の誤り・ネットワーク・OpenAI の障害のどれでも「届かない」
        # 例外の本文は鍵の断片を含みうるので、種類だけを返す
        result = {"reachable": False, "detail": type(exc).__name__}
    with _openai_probe_lock:
        _openai_probe["at"] = now
        _openai_probe["result"] = result
    return result


def _openai_status(deep: bool) -> Dict[str, Any]:
    """OpenAI の設定（と deep なら到達性）。鍵の値は返さない（SOCAIAGENT-562）。

    以前は鍵が無くても status=ok を返し、全 LLM 経路が 500 になっていても緑のままだった。
    """
    api_key = (os.getenv("OPENAI_API_KEY") or "").strip()
    status: Dict[str, Any] = {"configured": bool(api_key)}
    if deep and api_key:
        status.update(_probe_openai(api_key))
    return status


def _payload(deep: bool) -> Dict[str, Any]:
    ok, detail = ping_chroma()
    openai_status = _openai_status(deep)
    openai_ok = openai_status["configured"] and openai_status.get("reachable", True)
    return {
        "status": "ok" if ok and openai_ok else "degraded",
        "vector_store": {"ok": ok, "detail": detail},
        "openai": openai_status,
        "chat_model": _chat_model(),
    }


@router.get("/health")
def health(deep: bool = False) -> dict:
    """状態を返す。OpenAI の鍵が無ければ degraded。?deep=1 で OpenAI への到達も確かめる。"""
    return _payload(deep)


# /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
# /health は後方互換のため維持
@router.get("/healthz")
def healthz() -> dict:
    """ECS / ALB が見るパス。503 にするのはベクトルストアに届かないときだけ。

    OpenAI の状態は payload に載せるが、503 にはしない。OpenAI の障害でタスクを入れ替えても
    直らず、全タスクが入れ替わり続けるだけになるため。
    """
    payload = _payload(deep=False)
    if not payload["vector_store"]["ok"]:
        return JSONResponse(status_code=503, content=payload)
    return payload
