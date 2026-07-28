"""面接ヒントの Web Search と構造化パース。"""
from __future__ import annotations

import asyncio
import logging
import os
from concurrent.futures import ThreadPoolExecutor
from typing import List, Optional

from models import CompanyHintsResponse
from services.sanitize import _sanitize_company_name_for_query, _sanitize_job_title

logger = logging.getLogger("main")


def _run_hints_web_search(company_name: str, position: str) -> Optional[str]:
    """OpenAI Web Search APIで面接傾向を多角的に調査し、採用観点でLLM要約して返す。

    3軸のクエリ（面接スタイル・頻出質問・採用基準）を並列実行して情報ヒット率を高める。
    """
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return None
    safe_company = _sanitize_company_name_for_query(company_name)
    role_text = _sanitize_job_title(position) if position else "一般職"
    # 面接調査に特化した3軸クエリ
    queries = [
        f"{safe_company} {role_text} 面接 選考スタイル 特徴 体験談",
        f"{safe_company} {role_text} 面接 よく聞かれる質問 口コミ",
        f"{safe_company} 採用 求める人物像 評価軸 公式",
    ]
    try:
        summary = m._run_async(m._run_hints_web_search_pipeline, safe_company, role_text, queries)
        return summary if summary else None
    except Exception as exc:
        logger.warning("hints web search failed company=%s error=%s", safe_company, exc)
        return None


async def _run_hints_web_search_pipeline(
        company_name: str, role_text: str, queries: List[str]
) -> str:
    """面接調査クエリを並列Web Searchし、面接観点でLLM要約して返す。"""
    import main as m

    loop = asyncio.get_event_loop()

    with ThreadPoolExecutor() as executor:
        raw_results = list(await asyncio.gather(
            *[loop.run_in_executor(executor, m._web_search_openai, q) for q in queries]
        ))
    raw_results = [r for r in raw_results if r]

    if not raw_results:
        return ""

    # 面接観点での要約（一次情報優先）
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return "\n\n".join(raw_results)
    client = m.OpenAI(api_key=api_key)
    combined = "\n\n---\n\n".join(raw_results)[:5000]
    prompt = (
                 "企業名: {company}\n"
                 "職種: {role}\n\n"
                 "以下の検索結果から、面接・選考に関する情報を採用観点で整理してください。\n\n"
             ).format(company=company_name, role=role_text) + (
                 "【優先情報ソース（重要度順）】\n"
                 "1. 企業公式採用サイト・説明会レポート\n"
                 "2. 実際の選考体験談（一次情報）\n"
                 "3. IR情報・インタビュー記事\n"
                 "4. 就活メディア・口コミサイト（参考程度）\n\n"
                 "不確かな情報には「※要確認」を付けてください。\n\n"
                 "以下の2点を簡潔にまとめてください:\n"
                 "1. 面接スタイルの特徴（ケース面接の有無・深掘り傾向・グループディスカッションの有無等）\n"
                 "2. よく聞かれる質問トップ5\n\n"
                 f"【検索結果】\n{combined}"
             )
    try:
        resp = client.chat.completions.create(
            model=os.getenv("OPENAI_CHAT_MODEL", "gpt-4o"),
            messages=[
                {"role": "system", "content": "あなたは就活生向けの面接アドバイザーです。"},
                {"role": "user", "content": prompt},
            ],
            temperature=0.2,
            max_tokens=800,
        )
        summary = resp.choices[0].message.content or ""
        logger.info("hints web search summary company=%s chars=%d", company_name, len(summary))
        return summary.strip()
    except Exception as exc:
        logger.warning("hints web search summary failed company=%s error=%s", company_name, exc)
        return "\n\n".join(raw_results)


def _parse_hints_from_text(company_name: str, position: str, research_text: str) -> CompanyHintsResponse:
    """調査テキストから構造化ヒントを抽出する。"""
    import json as _json
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return CompanyHintsResponse(style_tags=[], top_questions=[])
    client = m.OpenAI(api_key=api_key)
    # 構造化JSON抽出には通常の chat モデルを使用（search-preview は json_object 非対応）
    model = os.getenv("OPENAI_HINTS_PARSE_MODEL", "gpt-4o")
    role_text = position or "一般職"
    system_prompt = (
        "あなたは就活生向けの面接アドバイザーです。"
        "提供されたリサーチ結果をもとに、以下の2項目をJSON形式で返してください。\n"
        "1. style_tags: 面接スタイルの特徴を示す短いタグ（例: ケース面接あり, 志望動機深掘り, グループディスカッション, 逆質問重視）を最大5件\n"
        "2. top_questions: よく聞かれる質問トップ5（日本語の質問文として）\n"
        "JSONのみを返し、説明文は不要です。フォーマット: {\"style_tags\": [...], \"top_questions\": [...]}"
    )
    user_prompt = (
        f"企業名: {company_name}\n"
        f"職種: {role_text}\n\n"
        f"リサーチ結果:\n{research_text[:3000]}"
    )
    try:
        resp = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.2,
            max_tokens=m.HINTS_PARSE_MAX_TOKENS,
            response_format={"type": "json_object"},
        )
        raw = resp.choices[0].message.content or "{}"
        data = _json.loads(raw)
        return CompanyHintsResponse(
            style_tags=data.get("style_tags", [])[:5],
            top_questions=data.get("top_questions", [])[:5],
        )
    except Exception as exc:
        logger.warning("hints parse failed error=%s", exc)
        return CompanyHintsResponse(style_tags=[], top_questions=[])
