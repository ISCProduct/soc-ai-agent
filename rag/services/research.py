"""Deep Research / Web Search / ドメイン信頼度 / クエリ生成。

patch("main.OpenAI") / patch("main._run_async") 等との互換のため、
実行時に main モジュール上のシンボルを参照する。
"""
from __future__ import annotations

import asyncio
import datetime
import json
import logging
import os
import re
import time
from concurrent.futures import ThreadPoolExecutor
from typing import List

from fastapi import HTTPException

from services.sanitize import _sanitize_company_name_for_query, _sanitize_job_title
from services.settings import DEFAULT_SEARCH_LOG_DIR

logger = logging.getLogger("main")


def extract_output_text(response) -> str:
    output_text = getattr(response, "output_text", None)
    if output_text:
        return output_text.strip()
    choices = getattr(response, "choices", None)
    if choices:
        message = getattr(choices[0], "message", None)
        if message:
            content = getattr(message, "content", "")
            if content:
                return str(content).strip()
    outputs = getattr(response, "output", None)
    if not outputs:
        return ""
    parts = []
    for item in outputs:
        for content in getattr(item, "content", []):
            if getattr(content, "type", "") == "output_text":
                text = getattr(content, "text", "")
                if text:
                    parts.append(text.strip())
    return "\n".join(parts).strip()


def _is_incomplete_response(response) -> bool:
    """max_output_tokens等で打ち切られた応答かどうかを判定する(#992)。

    出力テキストが非空かどうかだけを見ると、途中で切れた不完全なレポートも
    「成功」と誤判定され、そのまま24hキャッシュされて配信され続けてしまう。
    OpenAI Responses APIはstatus="incomplete"でこれを明示するので、それを見る。
    """
    return getattr(response, "status", None) == "incomplete"


def run_deep_research(company_name: str, job_title: str) -> str:
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise HTTPException(status_code=500, detail="OPENAI_API_KEY is required")
    model = os.getenv("OPENAI_DEEP_RESEARCH_MODEL", "o3-deep-research")
    fallback_model = os.getenv("OPENAI_DEEP_RESEARCH_FALLBACK_MODEL", "").strip()
    client = m.OpenAI(api_key=api_key, timeout=m.OPENAI_TIMEOUT_SEC)
    if not hasattr(client, "responses"):
        raise HTTPException(
            status_code=500,
            detail="Deep Research requires OpenAI responses API. Upgrade openai>=1.66 and rebuild the image.",
        )
    safe_company = _sanitize_company_name_for_query(company_name)
    role = _sanitize_job_title(job_title) if job_title else "指定なし"
    logger.info("deep research start model=%s company=%s role=%s", model, safe_company, role)
    prompt = (
        "以下の企業について、採用に関わる価値観・求める人物像・評価軸・事業の特徴を、"
        "一次情報または信頼できる情報に基づいて簡潔に整理してください。"
        "誤りや不確実な点は断定せずに注意書きを入れてください。\n\n"
        "企業名: {company}\n"
        "職種: {role}\n"
        "出力は日本語で、箇条書きを含む短いレポート形式にしてください。"
    ).format(company=safe_company, role=role)
    def request_response(use_tools: bool, model_name: str):
        kwargs = {
            "model": model_name,
            "input": prompt,
            "temperature": 0.2,
            "max_output_tokens": 800,
        }
        if use_tools:
            kwargs["tools"] = [{"type": "web_search"}]
        return client.responses.create(**kwargs)

    # 1回目: メインリクエスト（tools有効・メインモデル）
    try:
        response = request_response(True, model)
        output = extract_output_text(response)
        if output and not _is_incomplete_response(response):
            logger.info("deep research finished chars=%d", len(output))
            return output
        if output:
            logger.warning("deep research response truncated (incomplete), treating as failure chars=%d", len(output))
            last_err = HTTPException(status_code=502, detail="Deep Research response was truncated")
        else:
            logger.warning("deep research returned empty result")
            last_err = HTTPException(status_code=502, detail="Deep Research returned empty result")
    except Exception as exc:
        last_err = exc
        logger.warning("deep research failed error=%s", exc)

    # 2回目（最後の砦）: フォールバックリクエスト（tools無効・フォールバックモデル）
    fallback_name = fallback_model or model
    try:
        response = request_response(False, fallback_name)
        output = extract_output_text(response)
        if output and not _is_incomplete_response(response):
            logger.info("deep research fallback finished chars=%d model=%s", len(output), fallback_name)
            return output
        if output:
            logger.warning(
                "deep research fallback response truncated (incomplete) model=%s chars=%d", fallback_name, len(output)
            )
            last_err = HTTPException(status_code=502, detail="Deep Research fallback response was truncated")
        else:
            logger.warning("deep research fallback returned empty result model=%s", fallback_name)
            last_err = HTTPException(status_code=502, detail="Deep Research fallback returned empty result")
    except Exception as fallback_exc:
        last_err = fallback_exc
        logger.warning("deep research fallback failed model=%s error=%s", fallback_name, fallback_exc)

    raise last_err


