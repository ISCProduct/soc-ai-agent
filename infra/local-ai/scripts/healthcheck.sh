#!/usr/bin/env bash
# 3 サービスの生存確認。Backend の aifailover が叩く手前の到達性だけを見る。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd curl

fail=0

check() {
  local name="$1" url="$2" expect="$3"
  local code
  code="$(http_code "${url}")"
  if [[ "${code}" == "${expect}" ]]; then
    echo "OK  ${name}  ${url}  HTTP ${code}"
    return
  fi
  echo "NG  ${name}  ${url}  HTTP ${code} (want ${expect})" >&2
  fail=1
}

# Whisper: ルートは UI（200）。/inference は GET だと 4xx になりうるのでルートで見る。
check "whisper"  "${STT_URL}/" 200
# Ollama: ネイティブのモデル一覧（OpenAI 互換の /v1 より軽い）
check "ollama"   "${LLM_URL}/api/tags" 200
# VOICEVOX: 話者一覧
check "voicevox" "${TTS_URL}/speakers" 200

if [[ "${fail}" -ne 0 ]]; then
  echo "healthcheck failed" >&2
  exit 1
fi
echo "healthcheck ok"
