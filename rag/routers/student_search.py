"""企業向け学生セマンティック検索エンドポイント（#1094）。

Backend から呼ばれ、学生プロフィールのベクトル登録・検索・削除を行う。
ベクトル化するのは学生が公開に同意した範囲のテキストのみで、
同意範囲の判定は Backend 側（allow_scout_visibility）の責務。
"""
from __future__ import annotations

import logging
import os

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from services.embed import embed_texts
from student_profile_store import delete_student, query_students, upsert_student

logger = logging.getLogger("main")

router = APIRouter()

MAX_TOP_K = 200

# 関連度（score = 1/(1+距離)）の下限の既定値（SOCAIAGENT-568）。これより低い学生は返さない。
#
# コレクションは距離関数を指定しておらず Chroma の既定（L2 の2乗）。埋め込み（OpenAI）は
# 長さ1に正規化されているので、距離 = 2 - 2cos、score = 1 / (3 - 2cos) になる。
# 0.38 は cos ≒ 0.2（無関係な文どうしで出る程度の類似度）にあたる。
# 距離関数を変える（cosine にする、SOCAIAGENT-554）ときは、この換算もやり直すこと。
# 環境変数 STUDENT_SEARCH_MIN_SCORE で変えられ、リクエストの min_score が優先する。
DEFAULT_MIN_SCORE = 0.38


def default_min_score() -> float:
    raw = os.getenv("STUDENT_SEARCH_MIN_SCORE", "").strip()
    if not raw:
        return DEFAULT_MIN_SCORE
    try:
        value = float(raw)
    except ValueError:
        logger.warning("invalid STUDENT_SEARCH_MIN_SCORE=%r; using default %.2f", raw, DEFAULT_MIN_SCORE)
        return DEFAULT_MIN_SCORE
    if not 0.0 <= value <= 1.0:
        logger.warning("STUDENT_SEARCH_MIN_SCORE=%r is out of [0, 1]; using default %.2f", raw, DEFAULT_MIN_SCORE)
        return DEFAULT_MIN_SCORE
    return value


class StudentIndexRequest(BaseModel):
    user_id: int = Field(gt=0)
    text: str = Field(min_length=1)


class StudentQueryRequest(BaseModel):
    query: str = Field(min_length=1)
    top_k: int = Field(default=50, gt=0)
    # 関連度の下限。省略時は default_min_score()（0 を渡すと絞らない）
    min_score: float | None = Field(default=None, ge=0.0, le=1.0)


class StudentHit(BaseModel):
    user_id: int
    score: float


class StudentQueryResponse(BaseModel):
    hits: list[StudentHit]


@router.post("/student-search/index")
def student_index(request: StudentIndexRequest) -> dict:
    """学生プロフィールをベクトル化して登録・更新する。"""
    try:
        embedding = embed_texts([request.text])[0]
        upsert_student(request.user_id, request.text, embedding)
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("student index failed user_id=%s error=%s", request.user_id, exc)
        raise HTTPException(status_code=503, detail=f"student index failed: {exc}") from exc
    return {"status": "ok", "user_id": request.user_id}


@router.post("/student-search/query", response_model=StudentQueryResponse)
def student_query(request: StudentQueryRequest) -> StudentQueryResponse:
    """自然文クエリに意味的に近い学生を関連度順で返す。"""
    top_k = min(request.top_k, MAX_TOP_K)
    try:
        embedding = embed_texts([request.query])[0]
        min_score = request.min_score if request.min_score is not None else default_min_score()
        hits = query_students(embedding, top_k, min_score)
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("student query failed error=%s", exc)
        raise HTTPException(status_code=503, detail=f"student query failed: {exc}") from exc
    return StudentQueryResponse(hits=[StudentHit(**h) for h in hits])


@router.delete("/student-search/{user_id}")
def student_delete(user_id: int) -> dict:
    """学生のベクトルを削除する（公開同意の撤回・退会時）。"""
    if user_id <= 0:
        raise HTTPException(status_code=400, detail="invalid user_id")
    try:
        delete_student(user_id)
    except Exception as exc:
        logger.exception("student delete failed user_id=%s error=%s", user_id, exc)
        raise HTTPException(status_code=503, detail=f"student delete failed: {exc}") from exc
    return {"status": "ok", "user_id": user_id}
