#!/usr/bin/env bash
# 3 サービスの生存確認。Backend の aifailover が叩く手前の到達性だけを見る。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd curl python3

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
check "whisper"  "${LOCAL_AI_STT_URL}/" 200
# Ollama: ネイティブのモデル一覧
check "ollama"   "${LOCAL_AI_LLM_URL}/api/tags" 200
# VOICEVOX: 話者一覧
check "voicevox" "${LOCAL_AI_TTS_URL}/speakers" 200

# /api/tags はモデル未 pull でも 200 なので、設定したモデルがあるか別途確認する。
if [[ "${fail}" -eq 0 ]]; then
  tags_body="$(tmpfile)"
  if ! curl -fsS -o "${tags_body}" "${LOCAL_AI_LLM_URL}/api/tags"; then
    echo "NG  ollama モデル一覧の取得に失敗" >&2
    fail=1
  else
    if OLLAMA_MODEL="${OLLAMA_MODEL}" python3 - "${tags_body}" <<'PY'
import json, os, sys
want = os.environ["OLLAMA_MODEL"]
with open(sys.argv[1]) as f:
    data = json.load(f)
names = [(m.get("name") or m.get("model") or "") for m in (data.get("models") or [])]
for name in names:
    # ollama list は "qwen3:1.7b" のほか digest 付き名もあり得る
    if name == want or name.startswith(want + "-") or name.startswith(want + "@"):
        sys.exit(0)
print("available:", ", ".join(names) or "(none)", file=sys.stderr)
sys.exit(1)
PY
    then
      echo "OK  ollama モデル ${OLLAMA_MODEL} あり"
    else
      echo "NG  ollama にモデル ${OLLAMA_MODEL} がありません（ollama-init / setup.sh の pull を確認）" >&2
      fail=1
    fi
  fi
fi

if [[ "${fail}" -ne 0 ]]; then
  echo "healthcheck failed" >&2
  exit 1
fi
echo "healthcheck ok"
