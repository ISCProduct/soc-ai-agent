#!/bin/sh
# Docker Compose 開発用 entrypoint: DB マイグレーション後に本コマンドを起動する
set -eu

echo "[entrypoint] applying database migrations..."
attempt=0
max_attempts=30
until go run ./cmd/migrate up; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge "$max_attempts" ]; then
    echo "[entrypoint] migration failed after ${max_attempts} attempts" >&2
    exit 1
  fi
  echo "[entrypoint] migrate not ready (attempt ${attempt}/${max_attempts}), retrying in 2s..."
  sleep 2
done
echo "[entrypoint] migrations applied"

echo "[entrypoint] starting: $*"
exec "$@"
