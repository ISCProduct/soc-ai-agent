import asyncio
import chromadb
import contextvars
import datetime
import json
import logging
import math
import openai as openai_module
import os
import re
import threading
import tiktoken
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from crewai import Agent, Task, Crew, Process
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import StreamingResponse
from openai import OpenAI
from pydantic import BaseModel, Field, field_validator
from typing import Any, Awaitable, Callable, Generator, List, Optional, Tuple, TypeVar

# ── 構造化ログ設定 ────────────────────────────────────────────────────────────
_trace_id_var: contextvars.ContextVar[str] = contextvars.ContextVar("trace_id", default="")

_LOG_LEVEL_MAP = {
    "DEBUG": logging.DEBUG,
    "INFO": logging.INFO,
    "WARN": logging.WARNING,
    "WARNING": logging.WARNING,
    "ERROR": logging.ERROR,
}
_log_level = _LOG_LEVEL_MAP.get(os.getenv("LOG_LEVEL", "INFO").upper(), logging.INFO)


class _JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "time": self.formatTime(record, "%Y-%m-%dT%H:%M:%S"),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }
        trace_id = _trace_id_var.get("")
        if trace_id:
            payload["trace_id"] = trace_id
        if record.exc_info:
            payload["exc_info"] = self.formatException(record.exc_info)
        return json.dumps(payload, ensure_ascii=False)


def _setup_logging() -> logging.Logger:
    handler = logging.StreamHandler()
    handler.setFormatter(_JsonFormatter())
    root = logging.getLogger()
    root.setLevel(_log_level)
    root.handlers = [handler]
    return logging.getLogger(__name__)


logger = _setup_logging()

app = FastAPI()

# Training export endpoints (registered from training_api.py)
import training_api
training_api.register(app)

@app.middleware("http")
async def _trace_id_middleware(request: Request, call_next: Callable) -> Any:
    trace_id = request.headers.get("X-Trace-ID") or str(uuid.uuid4())
    token = _trace_id_var.set(trace_id)
    start = time.time()
    try:
        response = await call_next(request)
        duration_ms = int((time.time() - start) * 1000)
        level = "INFO" if response.status_code < 400 else ("WARN" if response.status_code < 500 else "ERROR")
        payload = {
            "time": datetime.datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
            "level": level,
            "logger": __name__,
            "message": "http request",
            "trace_id": trace_id,
            "method": request.method,
            "path": request.url.path,
            "status": response.status_code,
            "duration_ms": duration_ms,
        }
        print(json.dumps(payload, ensure_ascii=False), flush=True)
        response.headers["X-Trace-ID"] = trace_id
        return response
    finally:
        _trace_id_var.reset(token)


# ── 環境変数 ────────────────────────────────────────────────────────────────
DEFAULT_CACHE_TTL_SECONDS = 86400
DEFAULT_MAX_EMBED_TOKENS = 8191
DEFAULT_EMBED_MAX_RETRIES = 3
DEFAULT_CHROMA_DATA_DIR = "/app/chroma_db"
DEFAULT_HINTS_PARSE_MAX_TOKENS = 600
DEFAULT_RESUME_REVIEW_INPUT_CHAR_LIMIT = 10000

DEFAULT_WEB_SEARCH_MODEL = "gpt-4o-search-preview"
DEFAULT_SEARCH_LOG_DIR = "/app/search_logs"

