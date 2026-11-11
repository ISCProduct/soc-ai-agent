"""工程ごとの LLM モデルの二層化（#1526）。Backend/internal/aimodel と同じ考え方。

工程を「学生に見える判断（ES添削・履歴書レビュー）」と「内部の定型処理（検索結果の要約・ヒントの parse）」に
分け、層ごとにモデルを差し替えられるようにする。

モデルを決める順番:
  1. 工程ごとの環境変数（既存の名前。例 OPENAI_REVIEW_MODEL）
  2. 層ごとの環境変数（AI_MODEL_PREMIUM / AI_MODEL_LIGHT）
  3. 今までの既定（OPENAI_CHAT_MODEL など）

何も設定しなければ今までと同じモデルになるので、導入だけではコストは変わらない。
"""
from __future__ import annotations

import os
from typing import Optional

ENV_PREMIUM = "AI_MODEL_PREMIUM"
ENV_LIGHT = "AI_MODEL_LIGHT"


def _env(name: Optional[str]) -> str:
    return (os.getenv(name) or "").strip() if name else ""


def resolve(stage_env: Optional[str], tier_env: str, fallback: str) -> str:
    """工程の設定 → 層の設定 → 既定 の順に決める。"""
    return _env(stage_env) or _env(tier_env) or fallback


def es_review_model() -> str:
    """ES添削（上位の層）。OPENAI_ES_REVIEW_MODEL → AI_MODEL_PREMIUM → OPENAI_CHAT_MODEL → gpt-4o。"""
    return resolve("OPENAI_ES_REVIEW_MODEL", ENV_PREMIUM, _env("OPENAI_CHAT_MODEL") or "gpt-4o")


def resume_review_model() -> str:
    """履歴書レビューのレポート（上位の層）。OPENAI_REVIEW_MODEL → AI_MODEL_PREMIUM → gpt-4o-mini。"""
    return resolve("OPENAI_REVIEW_MODEL", ENV_PREMIUM, "gpt-4o-mini")


def summary_model() -> str:
    """検索結果の要約・ヒントの生成（軽量の層）。AI_MODEL_LIGHT → OPENAI_CHAT_MODEL → gpt-4o-mini。"""
    return resolve(None, ENV_LIGHT, _env("OPENAI_CHAT_MODEL") or "gpt-4o-mini")


def hints_parse_model() -> str:
    """ヒントの parse（軽量の層）。OPENAI_HINTS_PARSE_MODEL → AI_MODEL_LIGHT → gpt-4o-mini。"""
    return resolve("OPENAI_HINTS_PARSE_MODEL", ENV_LIGHT, "gpt-4o-mini")
