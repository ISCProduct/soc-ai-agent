#!/usr/bin/env bash
# 運用アラート(起動ジョブの失敗など)を Discord へ投稿する。
#
# 使い方: automation/ops/notify-discord.sh "<本文>"
#
# Webhook は DISCORD_OPS_WEBHOOK_URL を優先し、未設定なら DISCORD_RELEASE_WEBHOOK_URL を使う。
# どちらも無い場合は「黙ってスキップ」しない。アラート経路が沈黙していること自体が事故であり、
# 実際 DISCORD_DEPLOY_WEBHOOK_URL は未設定のままデプロイスモークの通知が1件も飛んでいなかった(#1354)。
set -uo pipefail

content="${1:-}"
if [ -z "$content" ]; then
  echo "::error::通知本文が空です" >&2
  exit 2
fi

url="${DISCORD_OPS_WEBHOOK_URL:-${DISCORD_RELEASE_WEBHOOK_URL:-}}"
if [ -z "$url" ]; then
  echo "::error::Discord Webhook が未設定のため通知できません(DISCORD_OPS_WEBHOOK_URL または DISCORD_RELEASE_WEBHOOK_URL を設定してください)" >&2
  echo "通知できなかった内容: $content" >&2
  exit 1
fi

payload="$(python3 -c "import json,sys; print(json.dumps({'content': sys.argv[1]}))" "$content")"

# テスト用。実際には投稿せずペイロードだけ出す。
if [ -n "${DRY_RUN:-}" ]; then
  echo "$payload"
  exit 0
fi

if ! curl -sS -f --max-time 15 -X POST -H 'Content-Type: application/json' -d "$payload" "$url" >/dev/null; then
  echo "::error::Discord への通知に失敗しました(Webhook URL とチャンネルを確認してください)" >&2
  exit 1
fi
echo "Discord へ通知しました"
