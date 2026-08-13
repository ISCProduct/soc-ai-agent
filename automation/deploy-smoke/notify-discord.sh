#!/usr/bin/env bash
# デプロイスモーク結果を Discord Incoming Webhook へ投稿する。
# DISCORD_DEPLOY_WEBHOOK_URL 未設定ならスキップ（デプロイ本体は壊さない）。
set -euo pipefail

status="${1:-unknown}"       # success | failure
environment="${2:-staging}"
base_url="${3:-}"
run_url="${4:-}"
github_actor="${5:-}"

if [[ -z "${DISCORD_DEPLOY_WEBHOOK_URL:-}" ]]; then
  echo "DISCORD_DEPLOY_WEBHOOK_URL is not set; skip Discord notify"
  exit 0
fi

mention=""
map_file="$(cd "$(dirname "$0")" && pwd)/mention-map.json"
if [[ -n "$github_actor" && -f "$map_file" ]]; then
  discord_id="$(python3 -c "
import json,sys
m=json.load(open(sys.argv[1]))
print(m.get(sys.argv[2],''))
" "$map_file" "$github_actor" 2>/dev/null || true)"
  if [[ -n "$discord_id" ]]; then
    mention="<@${discord_id}>"
  fi
fi

if [[ "$status" == "success" ]]; then
  content="✅ **${environment}** 環境反映後の Playwright スモークが成功しました。
URL: ${base_url}
Run: ${run_url}
${mention}"
else
  if [[ -z "$mention" ]]; then
    mention="@here"
  fi
  content="❌ **${environment}** Playwright スモークが失敗しました。確認お願いします。
URL: ${base_url}
Run: ${run_url}
担当: ${mention}"
fi

payload="$(python3 -c "import json,sys; print(json.dumps({'content': sys.argv[1]}))" "$content")"
curl -sS -X POST -H 'Content-Type: application/json' -d "$payload" "$DISCORD_DEPLOY_WEBHOOK_URL"
echo
