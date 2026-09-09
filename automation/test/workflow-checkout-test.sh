#!/bin/bash
# リポジトリのファイルを読むワークフローに actions/checkout があることを検査する。
#
# backlog-deleted-to-github-issue.yml が checkout を置かないまま
# .github/scripts/backlog_client.py を import しており、毎時の実行が
# ModuleNotFoundError で失敗し続けていた（2026-09-08〜09-09 で直近5回すべて失敗）。
#
# schedule で動くワークフローは誰も結果を見ないため、壊れても気づけない。
# 静的に検出する。
#
# 使い方: ./automation/test/workflow-checkout-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF_DIR="$ROOT/.github/workflows"

fail=0
checked=0
bl=0
au=0

for f in "$WF_DIR"/*.yml "$WF_DIR"/*.yaml; do
  [ -f "$f" ] || continue
  name=$(basename "$f")

  # リポジトリのファイルに依存する記述
  needs_repo=""
  grep -q "sys.path.insert(0, \".github/scripts\")" "$f" && needs_repo="${needs_repo}backlog_client "
  grep -qE "\./automation/(test|discord)/" "$f" && needs_repo="${needs_repo}automation/ "
  grep -qE "^\s*run:.*\./(scripts|automation)/" "$f" && needs_repo="${needs_repo}script "

  [ -z "$needs_repo" ] && continue
  checked=$((checked + 1))
  case "$needs_repo" in *backlog_client*) bl=$((bl + 1));; esac
  case "$needs_repo" in *automation/*) au=$((au + 1));; esac

  if grep -q "actions/checkout@" "$f"; then
    echo "ok   $name (依存: ${needs_repo%% })"
  else
    echo "FAIL $name はリポジトリのファイル（${needs_repo%% }）を読むのに actions/checkout がありません"
    fail=$((fail + 1))
  fi
done

# 抽出条件がひとつでも壊れると「検査したつもり」で素通りする。
# 依存の種類ごとに最低件数を固定して、条件の欠落を検出する。
# 対象を減らすときはこの数も一緒に下げること。
MIN_BACKLOG_CLIENT=5
MIN_AUTOMATION=1
if [ "$bl" -lt "$MIN_BACKLOG_CLIENT" ]; then
  echo "NG: backlog_client を読むワークフローが $bl 件しか見つかりません（期待 $MIN_BACKLOG_CLIENT 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi
if [ "$au" -lt "$MIN_AUTOMATION" ]; then
  echo "NG: automation/ を読むワークフローが $au 件しか見つかりません（期待 $MIN_AUTOMATION 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS ($checked ワークフローを検査: backlog_client=$bl automation=$au)"
