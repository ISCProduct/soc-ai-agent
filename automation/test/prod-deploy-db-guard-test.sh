#!/bin/bash
# 本番デプロイのマイグレーションがRDS停止中でも通ることを、ワークフロー定義の構造で固定する。
#
# 本番は「指定日のみ終日起動」ポリシー(prod-uptime-scheduler)のため、通常日はRDSが停止している。
# ガードが無いとマイグレーションタスクがDBへ到達できず、mainへのマージが必ず失敗する
# （実際に #1488 のデプロイが "no route to host" で落ちた 2026-09-23）。
#
# 使い方: ./automation/test/prod-deploy-db-guard-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows/deployment.yml"

fail=0
line_of() { grep -n "$1" "$WF" | head -1 | cut -d: -f1; }

START=$(line_of "Start production RDS if stopped")
MIGRATE=$(line_of "Run DB migration (production)")
STOP=$(line_of "Stop production RDS if this deploy started it")

if [ -z "$START" ]; then
  echo "FAIL 本番RDSの起動ステップが無い（停止中はマイグレーションが必ず失敗する）"
  fail=$((fail + 1))
elif [ -z "$MIGRATE" ] || [ "$START" -ge "$MIGRATE" ]; then
  echo "FAIL 本番RDSの起動はマイグレーションより前に置くこと（start=$START migrate=${MIGRATE}）"
  fail=$((fail + 1))
else
  echo "ok   本番RDSの起動がマイグレーションより前にある（$START < ${MIGRATE}）"
fi

if [ -z "$STOP" ]; then
  echo "FAIL 起動したRDSを停止へ戻すステップが無い（通常日に課金が残る）"
  fail=$((fail + 1))
elif [ -n "$MIGRATE" ] && [ "$STOP" -le "$MIGRATE" ]; then
  echo "FAIL RDSの停止はマイグレーションより後に置くこと（stop=$STOP migrate=${MIGRATE}）"
  fail=$((fail + 1))
else
  echo "ok   起動したRDSを停止へ戻すステップがある（${STOP}）"
  # 稼働中の本番を止めないためのガード。条件が消えると展示会当日にDBを落とす。
  if ! sed -n "${STOP},$((STOP + 12))p" "$WF" | grep -q "desiredCount"; then
    echo "FAIL 停止ステップに desiredCount の確認が無い（稼働中の本番を止めうる）"
    fail=$((fail + 1))
  fi
  if ! sed -n "${STOP},$((STOP + 3))p" "$WF" | grep -q "started_by_deploy == 'true'"; then
    echo "FAIL 停止ステップが started_by_deploy を見ていない（他者が起動したDBを止めうる）"
    fail=$((fail + 1))
  fi
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS"
