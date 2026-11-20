"""LLM の JSON 出力のスキーマ（Structured Outputs、SOCAIAGENT-563）。

以前は response_format={"type": "json_object"}（JSON モード）だけで、「有効な JSON であること」しか
保証されず、キーの欠落・型違い・想定外の値は受け取り側の防御的なパースで吸収していた。
strict の json_schema を渡すと、モデル側が形を保証する（#446 の RAG 側）。

strict の制約で、すべてのキーを required にし、省略したい項目は null を許す型にする
（例: 企業情報が無いときの company_fit_score）。受け取り側は null と欠落を同じに扱っている。

スキーマに対応しないモデル・OpenAI 互換サーバ（vLLM など）では、呼び出し側が JSON モードへ落とす
（json_response_format / is_schema_unsupported）。受け取り側の検証は今までどおり残す。
"""
from __future__ import annotations

import os
from typing import Any, Dict

_SCORE_KEYS = ["specificity_score", "star_score", "company_fit_score", "length_balance_score"]

ES_REVIEW_SCHEMA: Dict[str, Any] = {
    "type": "object",
    "properties": {
        "specificity_score": {"type": "integer"},
        "star_score": {"type": "integer"},
        "company_fit_score": {"type": ["integer", "null"]},
        "length_balance_score": {"type": "integer"},
        "feedback": {"type": "string"},
        "feedback_items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "score_key": {"type": "string", "enum": _SCORE_KEYS},
                    "source_excerpt": {"type": "string"},
                    "issue": {"type": "string"},
                    "suggestion": {"type": "string"},
                },
                "required": ["score_key", "source_excerpt", "issue", "suggestion"],
                "additionalProperties": False,
            },
        },
        "company_strategy": {"type": ["string", "null"]},
    },
    "required": [
        "specificity_score", "star_score", "company_fit_score", "length_balance_score",
        "feedback", "feedback_items", "company_strategy",
    ],
    "additionalProperties": False,
}

ES_IMPROVED_SCHEMA: Dict[str, Any] = {
    "type": "object",
    "properties": {
        "improved_text": {"type": "string"},
        "star": {
            "type": "object",
            "properties": {k: {"type": "string"} for k in ("situation", "task", "action", "result")},
            "required": ["situation", "task", "action", "result"],
            "additionalProperties": False,
        },
    },
    "required": ["improved_text", "star"],
    "additionalProperties": False,
}

HINTS_PARSE_SCHEMA: Dict[str, Any] = {
    "type": "object",
    "properties": {
        "style_tags": {"type": "array", "items": {"type": "string"}},
        "top_questions": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["style_tags", "top_questions"],
    "additionalProperties": False,
}


def structured_outputs_enabled() -> bool:
    """RAG_STRUCTURED_OUTPUTS=0 なら JSON モードに戻す（切り戻し用）。既定は有効。"""
    return os.getenv("RAG_STRUCTURED_OUTPUTS", "1").strip().lower() not in ("0", "false", "off")


def json_response_format(name: str, schema: Dict[str, Any]) -> Dict[str, Any]:
    """response_format の値。無効にしていれば JSON モード。"""
    if not structured_outputs_enabled():
        return {"type": "json_object"}
    return {"type": "json_schema", "json_schema": {"name": name, "schema": schema, "strict": True}}


def is_schema_unsupported(exc: Exception) -> bool:
    """スキーマ（json_schema）に対応しないモデル・サーバのエラーか。JSON モードで呼び直す目安。"""
    text = str(exc).lower()
    return "json_schema" in text or ("response_format" in text and "support" in text)
