#!/usr/bin/env bash
# #589: ローカル開発用 — 独立 Chroma + RAG を確実に起動する
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PROFILE_ARGS=(--profile rag)

echo "==> Building and starting chroma + rag-review (compose.yml, profile=rag)"
docker compose "${PROFILE_ARGS[@]}" up -d --build chroma rag-review

echo "==> Waiting for health endpoints"
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -sf "http://127.0.0.1:8000/api/v2/heartbeat" >/dev/null \
    && curl -sf "http://127.0.0.1:9000/health" >/dev/null; then
    break
  fi
  sleep 1
done

echo "==> Smoke checks"
CHROMA_HB="$(curl -sf "http://127.0.0.1:8000/api/v2/heartbeat" || true)"
HEALTH="$(curl -sf "http://127.0.0.1:9000/health" || true)"
STATUS="$(curl -sf "http://127.0.0.1:9000/vector/status" || true)"

echo "Chroma heartbeat: ${CHROMA_HB:-FAILED}"
echo "RAG /health:      ${HEALTH:-FAILED}"
echo "RAG /vector/status: ${STATUS:-FAILED}"

if ! echo "${HEALTH}" | grep -q '"vector_store"'; then
  echo ""
  echo "ERROR: /health に vector_store がありません。旧 RAG イメージの可能性があります。"
  echo "  docker compose --profile rag up -d --build --force-recreate rag-review"
  exit 1
fi

if ! echo "${HEALTH}" | grep -q '"ok":true'; then
  echo ""
  echo "ERROR: vector_store.ok が true ではありません。Chroma 接続を確認してください。"
  echo "  docker compose --profile rag ps"
  echo "  docker compose --profile rag logs --tail 50 chroma rag-review"
  exit 1
fi

if [[ -z "${STATUS}" ]]; then
  echo ""
  echo "ERROR: /vector/status が応答しません。"
  exit 1
fi

echo ""
echo "OK: chroma (:8000) + rag-review (:9000) are ready."
echo "  Backend からは RAG_REVIEW_URL=http://rag-review:9000（compose）または http://localhost:9000"
