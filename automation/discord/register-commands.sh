#!/bin/bash
# Discordスラッシュコマンド /prod, /prod-uptime, /prod-uptime-list をアプリケーションに登録する。
# 一度実行すれば以後は再実行不要（コマンド内容を変更したときのみ再実行）。
#
# 必要な環境変数:
#   DISCORD_BOT_TOKEN      Discord Developer Portal > Bot > Token
#   DISCORD_APPLICATION_ID Discord Developer Portal > General Information > Application ID
#
# 使い方:
#   DISCORD_BOT_TOKEN=xxx DISCORD_APPLICATION_ID=yyy ./automation/discord/register-commands.sh
#
# 登録済みの確認だけしたいとき:
#   DISCORD_BOT_TOKEN=xxx DISCORD_APPLICATION_ID=yyy ./automation/discord/register-commands.sh --list

set -euo pipefail

if [ -z "${DISCORD_BOT_TOKEN:-}" ] || [ -z "${DISCORD_APPLICATION_ID:-}" ]; then
  echo "DISCORD_BOT_TOKEN と DISCORD_APPLICATION_ID を環境変数で指定してください" >&2
  exit 1
fi

API="https://discord.com/api/v10/applications/${DISCORD_APPLICATION_ID}/commands"
AUTH="Authorization: Bot ${DISCORD_BOT_TOKEN}"

# call METHOD [BODY] -> レスポンス本文を標準出力、HTTPステータスを戻り値の判定に使う。
#
# 以前は curl -sf で投げていたため、失敗しても Discord が返す理由
# （401 Unauthorized / 403 Missing Access / 50035 Invalid Form Body など）が
# 一切表示されず、「登録されないが理由が分からない」状態になっていた。
call() {
  local method="$1" body="${2:-}" out status
  out=$(mktemp)
  if [ -n "$body" ]; then
    status=$(curl -s -o "$out" -w '%{http_code}' -X "$method" "$API" \
      -H "$AUTH" -H "Content-Type: application/json" -d "$body")
  else
    status=$(curl -s -o "$out" -w '%{http_code}' -X "$method" "$API" -H "$AUTH")
  fi

  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Discord API エラー: HTTP $status" >&2
    # トークンは出力しない。レスポンス本文にはトークンは含まれない
    cat "$out" >&2
    echo >&2
    case "$status" in
      401) echo "ヒント: Bot Token が誤っているか失効しています（Application ID ではなく Bot タブの Token）。" >&2 ;;
      403) echo "ヒント: Bot に applications.commands スコープが付与されていません。" >&2 ;;
      404) echo "ヒント: Application ID が誤っています（General Information > Application ID）。" >&2 ;;
      429) echo "ヒント: レート制限です。しばらく待って再実行してください。" >&2 ;;
    esac
    rm -f "$out"
    return 1
  fi
  cat "$out"
  rm -f "$out"
}

# 登録済みコマンド名を出す。jq が無い環境でも動くようにフォールバックする。
print_command_names() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.[].name' 2>/dev/null | sed 's/^/  \//'
  else
    grep -oE '"name":"[^"]+"' | sed 's/"name":"/  \//; s/"$//' | sort -u
  fi
}

if [ "${1:-}" = "--list" ]; then
  echo "登録済みのコマンド:"
  call GET | print_command_names
  exit 0
fi

COMMANDS='[
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
    "dm_permission": false,
    "default_member_permissions": "0",
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

call PUT "$COMMANDS" > /dev/null
echo "登録しました。現在このアプリケーションに登録されているコマンド:"
call GET | print_command_names

cat <<'NOTE'

反映まで数分かかる場合があります。Discordに出てこない場合は次を確認してください。

1. /prod だけ出ない場合（最も多い）
   /prod は default_member_permissions="0" で登録される。これは
   「既定では誰も実行できない」という意味で、事故防止のための設定。
   Discordの サーバー設定 > 連携サービス > 該当アプリ > /prod から
   実行を許可するロール/メンバーを追加するまで、誰の一覧にも出ない。
   /prod-uptime と /prod-uptime-list には制限が無いので、
   その2つが出て /prod だけ出ないならこれが原因。

2. どのコマンドも出ない場合
   OAuth2 > URL Generator の scopes に applications.commands が必要。
   これが無いと登録は成功してもサーバーに出ない。
   なお bot スコープは不要（Interactions Endpoint 方式のため、
   Botがサーバーのメンバーになる必要はない）。

3. Interactions Endpoint URL が設定されているか
   General Information > Interactions Endpoint URL に
   https://api-stg.shukatsu-ai.jp/api/discord/interactions を設定して保存する。
   保存時にDiscordがPINGを送り、応答できないと保存自体が失敗する。
NOTE
