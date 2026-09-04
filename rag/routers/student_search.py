"""企業向け学生セマンティック検索エンドポイント（#1094）。

Backend から呼ばれ、学生プロフィールのベクトル登録・検索・削除を行う。
ベクトル化するのは学生が公開に同意した範囲のテキストのみで、
同意範囲の判定は Backend 側（allow_scout_visibility）の責務。
"""
from __future__ import annotations

import logging

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from services.embed import embed_texts
from student_profile_store import delete_student, query_students, upsert_student

logger = logging.getLogger("main")

router = APIRouter()

MAX_TOP_K = 200


class StudentIndexRequest(BaseModel):
    user_id: int = Field(gt=0)
    text: str = Field(min_length=1)


class StudentQueryRequest(BaseModel):
    query: str = Field(min_length=1)
    top_k: int = Field(default=50, gt=0)


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
        hits = query_students(embedding, top_k)
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
