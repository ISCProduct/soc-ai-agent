#!/bin/bash
# Discordスラッシュコマンド /prod, /prod-uptime, /prod-uptime-list をアプリケーションに登録する。
# 一度実行すれば以後は再実行不要（コマンド内容を変更したときのみ再実行）。
#
# 必要な環境変数:
#   DISCORD_BOT_TOKEN     Discord Developer Portal > Bot > Token
#   DISCORD_APPLICATION_ID Discord Developer Portal > General Information > Application ID
#
# 使い方:
#   DISCORD_BOT_TOKEN=xxx DISCORD_APPLICATION_ID=yyy ./automation/discord/register-commands.sh

set -euo pipefail

if [ -z "${DISCORD_BOT_TOKEN:-}" ] || [ -z "${DISCORD_APPLICATION_ID:-}" ]; then
  echo "DISCORD_BOT_TOKEN と DISCORD_APPLICATION_ID を環境変数で指定してください" >&2
  exit 1
fi

curl -sf -X PUT \
  "https://discord.com/api/v10/applications/${DISCORD_APPLICATION_ID}/commands" \
  -H "Authorization: Bot ${DISCORD_BOT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '[
    {
      "name": "prod-uptime",
      "description": "本番を終日起動する日付を追加します",
      "type": 1
    },
    {
      "name": "prod-uptime-list",
      "description": "本番の起動予定日と現在の設定を表示します(誰でも閲覧可)",
      "type": 1
    },
    {
      "name": "prod",
      "description": "本番環境を起動/停止します",
      "type": 1,
      "options": [
        {
          "name": "state",
          "description": "on=常時起動 / off=常時停止 / auto=日付リストに従う",
          "type": 3,
          "required": true,
          "choices": [
            { "name": "on (今すぐ起動して起動し続ける)", "value": "on" },
            { "name": "off (今すぐ停止して停止し続ける)", "value": "off" },
            { "name": "auto (日付リストに従う・既定)", "value": "auto" }
          ]
        }
      ]
    }
  ]'

echo
echo "コマンド登録完了。Discordサーバーで /prod, /prod-uptime, /prod-uptime-list が使えるようになります(反映まで数分かかる場合があります)。"
