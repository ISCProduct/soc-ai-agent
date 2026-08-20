"""環境変数由来の設定値。"""
from __future__ import annotations

import os

DEFAULT_CACHE_TTL_SECONDS = 86400
DEFAULT_MAX_EMBED_TOKENS = 8191
DEFAULT_EMBED_MAX_RETRIES = 3
DEFAULT_HINTS_PARSE_MAX_TOKENS = 600
DEFAULT_RESUME_REVIEW_INPUT_CHAR_LIMIT = 10000
# #996: embeddings経路(langchain_runtime.py)は既にRAG_OPENAI_TIMEOUT_SEC経由で
# タイムアウトを設定済みだったが、chat.completions.create/responses.create系
# (research.py/es_review.py/hints.py/resume.py)は未設定でOpenAI側の応答が
# 極端に遅延するとハングしうる状態だった。同じ環境変数・既定値で統一する。
DEFAULT_OPENAI_TIMEOUT_SEC = 60.0

DEFAULT_WEB_SEARCH_MODEL = "gpt-4o-search-preview"
DEFAULT_SEARCH_LOG_DIR = "/app/search_logs"

CACHE_TTL_SECONDS = int(os.getenv("RAG_SEARCH_CACHE_TTL_SECONDS", str(DEFAULT_CACHE_TTL_SECONDS)))
# Phase 1B (#557): Read 経路の既定はキャッシュ/呼び出し元 brief のみ（Search/$0）
USE_DEEP_RESEARCH = os.getenv("RAG_USE_DEEP_RESEARCH", "false").lower() == "true"
ALLOW_WEB_SEARCH_FALLBACK = os.getenv(
    "RAG_ALLOW_WEB_SEARCH_FALLBACK",
    os.getenv("RAG_ALLOW_DUCKDUCKGO_FALLBACK", "false"),
).lower() == "true"
STRICT_DEEP_RESEARCH = os.getenv("RAG_DEEP_RESEARCH_STRICT", "false").lower() == "true"
CREWAI_VERBOSE = os.getenv("RAG_CREWAI_VERBOSE", "false").lower() == "true"
MAX_EMBED_TOKENS = int(os.getenv("RAG_MAX_EMBED_TOKENS", str(DEFAULT_MAX_EMBED_TOKENS)))
EMBED_MAX_RETRIES = int(os.getenv("RAG_EMBED_MAX_RETRIES", str(DEFAULT_EMBED_MAX_RETRIES)))
OPENAI_TIMEOUT_SEC = float(os.getenv("RAG_OPENAI_TIMEOUT_SEC", str(DEFAULT_OPENAI_TIMEOUT_SEC)))
HINTS_PARSE_MAX_TOKENS = int(os.getenv("RAG_HINTS_PARSE_MAX_TOKENS", str(DEFAULT_HINTS_PARSE_MAX_TOKENS)))
RESUME_REVIEW_INPUT_CHAR_LIMIT = int(
    os.getenv("RAG_REVIEW_RESUME_CHAR_LIMIT", str(DEFAULT_RESUME_REVIEW_INPUT_CHAR_LIMIT)))
WEB_SEARCH_MODEL = os.getenv("OPENAI_WEB_SEARCH_MODEL", DEFAULT_WEB_SEARCH_MODEL)
SEARCH_LOG_DIR = os.getenv("RAG_SEARCH_LOG_DIR", DEFAULT_SEARCH_LOG_DIR)

INTERNAL_TOKEN_HEADER = "X-Internal-Token"
_INTERNAL_AUTH_EXEMPT_PATHS = frozenset({"/health", "/healthz"})
