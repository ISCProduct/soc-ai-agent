#!/usr/bin/env bash
# デプロイスモーク結果を Discord Incoming Webhook へ投稿する。
#
# Webhook は DISCORD_DEPLOY_WEBHOOK_URL → DISCORD_OPS_WEBHOOK_URL → DISCORD_RELEASE_WEBHOOK_URL
# の順に探す。DISCORD_DEPLOY_WEBHOOK_URL はリポジトリにも environment にも存在せず、
# 通知が1件も飛ばないまま「スキップ」されていた(#1354)。
#
# どれも無い場合は ::error:: を出して気づけるようにするが、終了コードは 0 のままにする。
# 通知先の設定漏れでデプロイ自体を落とすのは blast radius が大きすぎる。
# (起動ジョブ側の automation/ops/notify-discord.sh は、ジョブが既に失敗している文脈なので 1 を返す)
set -euo pipefail

status="${1:-unknown}"       # success | failure
environment="${2:-staging}"
base_url="${3:-}"
run_url="${4:-}"
github_actor="${5:-}"

webhook_url="${DISCORD_DEPLOY_WEBHOOK_URL:-${DISCORD_OPS_WEBHOOK_URL:-${DISCORD_RELEASE_WEBHOOK_URL:-}}}"
if [[ -z "$webhook_url" ]]; then
  echo "::error::Discord Webhook が未設定のためスモーク結果を通知できません(DISCORD_DEPLOY_WEBHOOK_URL / DISCORD_OPS_WEBHOOK_URL / DISCORD_RELEASE_WEBHOOK_URL のいずれかを設定してください)" >&2
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
curl -sS -X POST -H 'Content-Type: application/json' -d "$payload" "$webhook_url"
echo
