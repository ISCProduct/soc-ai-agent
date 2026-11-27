#!/bin/sh
# Whisper.cpp server を起動する。モデルが無ければ download-ggml-model.sh で取る。
set -eu

MODEL="${WHISPER_MODEL:-tiny}"
MODEL_DIR="${WHISPER_MODEL_DIR:-/models}"
MODEL_PATH="${MODEL_DIR}/ggml-${MODEL}.bin"
# 公式イメージの PATH は /app/build/bin のみで、スクリプトは /app/models にある。
DOWNLOAD_SCRIPT="${WHISPER_DOWNLOAD_SCRIPT:-/app/models/download-ggml-model.sh}"
# 途中失敗の残骸を弾く下限（tiny でも数十 MB。1MB 未満は不完全とみなす）。
MIN_MODEL_BYTES="${WHISPER_MIN_MODEL_BYTES:-1000000}"

mkdir -p "${MODEL_DIR}"

model_ok() {
  [ -f "${MODEL_PATH}" ] || return 1
  size="$(wc -c < "${MODEL_PATH}" | tr -d ' ')"
  [ "${size}" -ge "${MIN_MODEL_BYTES}" ]
}

if ! model_ok; then
  echo "[whisper] モデル ${MODEL} を ${MODEL_DIR} へ取得します"
  rm -f "${MODEL_PATH}"
  if [ ! -x "${DOWNLOAD_SCRIPT}" ] && [ ! -f "${DOWNLOAD_SCRIPT}" ]; then
    echo "[whisper] ダウンロードスクリプトがありません: ${DOWNLOAD_SCRIPT}" >&2
    exit 1
  fi
  if ! "${DOWNLOAD_SCRIPT}" "${MODEL}" "${MODEL_DIR}"; then
    echo "[whisper] モデル取得に失敗しました。残骸を削除します: ${MODEL_PATH}" >&2
    rm -f "${MODEL_PATH}"
    exit 1
  fi
  if ! model_ok; then
    echo "[whisper] モデルが不完全です: ${MODEL_PATH}" >&2
    rm -f "${MODEL_PATH}"
    exit 1
  fi
fi

THREADS_ARGS=""
if [ -n "${WHISPER_THREADS:-}" ]; then
  THREADS_ARGS="-t ${WHISPER_THREADS}"
fi

echo "[whisper] whisper-server を起動します model=${MODEL_PATH} host=0.0.0.0:8080"
# --convert: webm 等を ffmpeg 経由で WAV へ（面接の録音形式に合わせる）
# language はリクエストの multipart で ja を渡す（aifailover.LocalSTT）
# shellcheck disable=SC2086
exec whisper-server \
  --host 0.0.0.0 \
  --port 8080 \
  -m "${MODEL_PATH}" \
  --convert \
  ${THREADS_ARGS}
