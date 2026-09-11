"""RAG API の Pydantic リクエスト/レスポンスモデル。"""
from __future__ import annotations

from typing import List, Optional

from pydantic import BaseModel, Field, field_validator


class ReviewRequest(BaseModel):
    resume_text: str = Field(min_length=1, max_length=10000)
    company_name: str = Field(min_length=1)
    job_title: str = Field(default="")
    # Backend 共有キャッシュの brief（Search なし）。あればキャッシュ/Searchより優先
    company_context: str = Field(default="")

    @field_validator("job_title")
    @classmethod
    def normalize_job_title(cls, v: str) -> str:
        return v.strip()

    @field_validator("company_context")
    @classmethod
    def normalize_company_context(cls, v: str) -> str:
        return (v or "").strip()


class ReviewResponse(BaseModel):
    report: str


class CompanyHintsRequest(BaseModel):
    company_name: str = Field(min_length=1)
    position: str = Field(default="")
    company_context: str = Field(default="")

    @field_validator("company_context")
    @classmethod
    def normalize_hints_company_context(cls, v: str) -> str:
        return (v or "").strip()


class CompanyHintsResponse(BaseModel):
    style_tags: List[str]
    top_questions: List[str]
    cached: bool = False


class ESReviewRequest(BaseModel):
    es_text: str = Field(min_length=1, max_length=10000)
    question_type: str = Field(default="その他")
    company_name: str = Field(default="")
    company_context: str = Field(default="")

    @field_validator("company_context")
    @classmethod
    def normalize_es_company_context(cls, v: str) -> str:
        return (v or "").strip()


class ESReviewResponse(BaseModel):
    specificity_score: int  # 1-10: 具体性
    star_score: int  # 1-10: STAR法準拠
    company_fit_score: Optional[int]  # 1-10: 企業適合性（企業名なしは null）
    length_balance_score: int  # 1-10: 文字数バランス
    feedback: str  # 全体フィードバック文
    improved_text: str  # 改善後テキスト
    company_strategy: Optional[str] = None  # 企業特化の対策アドバイス（企業名なしは null）


class CompanyContextRequest(BaseModel):
    company_name: str = Field(min_length=1)
    context_type: str = Field(default="general")  # "jobs", "persona", "general"
    content: str = Field(min_length=1)


class CompanyContextResponse(BaseModel):
    status: str
    company: str
    context_type: str
    keys_updated: int


class VectorStatusResponse(BaseModel):
    backend: str
    host: Optional[str] = None
    port: Optional[int] = None
    company: Optional[str] = None
    collections: List[dict]
    total_documents: int


class VectorReembedRequest(BaseModel):
    company_name: str = Field(min_length=1)
    doc_type: Optional[str] = None  # resume_review / company_research / interview_hints / es_review
    refresh: bool = True  # True なら削除後に WebSearch で再取得


class VectorReembedResponse(BaseModel):
    status: str
    company: str
    deleted: int
    collections: dict
    refreshed: bool
    sources: List[str] = []