CACHE_TTL_SECONDS = int(os.getenv("RAG_SEARCH_CACHE_TTL_SECONDS", str(DEFAULT_CACHE_TTL_SECONDS)))
USE_DEEP_RESEARCH = os.getenv("RAG_USE_DEEP_RESEARCH", "true").lower() == "true"
ALLOW_WEB_SEARCH_FALLBACK = os.getenv(
    "RAG_ALLOW_WEB_SEARCH_FALLBACK",
    os.getenv("RAG_ALLOW_DUCKDUCKGO_FALLBACK", "true"),
).lower() == "true"
STRICT_DEEP_RESEARCH = os.getenv("RAG_DEEP_RESEARCH_STRICT", "false").lower() == "true"
CREWAI_VERBOSE = os.getenv("RAG_CREWAI_VERBOSE", "false").lower() == "true"
MAX_EMBED_TOKENS = int(os.getenv("RAG_MAX_EMBED_TOKENS", str(DEFAULT_MAX_EMBED_TOKENS)))
EMBED_MAX_RETRIES = int(os.getenv("RAG_EMBED_MAX_RETRIES", str(DEFAULT_EMBED_MAX_RETRIES)))
CHROMA_DATA_DIR = os.getenv("RAG_CHROMA_DATA_DIR", DEFAULT_CHROMA_DATA_DIR)
HINTS_PARSE_MAX_TOKENS = int(os.getenv("RAG_HINTS_PARSE_MAX_TOKENS", str(DEFAULT_HINTS_PARSE_MAX_TOKENS)))
RESUME_REVIEW_INPUT_CHAR_LIMIT = int(
    os.getenv("RAG_REVIEW_RESUME_CHAR_LIMIT", str(DEFAULT_RESUME_REVIEW_INPUT_CHAR_LIMIT)))
WEB_SEARCH_MODEL = os.getenv("OPENAI_WEB_SEARCH_MODEL", DEFAULT_WEB_SEARCH_MODEL)
SEARCH_LOG_DIR = os.getenv("RAG_SEARCH_LOG_DIR", DEFAULT_SEARCH_LOG_DIR)
T = TypeVar("T")


def _run_async(async_func: Callable[..., Awaitable[T]], *args: Any) -> T:
    """同期コンテキストから非同期関数を実行する。"""
    return asyncio.run(async_func(*args))


# ── Chromadb 永続ベクトルストア ────────────────────────────────────────────
_chroma_client: Optional[chromadb.PersistentClient] = None
_chroma_lock = threading.Lock()


@app.on_event("startup")
def log_openai_version() -> None:
    version = getattr(openai_module, "__version__", "unknown")
    has_responses = hasattr(openai_module.OpenAI, "responses")
    logger.info("openai version=%s responses_api=%s", version, has_responses)


class ReviewRequest(BaseModel):
    resume_text: str = Field(min_length=1, max_length=10000)
    company_name: str = Field(min_length=1)
    job_title: str = Field(default="")

    @field_validator("job_title")
    @classmethod
    def normalize_job_title(cls, v: str) -> str:
        return v.strip()


class ReviewResponse(BaseModel):
    report: str


def get_chroma_client() -> chromadb.PersistentClient:
    global _chroma_client
    if _chroma_client is None:
        with _chroma_lock:
            if _chroma_client is None:
                _chroma_client = chromadb.PersistentClient(path=CHROMA_DATA_DIR)
    return _chroma_client


def _sanitize_collection_name(cache_key: str) -> str:
    """chromadb のコレクション名制約に合わせてサニタイズする (3-63文字, 英数字/_/-)。"""
    name = re.sub(r"[^a-zA-Z0-9_-]", "_", cache_key)
    name = re.sub(r"^[^a-zA-Z0-9]+", "", name)
    name = re.sub(r"[^a-zA-Z0-9]+$", "", name)
    if len(name) < 3:
        name = name.ljust(3, "x")
    return name[:63]