def _extract_domains_from_text(text: str) -> List[str]:
    """テキストから URL ドメインを抽出する。見つからなければ空リストを返す。"""
    domains = []
    try:
        for m in re.finditer(r"https?://([^/\s]+)", text):
            host = m.group(1).lower()
            # strip port
            host = host.split(":")[0]
            domains.append(host)
    except Exception:
        pass
    return domains


def _domain_trust_score(domain: str, company_name: str) -> float:
    """ドメインに対して単純な信頼度スコアを返す（0.0-1.0）。"""
    if not domain:
        return 0.5
    d = domain.lower()
    # 日本語文字（ひらがな・カタカナ・漢字）も保持する（sanitize.py の許可文字集合と揃える）。
    # そうしないと日本語企業名では cn が常に空文字列になり、公式ドメインの信頼度ブーストが機能しない。
    cn = re.sub(r"[^0-9a-zぁ-んァ-ン一-龥ー々〆ヵヶ]", "", company_name.lower())
    # 高評価: 企業名がドメインに含まれる（公式サイトの可能性）
    if cn and cn in d:
        return 0.95
    # IR・投資家情報
    if any(x in d for x in ["ir", "investor", "investorrelations", "sec"]):
        return 0.9
    # ニュース・主要メディア
    news_indicators = ["news", "reuters", "nhk", "mainichi", "asahi", "nikkei", "yomiuri", "bloomberg"]
    if any(x in d for x in news_indicators):
        return 0.75
    # SNS は低め
    social = ["twitter.com", "x.com", "facebook.com", "linkedin.com", "instagram.com", "note.com", "reddit.com"]
    if any(s in d for s in social):
        return 0.2
    # デフォルト中立
    return 0.5


def rank_results_by_domain_trust(raw_texts: List[str], company_name: str) -> List[str]:
    """各 raw_text からドメインを抽出し、信頼度スコアで上位に並べ替える。"""
    scored = []
    for text in raw_texts:
        domains = _extract_domains_from_text(text)
        if domains:
            # pick best domain score
            scores = [_domain_trust_score(d, company_name) for d in domains]
            score = max(scores) if scores else 0.5
        else:
            # ドメインが見つからない場合は中立スコア
            score = 0.5
        scored.append((score, text))
    scored.sort(key=lambda x: x[0], reverse=True)
    return [t for _, t in scored]


