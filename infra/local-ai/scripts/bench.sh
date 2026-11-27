#!/usr/bin/env bash
# メモリと 1 回あたりの応答時間の目安を取る。実機（Ubuntu 8GB）での公式値ではない。
# 結果は標準出力に出し、README の「実機未検証」欄へ転記する想定。
# LLM はコールド（可能なら）と 2 回目以降を分けて測る。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd curl python3 docker

echo "== docker stats（瞬間値） =="
docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}' \
  $(compose ps -q) 2>/dev/null || docker stats --no-stream

WAV="${ROOT}/testdata/smoke.wav"

time_ms() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

curl_ok() {
  # 2xx 以外は失敗扱い（-f）。測定値にエラー応答の短時間を混ぜない。
  curl -fsS "$@"
}

echo "== 応答時間 =="
t0="$(time_ms)"
curl_ok -o /dev/null \
  -F "file=@${WAV};filename=smoke.wav;type=audio/wav" \
  -F "response_format=json" \
  -F "language=ja" \
  "${LOCAL_AI_STT_URL}/inference"
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

# 1 回目（warm 済みでも「測定1」、KEEP_ALIVE なし環境ではコールドに近い）
t0="$(time_ms)"
curl_ok -o /dev/null -H 'Content-Type: application/json' -d "${payload}" \
  "${LOCAL_AI_LLM_URL}/v1/chat/completions"
t1="$(time_ms)"
echo "LLM /v1/chat/completions 1回目 (${OLLAMA_MODEL}): $((t1 - t0)) ms"

t0="$(time_ms)"
curl_ok -o /dev/null -H 'Content-Type: application/json' -d "${payload}" \
  "${LOCAL_AI_LLM_URL}/v1/chat/completions"
t1="$(time_ms)"
echo "LLM /v1/chat/completions 2回目 (${OLLAMA_MODEL}): $((t1 - t0)) ms"

aq_url="$(VOICEVOX_SPEAKER="${VOICEVOX_SPEAKER}" LOCAL_AI_TTS_URL="${LOCAL_AI_TTS_URL}" python3 - <<'PY'
import os, urllib.parse
base = os.environ["LOCAL_AI_TTS_URL"].rstrip("/")
q = urllib.parse.urlencode({"text": "次の質問です。", "speaker": os.environ["VOICEVOX_SPEAKER"]})
print(f"{base}/audio_query?{q}")
PY
)"
aq="$(tmpfile)"
t0="$(time_ms)"
curl_ok -o "${aq}" -X POST "${aq_url}"
curl_ok -o /dev/null -X POST -H 'Content-Type: application/json' --data-binary @"${aq}" \
  "${LOCAL_AI_TTS_URL}/synthesis?speaker=${VOICEVOX_SPEAKER}"
t1="$(time_ms)"
echo "TTS audio_query+synthesis: $((t1 - t0)) ms"

echo
echo "host memory (free -h):"
if command -v free >/dev/null 2>&1; then
  free -h
else
  echo "(free が無い環境です。推論 PC の Ubuntu で実行してください)"
fi
echo
echo "注: LLM 1回目と2回目を比較する。OLLAMA_KEEP_ALIVE=-1 と ollama-init の warm-up 後なら両方とも warm に近い。"
echo "    Backend のローカル1工程上限は AI_FAILOVER_LOCAL_TIMEOUT_SECONDS（既定25秒）。"