def get_cached_context(
        cache_key: str, query: str = "採用 価値観 求める人物像"
) -> List[str]:
    """chromadb からキャッシュ済みドキュメントをベクトル類似度順で最大 5 件取得する。"""
    try:
        client = get_chroma_client()
        col_name = _sanitize_collection_name(cache_key)
        collection_names = {
            getattr(col, "name", "")
            for col in client.list_collections()
        }
        if col_name not in collection_names:
            logger.info("chromadb cache miss key=%s reason=collection_not_found", cache_key)
            return []
        collection = client.get_collection(col_name)
        count = collection.count()
        if count == 0:
            return []
        query_emb = embed_texts([query])[0]
        results = collection.query(
            query_embeddings=[query_emb],
            n_results=min(5, count),
        )
        docs: List[str] = results.get("documents", [[]])[0]
        logger.info("chromadb cache hit key=%s docs=%d", cache_key, len(docs))
        return docs
    except Exception as exc:
        logger.exception("chromadb get failed key=%s error=%s", cache_key, exc)
        return []


def set_cached_context(cache_key: str, docs: List[str]) -> None:
    """ドキュメントと埋め込みを chromadb に永続保存する。"""
    if not docs:
        return
    try:
        client = get_chroma_client()
        col_name = _sanitize_collection_name(cache_key)
        collection = client.get_or_create_collection(col_name)
        embeddings = embed_texts(docs)
        ids = [f"doc_{i}" for i in range(len(docs))]
        collection.upsert(ids=ids, documents=docs, embeddings=embeddings)
        logger.info("chromadb upsert key=%s docs=%d", cache_key, len(docs))
    except Exception as exc:
        logger.exception("chromadb set failed key=%s error=%s", cache_key, exc)


def _sanitize_company_name_for_query(company_name: str) -> str:
    sanitized = re.sub(r"[^0-9A-Za-zぁ-んァ-ン一-龥ー々〆ヵヶ・\s]", "", company_name)
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    if not sanitized:
        raise HTTPException(status_code=400, detail="invalid company_name")
    return sanitized


def _sanitize_job_title(job_title: str) -> str:
    """職種名からプロンプトインジェクションに使われうる特殊文字を除去する"""
    sanitized = re.sub(r"[^\w\s\-（）()／/]", "", job_title, flags=re.UNICODE)
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    return sanitized or "指定なし"


def _truncate_text(text: str, model: str) -> str:
    """テキストが埋め込みモデルのトークン上限を超えている場合に切り詰める。"""
    try:
        enc = tiktoken.encoding_for_model(model)
    except KeyError:
        enc = tiktoken.get_encoding("cl100k_base")
    tokens = enc.encode(text)
    if len(tokens) > MAX_EMBED_TOKENS:
        logger.warning(
            "truncating text from %d to %d tokens for model=%s",
            len(tokens),
            MAX_EMBED_TOKENS,
            model,
        )
        return enc.decode(tokens[:MAX_EMBED_TOKENS])
    return text


def embed_texts(texts: List[str]) -> List[List[float]]:
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise HTTPException(status_code=500, detail="OPENAI_API_KEY is required")

    embedding_model = os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    client = OpenAI(api_key=api_key)

    # トークン上限チェック
    texts = [_truncate_text(t, embedding_model) for t in texts]

    last_err: Exception = RuntimeError("embed_texts: no attempts made")
    for attempt in range(1, EMBED_MAX_RETRIES + 1):
        try:
            response = client.embeddings.create(model=embedding_model, input=texts)
            return [item.embedding for item in response.data]
        except Exception as exc:
            last_err = exc
            if attempt < EMBED_MAX_RETRIES:
                wait = 2 ** (attempt - 1)
                logger.warning(
                    "embed_texts failed attempt=%d retrying in %ds error=%s",
                    attempt,
                    wait,
                    exc,
                )
                time.sleep(wait)
    raise last_err


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


