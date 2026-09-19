#!/bin/bash
# automation/ops/notify-discord.sh の Webhook 解決と、未設定時に沈黙しないことを検証する。
#
# 「通知が飛ばない」は障害時にしか露見せず、その時には手遅れになる。
# 実際 DISCORD_DEPLOY_WEBHOOK_URL は未設定のままスモーク通知が沈黙していた(#1354)。
#
# 使い方: ./automation/test/ops-notify-discord-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TARGET="$ROOT/automation/ops/notify-discord.sh"

pass=0 fail=0
# ペイロードは json.dumps 既定の \uXXXX エスケープで出る(既存の deploy-smoke 通知と同じ)。
# エンコード形式ではなく中身を見たいので、デコードしてから比較する。
decode() {
  python3 -c "import json,sys; print(json.loads(sys.stdin.read()).get('content',''))" 2>/dev/null
}

check() {
  local name="$1" expect="$2" actual="$3"
  if [[ "$actual" == *"$expect"* ]]; then
    echo "ok   $name"; pass=$((pass + 1))
  else
    echo "FAIL $name"; echo "  期待: $expect"; echo "  実際: $actual"; fail=$((fail + 1))
  fi
}

# OPS を優先して使う
out=$(DRY_RUN=1 DISCORD_OPS_WEBHOOK_URL="https://example.invalid/ops" \
  DISCORD_RELEASE_WEBHOOK_URL="https://example.invalid/release" \
  bash "$TARGET" "起動ジョブが失敗しました" 2>&1)
check "本文がJSONに入る" '"content"' "$out"
check "本文がそのまま載る" "起動ジョブが失敗しました" "$(echo "$out" | decode)"

# OPS 未設定なら RELEASE へフォールバックする（今日から通知が届く状態にするため）
out=$(DRY_RUN=1 DISCORD_RELEASE_WEBHOOK_URL="https://example.invalid/release" \
  bash "$TARGET" "フォールバック" 2>&1)
rc=$?
check "RELEASEへフォールバックして成功する" "フォールバック" "$(echo "$out" | decode)"
check "フォールバック時の終了コードは0" "0" "$rc"

# どちらも無い場合は黙ってスキップせず、失敗として扱う
out=$(env -u DISCORD_OPS_WEBHOOK_URL -u DISCORD_RELEASE_WEBHOOK_URL \
  bash "$TARGET" "宛先なし" 2>&1)
rc=$?
check "Webhook未設定はエラー出力する" "::error::" "$out"
check "通知できなかった内容をログに残す" "宛先なし" "$out"
check "Webhook未設定は終了コード1" "1" "$rc"

# 本文が空なら投稿しない
out=$(DRY_RUN=1 DISCORD_OPS_WEBHOOK_URL="https://example.invalid/ops" bash "$TARGET" "" 2>&1)
rc=$?
check "空の本文はエラー" "通知本文が空です" "$out"
check "空の本文は終了コード2" "2" "$rc"

# schedule で動くジョブから通知ステップが外れても、誰も気づけない。静的に固定する。
for wf in prod-uptime-scheduler staging-uptime-scheduler; do
  f="$ROOT/.github/workflows/$wf.yml"
  if grep -q "if: failure()" "$f" && grep -q "./automation/ops/notify-discord.sh" "$f"; then
    echo "ok   $wf.yml に失敗通知ステップがある"; pass=$((pass + 1))
  else
    echo "FAIL $wf.yml の失敗通知ステップが無くなっています"; fail=$((fail + 1))
  fi
done

echo "----"
echo "pass=$pass fail=$fail"
[ "$fail" -eq 0 ] || exit 1
