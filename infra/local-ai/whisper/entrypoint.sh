#!/bin/sh
# Whisper.cpp server を起動する。モデルが無ければ download-ggml-model.sh で取る。
set -eu

MODEL="${WHISPER_MODEL:-tiny}"
MODEL_DIR="${WHISPER_MODEL_DIR:-/models}"
MODEL_PATH="${MODEL_DIR}/ggml-${MODEL}.bin"

mkdir -p "${MODEL_DIR}"

if [ ! -f "${MODEL_PATH}" ]; then
  echo "[whisper] モデル ${MODEL} を ${MODEL_DIR} へ取得します"
  download-ggml-model.sh "${MODEL}" "${MODEL_DIR}"
fi

if [ ! -f "${MODEL_PATH}" ]; then
  echo "[whisper] モデルが見つかりません: ${MODEL_PATH}" >&2
  exit 1
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