def run_deep_research(company_name: str, job_title: str) -> str:
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise HTTPException(status_code=500, detail="OPENAI_API_KEY is required")
    model = os.getenv("OPENAI_DEEP_RESEARCH_MODEL", "o3-deep-research")
    fallback_model = os.getenv("OPENAI_DEEP_RESEARCH_FALLBACK_MODEL", "").strip()
    client = OpenAI(api_key=api_key)
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
    last_err = None

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

    for attempt in range(1, 3):
        try:
            response = request_response(True, model)
            output = extract_output_text(response)
            logger.info("deep research finished chars=%d attempt=%d", len(output), attempt)
            if output:
                return output
            logger.warning("deep research returned empty result attempt=%d", attempt)
        except Exception as exc:
            last_err = exc
            logger.warning("deep research failed attempt=%d error=%s", attempt, exc)
            if attempt == 1:
                fallback_name = fallback_model or model
                try:
                    response = request_response(False, fallback_name)
                    output = extract_output_text(response)
                    logger.info(
                        "deep research fallback finished chars=%d model=%s",
                        len(output),
                        fallback_name,
                    )
                    if output:
                        return output
                    logger.warning("deep research fallback returned empty result model=%s", fallback_name)
                except Exception as fallback_exc:
                    last_err = fallback_exc
                    logger.warning(
                        "deep research fallback failed model=%s error=%s",
                        fallback_name,
                        fallback_exc,
                    )
    raise last_err


def cosine_similarity(a: List[float], b: List[float]) -> float:
    dot = 0.0
    norm_a = 0.0
    norm_b = 0.0
    for av, bv in zip(a, b):
        dot += av * bv
        norm_a += av * av
        norm_b += bv * bv
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return dot / (math.sqrt(norm_a) * math.sqrt(norm_b))


def retrieve_docs(docs: List[str], query: str) -> List[str]:
    if not docs:
        return []
    embeddings = embed_texts(docs + [query])
    doc_embeddings = embeddings[:-1]
    query_embedding = embeddings[-1]

    scored = []
    for doc, emb in zip(docs, doc_embeddings):
        scored.append((cosine_similarity(query_embedding, emb), doc))
    scored.sort(key=lambda item: item[0], reverse=True)
    top_docs = [doc for _, doc in scored[: min(5, len(scored))]]
    return top_docs


def run_crewai(
        resume_text: str,
        company_name: str,
        job_title: str,
        context_docs: List[str],
        context_source: str = "none",
) -> str:
    safe_company = _sanitize_company_name_for_query(company_name)
    safe_job_title = _sanitize_job_title(job_title) if job_title else "指定なし"
    context_block = "\n\n".join(context_docs)

    source_labels = {
        "deep_research": "OpenAI Deep Research（o3-deep-research）",
        "web_search": "OpenAI Web Search（gpt-4o-search-preview）",
        "cache": "chromadb キャッシュ（以前の検索結果）",
        "none": "事前学習データのみ（外部検索なし）",
    }
    source_label = source_labels.get(context_source, context_source)

    researcher = Agent(
        role="Company Researcher",
        goal="Extract company hiring signals and values from search results",
        backstory="You summarize key hiring signals for job applicants.",
        verbose=CREWAI_VERBOSE,
    )

    reviewer = Agent(
        role="Resume Reviewer",
        goal="Produce a company-specific resume review report in Japanese",
        backstory="You are a professional career advisor.",
        verbose=CREWAI_VERBOSE,
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
        verbose=CREWAI_VERBOSE,
    )

    return str(crew.kickoff())


def _generate_search_queries(company_name: str, job_title: str) -> List[str]:
    """LLMを使って企業・職種に応じた3〜5つの検索クエリを生成する。

    以下の3軸をカバーするクエリを生成する:
    - 採用方針（採用基準・求める人物像・企業文化）
    - 選考の特徴（面接スタイル・選考フロー・評価ポイント）
    - 最近の事業展開（直近ニュース・IR情報・新規事業）
    """
    api_key = os.getenv("OPENAI_API_KEY")
    safe_company = _sanitize_company_name_for_query(company_name)
    role_text = _sanitize_job_title(job_title) if job_title else "一般職"
    if not api_key:
        return [
            f"{safe_company} {role_text} 採用方針 求める人物像",
            f"{safe_company} {role_text} 面接 選考 特徴",
            f"{safe_company} 最近の事業展開 ニュース IR",
        ]
    client = OpenAI(api_key=api_key)
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
    try:
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
            logger.info("generated %d search queries company=%s", len(queries), safe_company)
            return queries[:5]
    except Exception as exc:
        logger.warning("query generation failed company=%s error=%s", safe_company, exc)
    return [
        f"{company_name} {role_text} 採用方針 求める人物像",
        f"{company_name} {role_text} 面接 選考 特徴",
        f"{company_name} 最近の事業展開 ニュース IR",
    ]


