#!/usr/bin/env bash
# aifailover.LocalSTT / LocalLLM / LocalTTS と同じ契約で 1 回ずつ叩く。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd curl python3

WAV="${ROOT}/testdata/smoke.wav"
if [[ ! -f "${WAV}" ]]; then
  echo "testdata/smoke.wav がありません" >&2
  exit 1
fi

echo "== STT (Whisper.cpp POST /inference) =="
stt_body="$(tmpfile)"
stt_code="$(curl -sS -o "${stt_body}" -w '%{http_code}' \
  -F "file=@${WAV};filename=smoke.wav;type=audio/wav" \
  -F "response_format=json" \
  -F "language=ja" \
  -F "prompt=" \
  "${LOCAL_AI_STT_URL}/inference")"
if [[ "${stt_code}" != "200" ]]; then
  echo "STT HTTP ${stt_code}: $(head -c 400 "${stt_body}")" >&2
  exit 1
fi
python3 - "${stt_body}" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    d = json.load(f)
text = (d.get("text") or "").strip()
print("stt text:", repr(text[:120]))
# トーンのみの WAV なので空でも「応答形式が JSON」なら契約は満たす
if "text" not in d:
    raise SystemExit("STT: text フィールドが無い")
PY

echo "== LLM (Ollama OpenAI 互換 POST /v1/chat/completions) =="
# Backend LocalLLM と同じ max_tokens=200。Qwen3 が <think> だけで使い切ると面接は失敗するため、
# strip 後が空ならここで落とす。
llm_body="$(tmpfile)"
llm_payload="$(OLLAMA_MODEL="${OLLAMA_MODEL}" python3 - <<'PY'
import json, os
print(json.dumps({
    "model": os.environ["OLLAMA_MODEL"],
    "messages": [
        {"role": "system", "content": "あなたは面接官です。短く質問してください。"},
        {"role": "user", "content": "自己紹介をお願いします、と言われました。一言で返してください。"},
    ],
    "max_tokens": 200,
    "temperature": 0.7,
    "stream": False,
}))
PY
)"
llm_code="$(curl -sS -o "${llm_body}" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "${llm_payload}" \
  "${LOCAL_AI_LLM_URL}/v1/chat/completions")"
if [[ "${llm_code}" != "200" ]]; then
  echo "LLM HTTP ${llm_code}: $(head -c 400 "${llm_body}")" >&2
  exit 1
fi
python3 - "${llm_body}" <<'PY'
import json, re, sys

def strip_thinking(text: str) -> str:
    # Backend/internal/aifailover/local.go の stripThinking と同じ方針。
    while True:
        start = text.find("<think>")
        if start < 0:
            break
        end = text.find("</think>", start)
        if end < 0:
            text = text[:start]
            break
        text = text[:start] + text[end + len("</think>"):]
    return text.strip()

with open(sys.argv[1]) as f:
    d = json.load(f)
content = ((d.get("choices") or [{}])[0].get("message") or {}).get("content") or ""
stripped = strip_thinking(content)
print("llm content (stripped):", repr(stripped[:160]))
if not stripped:
    raise SystemExit("LLM: <think> 除去後が空（Backend でも失敗する）")
PY

echo "== TTS (VOICEVOX /audio_query → /synthesis) =="
aq_url="$(VOICEVOX_SPEAKER="${VOICEVOX_SPEAKER}" LOCAL_AI_TTS_URL="${LOCAL_AI_TTS_URL}" python3 - <<'PY'
import os, urllib.parse
text = "次の質問です。自己紹介をお願いします。"
speaker = os.environ["VOICEVOX_SPEAKER"]
base = os.environ["LOCAL_AI_TTS_URL"].rstrip("/")
print(f"{base}/audio_query?{urllib.parse.urlencode({'text': text, 'speaker': speaker})}")
PY
)"
aq_body="$(tmpfile)"
aq_code="$(curl -sS -o "${aq_body}" -w '%{http_code}' -X POST "${aq_url}")"
if [[ "${aq_code}" != "200" ]]; then
  echo "TTS audio_query HTTP ${aq_code}: $(head -c 400 "${aq_body}")" >&2
  exit 1
fi
syn_url="${LOCAL_AI_TTS_URL}/synthesis?speaker=${VOICEVOX_SPEAKER}"
wav_body="$(tmpfile)"
syn_code="$(curl -sS -o "${wav_body}" -w '%{http_code}' \
  -X POST \
  -H 'Content-Type: application/json' \
  --data-binary @"${aq_body}" \
  "${syn_url}")"
if [[ "${syn_code}" != "200" ]]; then
  echo "TTS synthesis HTTP ${syn_code}" >&2
  exit 1
fi
python3 - "${wav_body}" <<'PY'
import os, sys
path = sys.argv[1]
with open(path, "rb") as f:
    b = f.read(12)
if len(b) < 12 or b[0:4] != b"RIFF" or b[8:12] != b"WAVE":
    raise SystemExit(f"TTS: WAV ではない header={b!r}")
print("tts wav header ok, bytes=", os.path.getsize(path))
PY

echo "smoke ok"
