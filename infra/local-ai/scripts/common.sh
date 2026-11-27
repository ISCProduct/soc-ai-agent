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

STT_PORT="${LOCAL_AI_STT_PORT:-8080}"
LLM_PORT="${LOCAL_AI_LLM_PORT:-11434}"
TTS_PORT="${LOCAL_AI_TTS_PORT:-50021}"
OLLAMA_MODEL="${OLLAMA_MODEL:-qwen3:1.7b}"
VOICEVOX_SPEAKER="${VOICEVOX_SPEAKER:-1}"
WHISPER_MODEL="${WHISPER_MODEL:-tiny}"

STT_URL="${STT_URL:-http://127.0.0.1:${STT_PORT}}"
LLM_URL="${LLM_URL:-http://127.0.0.1:${LLM_PORT}}"
TTS_URL="${TTS_URL:-http://127.0.0.1:${TTS_PORT}}"

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