def _web_search_openai(query: str) -> str:
    """OpenAI Web Search APIで1クエリを実行し、結果テキストを返す。"""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return ""
    client = OpenAI(api_key=api_key)
    try:
        response = client.chat.completions.create(
            model=WEB_SEARCH_MODEL,
            messages=[{"role": "user", "content": query}],
            max_tokens=1000,
        )
        text = response.choices[0].message.content or ""
        logger.info("web search query=%s chars=%d model=%s", query[:60], len(text), WEB_SEARCH_MODEL)
        return text.strip()
    except Exception as exc:
        logger.warning("web search failed query=%s model=%s error=%s", query[:60], WEB_SEARCH_MODEL, exc)
        return ""


def _summarize_for_hiring(company_name: str, job_title: str, raw_texts: List[str]) -> str:
    """検索結果を採用観点でLLMに要約させる。一次情報（公式/IR/インタビュー）を優先抽出。"""
    if not raw_texts:
        return ""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return "\n\n".join(raw_texts)
    client = OpenAI(api_key=api_key)
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
    try:
        os.makedirs(SEARCH_LOG_DIR, exist_ok=True)
        log_path = os.path.join(SEARCH_LOG_DIR, "search_log.jsonl")
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
    loop = asyncio.get_event_loop()

    # クエリ生成（スレッドプールで実行）
    queries = await loop.run_in_executor(
        None, _generate_search_queries, company_name, job_title
    )

    # Web Search を並列実行
    with ThreadPoolExecutor() as executor:
        raw_results = list(await asyncio.gather(
            *[loop.run_in_executor(executor, _web_search_openai, q) for q in queries]
        ))
    raw_results = [r for r in raw_results if r]

    if not raw_results:
        logger.warning("web search pipeline: no results company=%s", company_name)
        return ""

    # 採用観点での要約
    summary = await loop.run_in_executor(
        None, _summarize_for_hiring, company_name, job_title, raw_results
    )

    # JSONL ログ保存（非同期、失敗しても処理を止めない）
    await loop.run_in_executor(
        None, _save_search_log, company_name, job_title, queries, raw_results, summary
    )

    return summary


class CompanyHintsRequest(BaseModel):
    company_name: str = Field(min_length=1)
    position: str = Field(default="")


class CompanyHintsResponse(BaseModel):
    style_tags: List[str]
    top_questions: List[str]
    cached: bool = False


def _run_hints_web_search(company_name: str, position: str) -> Optional[str]:
    """OpenAI Web Search APIで面接傾向を多角的に調査し、採用観点でLLM要約して返す。

    3軸のクエリ（面接スタイル・頻出質問・採用基準）を並列実行して情報ヒット率を高める。
    """
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
        summary = _run_async(_run_hints_web_search_pipeline, safe_company, role_text, queries)
        return summary if summary else None
    except Exception as exc:
        logger.warning("hints web search failed company=%s error=%s", safe_company, exc)
        return None