def _generate_search_queries(company_name: str, job_title: str) -> List[str]:
    """LLMを使って企業・職種に応じた3〜5つの検索クエリを生成する。

    改善点：LLM失敗時は業種/職種テンプレートを用いたフォールバックを行う。
    生成クエリは短TTLのファイルキャッシュに保存して再利用する。
    """
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    safe_company = _sanitize_company_name_for_query(company_name)
    role_text = _sanitize_job_title(job_title) if job_title else "一般職"

    # 簡易ファイルキャッシュ
    cache_ttl = int(os.getenv("RAG_QUERY_CACHE_TTL_SECONDS", "300"))
    cache_path = os.path.join(os.getenv("RAG_SEARCH_LOG_DIR", DEFAULT_SEARCH_LOG_DIR), "query_cache.json")
    cache_key = f"{safe_company}::{role_text}"
    try:
        if os.path.exists(cache_path):
            with open(cache_path, "r", encoding="utf-8") as f:
                cache = json.load(f)
            entry = cache.get(cache_key)
            if entry:
                ts = entry.get("ts", 0)
                if time.time() - ts < cache_ttl:
                    return entry.get("queries", [])
    except Exception as exc:
        logger.warning("query cache read failed error=%s", exc)

    def save_to_cache(key: str, queries: List[str]) -> None:
        try:
            cache = {}
            if os.path.exists(cache_path):
                with open(cache_path, "r", encoding="utf-8") as f:
                    cache = json.load(f)
            cache[key] = {"ts": int(time.time()), "queries": queries}
            os.makedirs(os.path.dirname(cache_path), exist_ok=True)
            with open(cache_path, "w", encoding="utf-8") as f:
                json.dump(cache, f, ensure_ascii=False)
        except Exception as exc:
            logger.warning("query cache save failed error=%s", exc)

    # テンプレートベースのフォールバック
    def template_queries(cn: str, role: str) -> List[str]:
        return [
            f"{cn} {role} 採用方針 求める人物像",
            f"{cn} {role} 面接 選考 特徴 口コミ",
            f"{cn} {role} 採用 ニュース IR 採用情報",
            f"{cn} 採用 求める人物像 企業文化",
            f"{cn} 採用 選考 フロー 面接 質問",
        ]

    # LLM からの生成を試みる
    if api_key:
        try:
            client = m.OpenAI(api_key=api_key, timeout=m.OPENAI_TIMEOUT_SEC)
            prompt = (
                "以下の企業と職種について、採用情報を調査するための検索クエリを3〜5個生成してください。\n\n"
                "企業名: {company}\n"
                "職種: {role}\n\n"
                "以下の3軸をカバーする検索クエリを生成してください。\n"
                "軸1: 採用方針（採用基準・求める人物像・企業文化）\n"
                "軸2: 選考の特徴（面接スタイル・選考フロー・評価ポイント）\n"
                "軸3: 最近の事業展開（直近のニュース・IR情報・新規事業）\n\n"
                "検索エンジンのヒット率を最大化するため、具体的なキーワードを組み合わせてください。\n"
                "出力はJSONのみ: {{\"queries\": [\"クエリ1\", \"クエリ2\", ...]}}"
            ).format(company=safe_company, role=role_text)
            resp = client.chat.completions.create(
                model=os.getenv("OPENAI_CHAT_MODEL", "gpt-4o"),
                messages=[{"role": "user", "content": prompt}],
                temperature=0.3,
                max_tokens=300,
                response_format={"type": "json_object"},
            )
            data = json.loads(resp.choices[0].message.content or "{}")
            queries = data.get("queries", [])
            if queries:
                queries = [q.strip() for q in queries if q and isinstance(q, str)]
                if queries:
                    logger.info("generated %d search queries company=%s", len(queries), safe_company)
                    save_to_cache(cache_key, queries[:5])
                    return queries[:5]
        except Exception as exc:
            logger.warning("query generation failed company=%s error=%s", safe_company, exc)

    # フォールバック: テンプレートを使用
    tqs = template_queries(company_name, role_text)
    save_to_cache(cache_key, tqs[:5])
    return tqs[:5]


def _web_search_openai(query: str) -> str:
    """OpenAI Web Search APIで1クエリを実行し、結果テキストを返す。"""
    import main as m

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return ""
    client = m.OpenAI(api_key=api_key, timeout=m.OPENAI_TIMEOUT_SEC)
    try:
        response = client.chat.completions.create(
            model=m.WEB_SEARCH_MODEL,
            messages=[{"role": "user", "content": query}],
            max_tokens=1000,
        )
        text = response.choices[0].message.content or ""
        logger.info("web search query=%s chars=%d model=%s", query[:60], len(text), m.WEB_SEARCH_MODEL)
        return text.strip()
    except Exception as exc:
        logger.warning("web search failed query=%s model=%s error=%s", query[:60], m.WEB_SEARCH_MODEL, exc)
        return ""


