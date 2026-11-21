"""RAG のプロンプトの版（SOCAIAGENT-560）。

以前は ES 添削（es_review.py の _PROMPT_VERSION）だけが版を持ち、research（Deep Research・
検索クエリの生成・Web Search・採用観点の要約）・面接ヒント・履歴書レビューのプロンプトには版が
無かった。プロンプトを変えた前後の比較や切り戻しがログから追えなかった（#447 の RAG 側）。

工程ごとの版をここに集め、LLM を呼ぶたびに「工程・版・モデル」をログに出す（log_prompt_call）。
**プロンプトの文面を変えたら、その工程の版を上げること。**
"""
from __future__ import annotations

import logging
from typing import Dict

logger = logging.getLogger("rag-review")

# 工程 → 版。版の名前は「工程_v番号」にそろえる（テストで確かめる）
PROMPT_VERSIONS: Dict[str, str] = {
    "es_review": "es_review_v6",
    "deep_research": "deep_research_v1",
    "search_queries": "search_queries_v1",
    "web_search": "web_search_v1",
    "hiring_summary": "hiring_summary_v1",
    "hints_summary": "hints_summary_v1",
    "hints_parse": "hints_parse_v1",
    "resume_review": "resume_review_v1",
}


def prompt_version(stage: str) -> str:
    """工程の版。知らない工程は KeyError（版を付け忘れた工程を呼び出しで気づくため）。"""
    return PROMPT_VERSIONS[stage]


def log_prompt_call(stage: str, model: str) -> str:
    """LLM を呼ぶ前に「工程・版・モデル」をログに出し、版を返す。"""
    version = prompt_version(stage)
    logger.info("llm call stage=%s prompt_version=%s model=%s", stage, version, model)
    return version