async def _run_hints_web_search_pipeline(
        company_name: str, role_text: str, queries: List[str]
) -> str:
    """面接調査クエリを並列Web Searchし、面接観点でLLM要約して返す。"""
    loop = asyncio.get_event_loop()

    with ThreadPoolExecutor() as executor:
        raw_results = list(await asyncio.gather(
            *[loop.run_in_executor(executor, _web_search_openai, q) for q in queries]
        ))
    raw_results = [r for r in raw_results if r]

    if not raw_results:
        return ""

    # 面接観点での要約（一次情報優先）
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return "\n\n".join(raw_results)
    client = OpenAI(api_key=api_key)
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
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return CompanyHintsResponse(style_tags=[], top_questions=[])
    client = OpenAI(api_key=api_key)
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
            max_tokens=HINTS_PARSE_MAX_TOKENS,
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


@app.post("/company/hints", response_model=CompanyHintsResponse)
def company_hints(request: CompanyHintsRequest) -> CompanyHintsResponse:
    # 入力値サニタイズ（#331: クエリインジェクション対策）
    safe_company_name = _sanitize_company_name_for_query(request.company_name)
    role_label = request.position or "一般職"
    cache_key = "hints::{company}::{role}".format(
        company=safe_company_name, role=role_label
    )

    # キャッシュヒット: そのまま構造化して返す
    retrieved = get_cached_context(
        cache_key, query=f"{safe_company_name} 面接 よく聞かれる質問"
    )
    if retrieved:
        result = _parse_hints_from_text(safe_company_name, role_label, "\n\n".join(retrieved))
        result.cached = True
        return result

    # OpenAI Web Search（多角的クエリ生成 → 並列検索 → LLM要約）
    research_text = _run_hints_web_search(safe_company_name, role_label)

    if research_text:
        set_cached_context(cache_key, [research_text])

    return _parse_hints_from_text(safe_company_name, role_label, research_text or "")


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


# /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
# /health は後方互換のため維持
@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


def _gather_context(request: ReviewRequest) -> Tuple[List[str], str]:
    """RAGコンテキストを収集し (docs, context_source) を返す。

    キャッシュヒット時は即時返却。ミス時は以下のパイプラインを実行:
    クエリ生成 → 並列Web Search → LLM要約（採用観点） → キャッシュ保存
    """
    safe_company_name = _sanitize_company_name_for_query(request.company_name)
    role_label = request.job_title or "指定なし"
    cache_key = "{company}::{role}".format(company=safe_company_name, role=role_label)

    # キャッシュヒット: 即時返却
    retrieved = get_cached_context(cache_key)
    if retrieved:
        return retrieved, "cache"

    # Deep Research (o3-deep-research)
    if USE_DEEP_RESEARCH and safe_company_name:
        try:
            report = run_deep_research(safe_company_name, role_label)
            if report:
                retrieved = [report]
                set_cached_context(cache_key, retrieved)
                return retrieved, "deep_research"
            logger.warning("deep research returned empty result")
        except Exception as exc:
            logger.error("deep research failed error=%s", exc, exc_info=True)
            if STRICT_DEEP_RESEARCH:
                raise HTTPException(status_code=502, detail="Deep Research failed")

    # OpenAI Web Searchパイプライン（クエリ生成→並列検索→LLM要約→キャッシュ保存）
    if not retrieved and ALLOW_WEB_SEARCH_FALLBACK and safe_company_name:
        logger.info("web search pipeline start company=%s role=%s", safe_company_name, role_label)
        try:
            summary = _run_async(_run_web_search_pipeline, safe_company_name, role_label)
            if summary:
                retrieved = [summary]
                set_cached_context(cache_key, retrieved)
                return retrieved, "web_search"
            logger.warning("web search pipeline returned empty result company=%s", safe_company_name)
        except Exception as exc:
            logger.error("web search pipeline failed error=%s", exc, exc_info=True)

    return retrieved or [], "none"


