"""RAG API の Pydantic リクエスト/レスポンスモデル。"""
from __future__ import annotations

import logging
from typing import List, Literal, Optional

from pydantic import BaseModel, Field, field_validator

logger = logging.getLogger(__name__)

# 企業コンテキスト（Backend 共有 brief）の受け入れ上限。
# プロンプト側でさらに短く切られる参考情報なので、422 でリクエストを落とさず
# 切り詰める（DoS・入力トークンのコスト上限 / #1591）。
COMPANY_CONTEXT_MAX_LENGTH = 20000
# 質問種別は FE 側は固定の選択肢だが、自由記述で送れてしまうので上限を付ける(#1591)。
QUESTION_TYPE_MAX_LENGTH = 100
# 職種名。FE の選択肢は最長でも数十字で、これを超える値は入力ミスか攻撃(#1591)
POSITION_MAX_LENGTH = 100


def _normalize_company_context(value: str, field_name: str) -> str:
    normalized = (value or "").strip()
    if len(normalized) > COMPANY_CONTEXT_MAX_LENGTH:
        logger.info(
            "company context truncated field=%s original_chars=%d max_chars=%d",
            field_name,
            len(normalized),
            COMPANY_CONTEXT_MAX_LENGTH,
        )
        return normalized[:COMPANY_CONTEXT_MAX_LENGTH]
    return normalized


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
        return _normalize_company_context(v, "review")


class ReviewResponse(BaseModel):
    report: str


class CompanyHintsRequest(BaseModel):
    company_name: str = Field(min_length=1)
    # 職種はプロンプトとキャッシュキーの両方に入るので上限を置く(#1591)
    position: str = Field(default="", max_length=POSITION_MAX_LENGTH)
    company_context: str = Field(default="")

    @field_validator("company_context")
    @classmethod
    def normalize_hints_company_context(cls, v: str) -> str:
        return _normalize_company_context(v, "hints")


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
    question_type: str = Field(default="その他", max_length=QUESTION_TYPE_MAX_LENGTH)
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
        return _normalize_company_context(v, "es_review")


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
    # Backend が企業横断キャッシュへ保存する本文も他の企業コンテキストと同じ上限で拒否する。
    content: str = Field(min_length=1, max_length=COMPANY_CONTEXT_MAX_LENGTH)


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
