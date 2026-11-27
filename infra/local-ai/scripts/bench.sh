#!/usr/bin/env bash
# メモリと 1 回あたりの応答時間の目安を取る。実機（Ubuntu 8GB）での公式値ではない。
# 結果は標準出力に出し、README の「実機未検証」欄へ転記する想定。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd curl python3 docker

echo "== docker stats（瞬間値） =="
compose ps --format json >/dev/null 2>&1 || compose ps >/dev/null
docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}' \
  $(compose ps -q) 2>/dev/null || docker stats --no-stream

WAV="${ROOT}/testdata/smoke.wav"

time_ms() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

echo "== 応答時間（単発） =="
t0="$(time_ms)"
curl -sS -o /dev/null \
  -F "file=@${WAV};filename=smoke.wav;type=audio/wav" \
  -F "response_format=json" \
  -F "language=ja" \
  "${STT_URL}/inference"
t1="$(time_ms)"
echo "STT /inference: $((t1 - t0)) ms"

payload="$(OLLAMA_MODEL="${OLLAMA_MODEL}" python3 - <<'PY'
import json, os
print(json.dumps({
    "model": os.environ["OLLAMA_MODEL"],
    "messages": [
        {"role": "system", "content": "短く返してください。"},
        {"role": "user", "content": "こんにちは"},
    ],
    "max_tokens": 64,
    "stream": False,
}))
PY
)"
t0="$(time_ms)"
curl -sS -o /dev/null -H 'Content-Type: application/json' -d "${payload}" \
  "${LLM_URL}/v1/chat/completions"
t1="$(time_ms)"
echo "LLM /v1/chat/completions (${OLLAMA_MODEL}): $((t1 - t0)) ms"

aq_url="$(VOICEVOX_SPEAKER="${VOICEVOX_SPEAKER}" TTS_URL="${TTS_URL}" python3 - <<'PY'
import os, urllib.parse
base = os.environ["TTS_URL"].rstrip("/")
q = urllib.parse.urlencode({"text": "次の質問です。", "speaker": os.environ["VOICEVOX_SPEAKER"]})
print(f"{base}/audio_query?{q}")
PY
)"
aq="$(mktemp)"
t0="$(time_ms)"
curl -sS -o "${aq}" -X POST "${aq_url}"
curl -sS -o /dev/null -X POST -H 'Content-Type: application/json' --data-binary @"${aq}" \
  "${TTS_URL}/synthesis?speaker=${VOICEVOX_SPEAKER}"
t1="$(time_ms)"
rm -f "${aq}"
echo "TTS audio_query+synthesis: $((t1 - t0)) ms"

echo
echo "host memory (free -h):"
if command -v free >/dev/null 2>&1; then
  free -h
else
  echo "(free が無い環境です。推論 PC の Ubuntu で実行してください)"
fi
