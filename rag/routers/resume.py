"""履歴書レビューエンドポイント。"""
from __future__ import annotations

import json
import logging
import os
from typing import Generator

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse
from openai import OpenAI

from models import ReviewRequest, ReviewResponse
from services.model_tiers import resume_review_model
from services.resume_report import (
    RESUME_SYSTEM_PROMPT,
    build_resume_review_prompt,
    generate_resume_report,
)
from services.settings import OPENAI_TIMEOUT_SEC

logger = logging.getLogger("main")

router = APIRouter()


@router.post("/resume/review/stream")
def review_resume_stream(request: ReviewRequest) -> StreamingResponse:
    """RAGレポートをSSEでストリーミング配信するエンドポイント。"""
    import main as m

    retrieved, context_source = m._gather_context(request)
    prompt, _, _ = build_resume_review_prompt(
        request, retrieved, context_source, m.RESUME_REVIEW_INPUT_CHAR_LIMIT
    )

    def generate() -> Generator[str, None, None]:
        api_key = os.getenv("OPENAI_API_KEY")
        if not api_key:
            yield "data: {}\n\n".format(
                json.dumps({"type": "error", "message": "OPENAI_API_KEY not set"}, ensure_ascii=False)
            )
            return

        try:
            client = OpenAI(api_key=api_key, timeout=OPENAI_TIMEOUT_SEC)
            stream = client.chat.completions.create(
                model=resume_review_model(),
                messages=[
                    {"role": "system", "content": RESUME_SYSTEM_PROMPT},
                    {"role": "user", "content": prompt},
                ],
                stream=True,
                max_tokens=1500,
                temperature=0.3,
            )
            for chunk in stream:
                delta = chunk.choices[0].delta
                if delta.content:
                    yield "data: {}\n\n".format(
                        json.dumps({"type": "chunk", "text": delta.content}, ensure_ascii=False)
                    )
            yield "data: {}\n\n".format(json.dumps({"type": "done"}, ensure_ascii=False))
        except Exception as exc:
            logger.error("review_stream generate error: %s", exc)
            yield "data: {}\n\n".format(
                json.dumps({"type": "error", "message": str(exc)}, ensure_ascii=False)
            )

    return StreamingResponse(generate(), media_type="text/event-stream")


@router.post("/resume/review", response_model=ReviewResponse)
def review_resume(request: ReviewRequest) -> ReviewResponse:
    """非stream。stream と同じ OpenAI プロンプト（candidate_context 含む）で生成する。

    以前は CrewAI 専用で、未インストール時に固定文言だけ返していたため
    フライホイールの「スコア→RAG注入」が非stream経路で死んでいた。
    """
    import main as m

    retrieved, context_source = m._gather_context(request)
    try:
        report = generate_resume_report(
            request, retrieved, context_source, m.RESUME_REVIEW_INPUT_CHAR_LIMIT
        )
    except Exception as exc:
        logger.error("review_resume generate error: %s", exc)
        raise HTTPException(status_code=502, detail="resume review generation failed") from exc
    return ReviewResponse(report=report)
