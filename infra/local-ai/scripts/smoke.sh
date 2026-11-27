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
stt_body="$(mktemp)"
stt_code="$(curl -sS -o "${stt_body}" -w '%{http_code}' \
  -F "file=@${WAV};filename=smoke.wav;type=audio/wav" \
  -F "response_format=json" \
  -F "language=ja" \
  -F "prompt=" \
  "${STT_URL}/inference")"
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
rm -f "${stt_body}"

echo "== LLM (Ollama OpenAI 互換 POST /v1/chat/completions) =="
llm_body="$(mktemp)"
llm_payload="$(OLLAMA_MODEL="${OLLAMA_MODEL}" python3 - <<'PY'
import json, os
print(json.dumps({
    "model": os.environ["OLLAMA_MODEL"],
    "messages": [
        {"role": "system", "content": "あなたは面接官です。短く質問してください。"},
        {"role": "user", "content": "自己紹介をお願いします、と言われました。一言で返してください。"},
    ],
    "max_tokens": 80,
    "temperature": 0.7,
    "stream": False,
}))
PY
)"
llm_code="$(curl -sS -o "${llm_body}" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "${llm_payload}" \
  "${LLM_URL}/v1/chat/completions")"
if [[ "${llm_code}" != "200" ]]; then
  echo "LLM HTTP ${llm_code}: $(head -c 400 "${llm_body}")" >&2
  exit 1
fi
python3 - "${llm_body}" <<'PY'
import json, re, sys
with open(sys.argv[1]) as f:
    d = json.load(f)
content = ((d.get("choices") or [{}])[0].get("message") or {}).get("content") or ""
# Qwen3 の <think> は Backend 側で落とす。ここでは生の応答があることだけ見る。
stripped = re.sub(r"<think>[\s\S]*?</think>", "", content).strip()
print("llm content (stripped):", repr(stripped[:160]))
if not content.strip():
    raise SystemExit("LLM: 空の応答")
PY
rm -f "${llm_body}"

echo "== TTS (VOICEVOX /audio_query → /synthesis) =="
# LocalTTS と同じく query に text / speaker を載せる（POST）。
aq_url="$(VOICEVOX_SPEAKER="${VOICEVOX_SPEAKER}" TTS_URL="${TTS_URL}" python3 - <<'PY'
import os, urllib.parse
text = "次の質問です。自己紹介をお願いします。"
speaker = os.environ["VOICEVOX_SPEAKER"]
base = os.environ["TTS_URL"].rstrip("/")
print(f"{base}/audio_query?{urllib.parse.urlencode({'text': text, 'speaker': speaker})}")
PY
)"
aq_body="$(mktemp)"
aq_code="$(curl -sS -o "${aq_body}" -w '%{http_code}' -X POST "${aq_url}")"
if [[ "${aq_code}" != "200" ]]; then
  echo "TTS audio_query HTTP ${aq_code}: $(head -c 400 "${aq_body}")" >&2
  exit 1
fi
syn_url="${TTS_URL}/synthesis?speaker=${VOICEVOX_SPEAKER}"
wav_body="$(mktemp)"
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
rm -f "${aq_body}" "${wav_body}"

echo "smoke ok"
