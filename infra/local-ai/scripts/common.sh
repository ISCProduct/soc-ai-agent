#!/usr/bin/env bash
# infra/local-ai 配下のスクリプト共通。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

LOCAL_AI_BIND="${LOCAL_AI_BIND:-127.0.0.1}"
LOCAL_AI_STT_PORT="${LOCAL_AI_STT_PORT:-8080}"
LOCAL_AI_LLM_PORT="${LOCAL_AI_LLM_PORT:-11434}"
LOCAL_AI_TTS_PORT="${LOCAL_AI_TTS_PORT:-50021}"
OLLAMA_MODEL="${OLLAMA_MODEL:-qwen3:1.7b}"
VOICEVOX_SPEAKER="${VOICEVOX_SPEAKER:-1}"

# 呼び出し元の汎用 STT_URL 等は引き継がない（誤って外部 URL を叩かない）。
LOCAL_AI_STT_URL="${LOCAL_AI_STT_URL:-http://${LOCAL_AI_BIND}:${LOCAL_AI_STT_PORT}}"
LOCAL_AI_LLM_URL="${LOCAL_AI_LLM_URL:-http://${LOCAL_AI_BIND}:${LOCAL_AI_LLM_PORT}}"
LOCAL_AI_TTS_URL="${LOCAL_AI_TTS_URL:-http://${LOCAL_AI_BIND}:${LOCAL_AI_TTS_PORT}}"

compose() {
  docker compose -f "${ROOT}/docker-compose.yml" "$@"
}

require_cmd() {
  local c
  for c in "$@"; do
    if ! command -v "${c}" >/dev/null 2>&1; then
      echo "必要なコマンドがありません: ${c}" >&2
      exit 1
    fi
  done
}

http_code() {
  local url="$1"
  curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 3 --max-time 10 "${url}" || echo "000"
}

# mktemp ファイルを終了時に消す。
cleanup_files=()
cleanup() {
  local f
  for f in "${cleanup_files[@]:-}"; do
    rm -f "${f}"
  done
}
trap cleanup EXIT

tmpfile() {
  local f
  f="$(mktemp)"
  cleanup_files+=("${f}")
  printf '%s' "${f}"
}
