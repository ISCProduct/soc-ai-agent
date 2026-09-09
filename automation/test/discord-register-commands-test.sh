#!/bin/bash
# register-commands.sh のエラー処理と出力を、curl をスタブして検証する。
#
# 実際の Discord API は叩かない。以前は curl -sf で投げていたため
# 失敗理由が一切表示されず、「登録されないが理由が分からない」状態だった。
# その退行を防ぐ。

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TARGET="$SCRIPT_DIR/automation/discord/register-commands.sh"
STUB_DIR=$(mktemp -d)
trap 'rm -rf "$STUB_DIR"' EXIT

pass=0 fail=0
check() {
  local name="$1" expect="$2" actual="$3"
  if [[ "$actual" == *"$expect"* ]]; then
    echo "ok   $name"; pass=$((pass + 1))
  else
    echo "FAIL $name"; echo "  期待: $expect"; echo "  実際: $actual"; fail=$((fail + 1))
  fi
}

# curl のスタブ。STUB_STATUS と STUB_BODY で応答を作る。
make_stub() {
  cat > "$STUB_DIR/curl" <<STUB
#!/bin/bash
out=""
prev=""
for a in "\$@"; do
  if [ "\$prev" = "-o" ]; then out="\$a"; fi
  prev="\$a"
done
[ -n "\$out" ] && printf '%s' "\${STUB_BODY:-}" > "\$out"
printf '%s' "\${STUB_STATUS:-200}"
STUB
  chmod +x "$STUB_DIR/curl"
}
make_stub
export PATH="$STUB_DIR:$PATH"

run() { DISCORD_BOT_TOKEN=dummy DISCORD_APPLICATION_ID=123 bash "$TARGET" "$@" 2>&1; }

# --- 認証情報が無ければ止まる ---
out=$(DISCORD_BOT_TOKEN= DISCORD_APPLICATION_ID= bash "$TARGET" 2>&1)
check "認証情報が無ければ案内して終了" "環境変数で指定してください" "$out"

# --- 失敗時に理由が見えること（これが元の問題） ---
export STUB_STATUS=401
export STUB_BODY='{"message":"401: Unauthorized","code":0}'
out=$(run)
check "401でHTTPステータスを表示" "HTTP 401" "$out"
check "401でDiscordの応答本文を表示" "Unauthorized" "$out"
check "401で原因のヒントを出す" "Bot Token" "$out"

export STUB_STATUS=404
export STUB_BODY='{"message":"404: Not Found"}'
out=$(run)
check "404でApplication IDを疑うヒント" "Application ID" "$out"

export STUB_STATUS=403
export STUB_BODY='{"message":"Missing Access"}'
out=$(run)
check "403でスコープ不足のヒント" "applications.commands" "$out"

# --- 失敗を成功と report しないこと ---
export STUB_STATUS=401
export STUB_BODY='{"message":"401: Unauthorized"}'
out=$(run)
check "失敗時に登録成功と言わない" "" "$(echo "$out" | grep -c '登録しました' | grep -q '^0$' && echo "")"

# --- 成功時は登録済みコマンド名を出すこと ---
export STUB_STATUS=200
export STUB_BODY='[{"name":"prod"},{"name":"prod-uptime"},{"name":"prod-uptime-list"}]'
out=$(run)
for c in prod prod-uptime prod-uptime-list; do
  check "成功時に /$c を表示" "/$c" "$out"
done
check "成功時に次に見る場所を案内" "Interactions Endpoint URL" "$out"

# --- --list は登録せず一覧だけ出すこと ---
out=$(run --list)
check "--list は一覧を出す" "登録済みのコマンド" "$out"
check "--list は登録しない" "" "$(echo "$out" | grep -c '登録しました' | grep -q '^0$' && echo "")"

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS ($pass ケース)"
