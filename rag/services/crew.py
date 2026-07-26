"""CrewAI による履歴書レビュー。"""
from __future__ import annotations

import json
import logging
import re
from typing import List, Optional

from pydantic import BaseModel

from services.sanitize import _sanitize_company_name_for_query, _sanitize_job_title

logger = logging.getLogger("main")


def run_crewai(
        resume_text: str,
        company_name: str,
        job_title: str,
        context_docs: List[str],
        context_source: str = "none",
) -> str:
    """CrewAI を実行し、構造化されたレポートを試みる。
    crewai は依存衝突のため requirements.txt に含まれておらず、インストールされている場合のみ動作する。

    フェーズ1: 出力をまず文字列で受け取り、JSON 形式なら Pydantic で検証。
    構造化に失敗した場合は元の文字列をフォールバックとして返し、詳細ログを残す。
    """
    import main as m

    try:
        from crewai import Agent, Task, Crew, Process
    except ImportError:
        logger.warning("crewai not installed, returning fallback report")
        return "※ CrewAI がインストールされていないため、企業別レビューを生成できませんでした。"

    safe_company = _sanitize_company_name_for_query(company_name)
    safe_job_title = _sanitize_job_title(job_title) if job_title else "指定なし"
    context_block = "\n\n".join(context_docs)

    source_labels = {
        "deep_research": "OpenAI Deep Research（o3-deep-research）",
        "web_search": "OpenAI Web Search（gpt-4o-search-preview）",
        "cache": "ベクトルDBキャッシュ（Chroma / 以前の検索結果）",
        "none": "事前学習データのみ（外部検索なし）",
    }
    source_label = source_labels.get(context_source, context_source)

    researcher = Agent(
        role="Company Researcher",
        goal="Extract company hiring signals and values from search results",
        backstory="You summarize key hiring signals for job applicants.",
        verbose=m.CREWAI_VERBOSE,
    )

    reviewer = Agent(
        role="Resume Reviewer",
        goal="Produce a company-specific resume review report in Japanese",
        backstory="You are a professional career advisor.",
        verbose=m.CREWAI_VERBOSE,
    )

    task_research = Task(
        description=(
            "Use the context to extract the company's core hiring signals. "
            "Return concise bullet keywords only.\n\n"
            "Company: {company}\n"
            "Role: {role}\n"
            "Context:\n{context}\n"
        ).format(company=safe_company, role=safe_job_title, context=context_block),
        expected_output="Bullet keywords",
        agent=researcher,
    )

    task_review = Task(
        description=(
            "Write the final report in Japanese, following this format exactly:\n"
            "【企業別レビュー報告書】\n"
            "---\n"
            "#### ■ 対象企業\n"
            "{company}\n\n"
            "#### ■ この企業が求めている核心的要素\n"
            "- ...\n\n"
            "#### ■ 履歴書の最適化アドバイス\n"
            "- **強みの再定義**: ...\n"
            "- **不足している情報の補足**: ...\n\n"
            "#### ■ 職種別アドバイス（{role}）\n"
            "この職種特有の評価ポイント（技術スキル・マインドセット・実績の見せ方など）を "
            "3点以上、具体的に記述してください。\n\n"
            "#### ■ 修正後の自己PRイメージ\n"
            "...\n\n"
            "#### ■ 情報の信頼度・参照元\n"
            "- 情報ソース: {source}\n"
            "- 注意: 外部情報に基づく内容は変化する可能性があります。最新情報は企業公式サイトで確認してください。\n\n"
            "Use the resume text below and the extracted keywords. "
            "Keep it concise and practical.\n\n"
            "Company: {company}\n"
            "Role: {role}\n"
            "Resume:\n{resume}\n"
        ).format(
            company=safe_company,
            role=safe_job_title,
            resume=resume_text,
            source=source_label,
        ),
        expected_output="Final Japanese report in the requested format",
        agent=reviewer,
        context=[task_research],
    )

    crew = Crew(
        agents=[researcher, reviewer],
        tasks=[task_research, task_review],
        process=Process.sequential,
        verbose=m.CREWAI_VERBOSE,
    )

    # Run crew and attempt to parse structured output
    try:
        raw_out = crew.kickoff()
        out_str = str(raw_out)
    except Exception as exc:
        logger.exception("crew kickoff failed company=%s error=%s", safe_company, exc)
        return f"※CrewAI実行に失敗しました: {str(exc)[:300]}"

    # Try JSON parse -> Pydantic validation
    class CrewReport(BaseModel):
        report: str
        sources: Optional[List[str]] = None

    try:
        parsed = None
        # attempt to find JSON substring
        json_match = re.search(r"\{[\s\S]*\}\s*$", out_str)
        if json_match:
            candidate = json_match.group(0)
            parsed_json = json.loads(candidate)
            parsed = CrewReport(**parsed_json)
            return parsed.report
        # fallback: if output starts with expected heading, return raw
        if out_str.strip().startswith("【企業別レビュー報告書】"):
            return out_str
        # otherwise return raw but log warning
        logger.warning("crew output not structured, returning raw for company=%s len=%d", safe_company, len(out_str))
        return out_str
    except Exception as exc:
        logger.exception("CrewAI output parsing failed company=%s error=%s output=%s", safe_company, exc, out_str[:1000])
        return f"※CrewAI出力の構造化に失敗しました（詳細はログ）。出力冒頭: {out_str[:300]}"