@app.post("/resume/review/stream")
def review_resume_stream(request: ReviewRequest) -> StreamingResponse:
    """RAGレポートをSSEでストリーミング配信するエンドポイント。"""
    safe_company_for_prompt = _sanitize_company_name_for_query(request.company_name)
    role_label = _sanitize_job_title(request.job_title) if request.job_title else "指定なし"

    # コンテキスト収集（キャッシュ/DeepResearch/Web Search）
    retrieved, context_source = _gather_context(request)

    source_labels = {
        "deep_research": "OpenAI Deep Research（o3-deep-research）",
        "web_search": "OpenAI Web Search（gpt-4o-search-preview）",
        "cache": "chromadb キャッシュ（以前の検索結果）",
        "none": "事前学習データのみ（外部検索なし）",
    }
    source_label = source_labels.get(context_source, context_source)

    def generate() -> Generator[str, None, None]:
        api_key = os.getenv("OPENAI_API_KEY")
        if not api_key:
            yield "data: {}\n\n".format(
                json.dumps({"type": "error", "message": "OPENAI_API_KEY not set"}, ensure_ascii=False))
            return

        model = os.getenv("OPENAI_REVIEW_MODEL", "gpt-4o-mini")
        context_block = "\n\n".join(retrieved) if retrieved else "（外部情報なし）"

        prompt = (
            "以下の企業の採用観点に照らして、候補者の履歴書のレビューレポートを日本語で作成してください。\n\n"
            "【企業名】{company}\n"
            "【職種】{role}\n"
            "【企業情報（参考）】\n{context}\n\n"
            "【履歴書テキスト】\n{resume}\n\n"
            "以下のフォーマットに従って出力してください:\n\n"
            "【企業別レビュー報告書】\n---\n"
            "#### ■ 対象企業\n{company}\n\n"
            "#### ■ この企業が求めている核心的要素\n- ...\n\n"
            "#### ■ 履歴書の最適化アドバイス\n"
            "- **強みの再定義**: ...\n"
            "- **不足している情報の補足**: ...\n\n"
            "#### ■ 職種別アドバイス（{role}）\n"
            "この職種特有の評価ポイントを3点以上、具体的に記述してください。\n\n"
            "#### ■ 修正後の自己PRイメージ\n...\n\n"
            "#### ■ 情報の信頼度・参照元\n"
            "- 情報ソース: {source}\n"
            "- 注意: 外部情報に基づく内容は変化する可能性があります。最新情報は企業公式サイトで確認してください。\n"
        ).format(
            company=safe_company_for_prompt,
            role=role_label,
            context=context_block,
            resume=request.resume_text[:RESUME_REVIEW_INPUT_CHAR_LIMIT],
            source=source_label,
        )

        try:
            client = OpenAI(api_key=api_key)
            stream = client.chat.completions.create(
                model=model,
                messages=[
                    {"role": "system", "content": "あなたはプロのキャリアアドバイザーです。"},
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
                        json.dumps({"type": "chunk", "text": delta.content}, ensure_ascii=False))
            yield "data: {}\n\n".format(json.dumps({"type": "done"}, ensure_ascii=False))
        except Exception as exc:
            logger.error("review_stream generate error: %s", exc)
            yield "data: {}\n\n".format(json.dumps({"type": "error", "message": str(exc)}, ensure_ascii=False))

    return StreamingResponse(generate(), media_type="text/event-stream")


@app.post("/resume/review", response_model=ReviewResponse)
def review_resume(request: ReviewRequest) -> ReviewResponse:
    role_label = request.job_title or "指定なし"
    retrieved, context_source = _gather_context(request)

    report = run_crewai(
        resume_text=request.resume_text,
        company_name=request.company_name,
        job_title=role_label,
        context_docs=retrieved,
        context_source=context_source,
    )

    return ReviewResponse(report=report)


# ── ES添削エンドポイント ──────────────────────────────────────────────────────


class ESReviewRequest(BaseModel):
    es_text: str = Field(min_length=1)
    question_type: str = Field(default="その他")
    company_name: str = Field(default="")


class ESReviewResponse(BaseModel):
    specificity_score: int  # 1-10: 具体性
    star_score: int  # 1-10: STAR法準拠
    company_fit_score: Optional[int]  # 1-10: 企業適合性（企業名なしは null）
    length_balance_score: int  # 1-10: 文字数バランス
    feedback: str  # 全体フィードバック文
    improved_text: str  # 改善後テキスト


def _run_es_review(
        es_text: str,
        question_type: str,
        company_name: str,
        context_docs: List[str],
) -> ESReviewResponse:
    import json as _json
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise HTTPException(status_code=500, detail="OPENAI_API_KEY is required")
    client = OpenAI(api_key=api_key)
    model = os.getenv("OPENAI_CHAT_MODEL", "gpt-4o")
    has_company = bool(company_name.strip())
    context_text = "\n\n".join(context_docs) if context_docs else ""
    company_section = (
        f"\n\n【企業情報】\n{context_text[:2000]}" if context_text else ""
    )
    company_fit_key = (
        '"company_fit_score": <1-10の整数: 企業の価値観・求める人物像との適合度>'
        if has_company
        else '"company_fit_score": null'
    )
    system_prompt = (
        "あなたは就職活動の専門アドバイザーです。"
        "学生のES文章を添削し、以下のJSONのみを返してください。説明文は不要です。"
    )
    user_prompt = (
            f"【質問種別】{question_type}\n"
            f"【ES文章】\n{es_text}"
            + (f"\n\n【志望企業】{company_name}" if has_company else "")
            + company_section
            + f"""

以下のJSONフォーマットで添削結果を返してください:
{{
  "specificity_score": <1-10の整数: 具体的な数値・エピソード・固有名詞が含まれているか>,
  "star_score": <1-10の整数: Situation/Task/Action/Resultの構造が揃っているか>,
  {company_fit_key},
  "length_balance_score": <1-10の整数: 文字数・各要素のバランスが適切か>,
  "feedback": "<具体性・STAR準拠・企業適合性・文字数について200字以内でアドバイス>",
  "improved_text": "<元の文章を改善したバージョン（元の文字数の110〜130%を目安）>"
}}"""
    )
    try:
        resp = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.3,
            max_tokens=1200,
            response_format={"type": "json_object"},
        )
        data = _json.loads(resp.choices[0].message.content or "{}")
        company_fit = data.get("company_fit_score")
        if company_fit is not None:
            company_fit = max(1, min(10, int(company_fit)))
        return ESReviewResponse(
            specificity_score=max(1, min(10, int(data.get("specificity_score", 5)))),
            star_score=max(1, min(10, int(data.get("star_score", 5)))),
            company_fit_score=company_fit,
            length_balance_score=max(1, min(10, int(data.get("length_balance_score", 5)))),
            feedback=str(data.get("feedback", "")),
            improved_text=str(data.get("improved_text", "")),
        )
    except Exception as exc:
        logger.warning("es review failed error=%s", exc)
        raise HTTPException(status_code=500, detail=f"ES review failed: {exc}")


@app.post("/es/review", response_model=ESReviewResponse)
def es_review(request: ESReviewRequest) -> ESReviewResponse:
    context_docs: List[str] = []
    safe_company_name = ""
    if request.company_name.strip():
        safe_company_name = _sanitize_company_name_for_query(request.company_name)
        cache_key = "{company}::es_review".format(company=safe_company_name)
        context_docs = get_cached_context(
            cache_key, query=f"{safe_company_name} 求める人物像 採用 価値観"
        )
        if not context_docs:
            logger.info("es review web search start company=%s", safe_company_name)
            try:
                summary = _run_async(_run_web_search_pipeline, safe_company_name, "")
                if summary:
                    context_docs = [summary]
                    set_cached_context(cache_key, context_docs)
            except Exception as exc:
                logger.warning("es review web search failed company=%s error=%s", safe_company_name, exc)

    return _run_es_review(
        es_text=request.es_text,
        question_type=request.question_type,
        company_name=safe_company_name,
        context_docs=context_docs,
    )
