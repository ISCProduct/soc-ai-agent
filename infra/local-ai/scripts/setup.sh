#!/usr/bin/env bash
# 推論 PC での初回セットアップ。イメージ取得・起動・Ollama モデル pull まで。
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd docker curl

if [[ -f .env ]]; then
  :
else
  cp .env.example .env
  echo "[setup] .env を .env.example から作成しました。必要なら LOCAL_AI_BIND やモデルを編集してください。"
fi

if [[ "${LOCAL_AI_BIND}" == "0.0.0.0" || "${LOCAL_AI_BIND}" == "::" ]]; then
  echo "[setup] 警告: LOCAL_AI_BIND=${LOCAL_AI_BIND} は全インターフェース公開です。認証のない API なので避けてください。" >&2
fi

echo "[setup] Whisper / Ollama / VOICEVOX を起動します（初回はモデル取得で数分かかることがある）"
compose up -d --build

echo "[setup] ollama-init（pull + warm-up）の完了を待ちます"
# init は restart: "no"。終了コードを見る。
for i in $(seq 1 180); do
  status="$(compose ps -a --format '{{.Service}} {{.Status}}' | awk '$1=="ollama-init"{print substr($0, index($0,$2))}')"
  if [[ "${status}" == Exited*\(0\)* ]] || [[ "${status}" == *"Exited (0)"* ]]; then
    echo "[setup] ollama-init 完了: ${status}"
    break
  fi
  if [[ "${status}" == Exited* ]] || [[ "${status}" == *"Exited ("* ]]; then
    echo "[setup] ollama-init が失敗しました: ${status}" >&2
    compose logs ollama-init >&2 || true
    exit 1
  fi
  sleep 2
  if [[ "${i}" -eq 180 ]]; then
    echo "[setup] ollama-init 待ちがタイムアウト: ${status:-unknown}" >&2
    compose logs ollama-init >&2 || true
    exit 1
  fi
done

echo "[setup] 完了。次は ./scripts/healthcheck.sh と ./scripts/smoke.sh を実行してください。"
echo "[setup] Backend へ渡す接続先の例（VPN/トンネル越しの到達可能なホストに置き換え）:"
echo "  AI_FAILOVER_LOCAL_STT_URL=http://<到達可能なホスト>:${LOCAL_AI_STT_PORT}"
echo "  AI_FAILOVER_LOCAL_LLM_URL=http://<到達可能なホスト>:${LOCAL_AI_LLM_PORT}/v1"
echo "  AI_FAILOVER_LOCAL_LLM_MODEL=${OLLAMA_MODEL}"
echo "  AI_FAILOVER_LOCAL_TTS_URL=http://<到達可能なホスト>:${LOCAL_AI_TTS_PORT}"
echo "  AI_FAILOVER_LOCAL_TTS_SPEAKER=${VOICEVOX_SPEAKER}"
echo "[setup] 注: 本番 Backend は Fargate のため、推論 PC の LAN IP だけでは届きません。"
