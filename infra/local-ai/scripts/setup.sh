#!/usr/bin/env bash
# 推論 PC での初回セットアップ。イメージ取得・起動・Ollama モデル pull まで。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd docker curl

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "[setup] .env を .env.example から作成しました。必要ならポートやモデルを編集してください。"
fi

echo "[setup] Whisper / Ollama / VOICEVOX を起動します（初回はモデル取得で数分かかることがある）"
compose up -d --build

echo "[setup] Ollama のモデル ${OLLAMA_MODEL} を pull します"
# ollama コンテナが受け付けるまで待つ
for i in $(seq 1 60); do
  if compose exec -T ollama ollama list >/dev/null 2>&1; then
    break
  fi
  sleep 2
  if [[ "${i}" -eq 60 ]]; then
    echo "[setup] Ollama の起動待ちがタイムアウトしました" >&2
    exit 1
  fi
done
compose exec -T ollama ollama pull "${OLLAMA_MODEL}"

echo "[setup] 完了。次は ./scripts/healthcheck.sh と ./scripts/smoke.sh を実行してください。"
echo "[setup] Backend へ渡す接続先の例（推論 PC の IP に置き換え）:"
echo "  AI_FAILOVER_LOCAL_STT_URL=http://<推論PC>:${STT_PORT}"
echo "  AI_FAILOVER_LOCAL_LLM_URL=http://<推論PC>:${LLM_PORT}/v1"
echo "  AI_FAILOVER_LOCAL_LLM_MODEL=${OLLAMA_MODEL}"
echo "  AI_FAILOVER_LOCAL_TTS_URL=http://<推論PC>:${TTS_PORT}"
echo "  AI_FAILOVER_LOCAL_TTS_SPEAKER=${VOICEVOX_SPEAKER}"
