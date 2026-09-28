"""RAG API の Pydantic リクエスト/レスポンスモデル。"""
from __future__ import annotations

from typing import List, Literal, Optional

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


# ES本文の入力上限(#1564)。
# 改善文は入力の最大130%＋STAR分解(400字)を1回の出力で生成するため、
# (6000 * 1.3 + 400) * 0.85 + 120 = 7090 トークン < _MAX_OUTPUT_TOKENS(8192) に収まる。
# 旧値の10,000字は 7,300字を超えた時点で必ず出力上限に達し 422 になっていた
# （「入力は通るが必ず失敗する」帯）。上限を上げるのではなく入力側を下げて、
# 生成を2回走らせてから失敗する（＝課金してから失敗する）のをやめる。
ES_TEXT_MAX_LENGTH = 6000

# 設問の文字数上限として受け付ける範囲(#1523)。実務のESは400〜800字が中心。
ES_CHAR_LIMIT_MIN = 100
ES_CHAR_LIMIT_MAX = 2000


class ESReviewRequest(BaseModel):
    es_text: str = Field(min_length=1, max_length=ES_TEXT_MAX_LENGTH)
    question_type: str = Field(default="その他")
    company_name: str = Field(default="")
    company_context: str = Field(default="")
    # 任意: 使用技術スタック。ESリライト経路(#1533)から渡る
    tech_stack: str = Field(default="")
    # 任意: 設問の文字数上限(#1523)。未指定なら従来どおり「元の110〜130%」を目安にする
    char_limit: Optional[int] = Field(
        default=None, ge=ES_CHAR_LIMIT_MIN, le=ES_CHAR_LIMIT_MAX
    )
    # within: 超過不可（「400字以内」）/ around: +10%まで許容（「400字程度」）
    char_limit_mode: Literal["within", "around"] = "within"

    @field_validator("company_context")
    @classmethod
    def normalize_es_company_context(cls, v: str) -> str:
        return (v or "").strip()


class ESStarBreakdown(BaseModel):
    """改善後テキストのSTAR分解(#1533)。ESリライト経路が表示に使う。"""

    situation: str = ""
    task: str = ""
    action: str = ""
    result: str = ""


class ESReviewUsage(BaseModel):
    """1リクエストで消費したトークン量(#1533)。

    RAGは api_call_logs を持たないため、Backend が機能別コストへ記録できるよう
    合計値を返す。公開APIへは出さない（Backend が転送前に取り除く）。
    """

    model: str = ""
    prompt_tokens: int = 0
    completion_tokens: int = 0
    calls: int = 0


class ESReviewResponse(BaseModel):
    specificity_score: int  # 1-10: 具体性
    star_score: int  # 1-10: STAR法準拠
    company_fit_score: Optional[int]  # 1-10: 企業適合性（企業名なしは null）
    length_balance_score: int  # 1-10: 文字数バランス（char_limit 指定時は指定字数に対する評価 / #1523）
    feedback: str  # 全体フィードバック文
    improved_text: str  # 改善後テキスト
    company_strategy: Optional[str] = None  # 企業特化の対策アドバイス（企業名なしは null）
    # 企業コンテキストの取得元(#1524)
    company_context_source: Literal["company_brief", "cache", "web_search", "none"] = "none"
    # 改善後テキストのSTAR分解(#1533)
    star: ESStarBreakdown = Field(default_factory=ESStarBreakdown)
    # 改善後テキストの文字数（count_es_chars の数え方。改行と前後の空白は数えない / #1523）
    improved_text_length: int = 0
    # 指定字数に収まったか。char_limit 未指定なら null(#1523)
    char_limit_satisfied: Optional[bool] = None
    usage: Optional[ESReviewUsage] = None


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
