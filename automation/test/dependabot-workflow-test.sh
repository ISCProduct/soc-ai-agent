#!/bin/bash
# Dependabot のPRで必ず失敗するワークフローが無いことを検査する。
#
# GitHub の仕様として、Dependabot 起因の実行には通常の secrets が渡らない。
# secrets を必須とするワークフローが Dependabot のPRでも動くと、
# 「Input required and not supplied」「未設定または空です」で必ず失敗する。
#
# 実際 Backlog 系2つがこれで失敗し続けており、
# Actions の失敗一覧が Dependabot 由来のノイズで埋まっていた（2026-09-10）。
#
# 使い方: ./automation/test/dependabot-workflow-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF_DIR="$ROOT/.github/workflows"

fail=0
checked=0
secret_users=0

for f in "$WF_DIR"/*.yml "$WF_DIR"/*.yaml; do
  [ -f "$f" ] || continue
  name=$(basename "$f")

  # pull_request で動き、かつ secrets を使うワークフローが対象。
  grep -qE "^\s*pull_request(_target)?:" "$f" || continue
  grep -q 'secrets\.' "$f" || continue
  # GITHUB_TOKEN は Dependabot でも渡るので、それ以外の secrets を使うものだけ
  grep -qE 'secrets\.(?!GITHUB_TOKEN)' "$f" 2>/dev/null || \
    grep 'secrets\.' "$f" | grep -qv 'secrets\.GITHUB_TOKEN' || continue

  checked=$((checked + 1))
  secret_users=$((secret_users + 1))

  if grep -q "dependabot\[bot\]" "$f"; then
    echo "ok   $name (dependabot を除外している)"
  else
    echo "FAIL $name は secrets を使うのに dependabot[bot] を除外していません"
    echo "     → Dependabot のPRで必ず失敗します。jobs.<id>.if に"
    echo "       github.actor != 'dependabot[bot]' を足してください。"
    fail=$((fail + 1))
  fi
done

# GitHub Actions は PR を承認できない設定なので、approve を叩くと必ず失敗する
approve_hits=$(grep -rlE "^\s*(run:.*)?gh pr review .*--approve" "$WF_DIR" 2>/dev/null | wc -l | tr -d ' ')
if [ "$approve_hits" -gt 0 ]; then
  echo "FAIL gh pr review --approve を実行するワークフローがあります:"
  grep -rlE "gh pr review .*--approve" "$WF_DIR" 2>/dev/null | sed 's|.*/|     |'
  echo "     → 「Allow GitHub Actions to approve pull requests」が無効なため必ず失敗します。"
  fail=$((fail + 1))
fi

# 抽出条件が壊れると「検査したつもり」で素通りする
MIN_SECRET_USERS=2
if [ "$secret_users" -lt "$MIN_SECRET_USERS" ]; then
  echo "NG: secrets を使う pull_request ワークフローが $secret_users 件しか見つかりません（期待 $MIN_SECRET_USERS 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS ($checked ワークフローを検査)"