def _summarize_for_hiring(company_name: str, job_title: str, raw_texts: List[str]) -> str:
    """検索結果を採用観点でLLMに要約させる。一次情報（公式/IR/インタビュー）を優先抽出。"""
    import main as m

    if not raw_texts:
        return ""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return "\n\n".join(raw_texts)
    client = m.OpenAI(api_key=api_key, timeout=m.OPENAI_TIMEOUT_SEC)
    safe_company = _sanitize_company_name_for_query(company_name)
    role_text = _sanitize_job_title(job_title) if job_title else "一般職"
    combined = "\n\n---\n\n".join(raw_texts)[:6000]
    prompt = (
                 "企業名: {company}\n"
                 "職種: {role}\n\n"
             ).format(company=safe_company, role=role_text) + (
                 "以下の検索結果をもとに、採用観点での企業分析サマリーを日本語で作成してください。\n\n"
                 "【優先情報ソース（重要度順）】\n"
                 "1. 企業公式サイト（採用ページ・企業理念・代表メッセージ）\n"
                 "2. IR情報（投資家向け資料・決算説明会・中期経営計画）\n"
                 "3. インタビュー記事・社員の声（一次情報）\n"
                 "4. ニュースリリース・プレスリリース\n"
                 "5. 就活メディア・口コミ（参考程度）\n\n"
                 "上位ソースの情報を優先的に引用し、不確かな情報には「※要確認」を付けてください。\n\n"
                 "【まとめる内容】\n"
                 "- 採用方針と求める人物像\n"
                 "- 選考の特徴・評価軸\n"
                 "- 最近の事業展開と成長戦略\n\n"
                 f"【検索結果】\n{combined}"
             )
    try:
        resp = client.chat.completions.create(
            model=os.getenv("OPENAI_CHAT_MODEL", "gpt-4o"),
            messages=[
                {"role": "system", "content": "あなたは企業リサーチの専門アナリストです。"},
                {"role": "user", "content": prompt},
            ],
            temperature=0.2,
            max_tokens=800,
        )
        summary = resp.choices[0].message.content or ""
        logger.info("hiring summary generated company=%s chars=%d", company_name, len(summary))
        return summary.strip()
    except Exception as exc:
        logger.warning("hiring summary failed company=%s error=%s", company_name, exc)
        return "\n\n".join(raw_texts)


def _save_search_log(
        company_name: str,
        job_title: str,
        queries: List[str],
        raw_results: List[str],
        summary: str,
) -> None:
    """検索結果をJSONL形式でログ保存する（ファインチューニング用データセット）。"""
    import main as m

    try:
        os.makedirs(m.SEARCH_LOG_DIR, exist_ok=True)
        log_path = os.path.join(m.SEARCH_LOG_DIR, "search_log.jsonl")
        record = {
            "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
            "company_name": company_name,
            "job_title": job_title,
            "queries": queries,
            "raw_results": raw_results,
            "summary": summary,
        }
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(json.dumps(record, ensure_ascii=False) + "\n")
        logger.info("search log saved company=%s path=%s", company_name, log_path)
    except Exception as exc:
        logger.warning("search log save failed company=%s error=%s", company_name, exc)


async def _run_web_search_pipeline(company_name: str, job_title: str) -> str:
    """クエリ生成 → 並列Web Search → LLM要約 → ログ保存 の非同期パイプライン。

    キャッシュミス時に呼び出し、構造化された採用情報サマリーを返す。
    """
    import main as m

    loop = asyncio.get_event_loop()

    # クエリ生成（スレッドプールで実行）
    queries = await loop.run_in_executor(
        None, m._generate_search_queries, company_name, job_title
    )

    # Web Search を並列実行
    with ThreadPoolExecutor() as executor:
        raw_results = list(await asyncio.gather(
            *[loop.run_in_executor(executor, m._web_search_openai, q) for q in queries]
        ))
    raw_results = [r for r in raw_results if r]

    if not raw_results:
        logger.warning("web search pipeline: no results company=%s", company_name)
        return ""

    # ドメインベースの信頼度で結果を再ランキングしてから要約（フェーズ1: 可視化＋加重）
    try:
        ranked_results = m.rank_results_by_domain_trust(raw_results, company_name)
    except Exception as exc:
        logger.warning("ranking by domain trust failed company=%s error=%s", company_name, exc)
        ranked_results = raw_results

    # 採用観点での要約
    summary = await loop.run_in_executor(
        None, m._summarize_for_hiring, company_name, job_title, ranked_results
    )

    # JSONL ログ保存（非同期、失敗しても処理を止めない）
    await loop.run_in_executor(
        None, m._save_search_log, company_name, job_title, queries, raw_results, summary
    )

    return summary
