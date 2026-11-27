#!/bin/bash
# サードパーティの GitHub Actions がタグではなく commit SHA に固定されていることを検査する。
#
# actions/ は GitHub 自身のアクション。それ以外（appleboy の SSH、bicstone の
# Backlog 通知、aws-actions など）はタグを付け替えられると、鍵やトークンを扱う
# ステップが差し替わる（SOCAIAGENT-579）。
#
# 使い方: ./automation/test/third-party-action-pin-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fail=0
found=0

while IFS= read -r line; do
  file=${line%%:*}
  rest=${line#*:}
  rest=${rest#*:}
  ref=$(printf '%s\n' "$rest" | sed -E 's/.*uses:[[:space:]]*([^[:space:]]+).*/\1/')
  owner=${ref%%/*}
  sha=${ref##*@}
  sha=${sha%%#*}
  found=$((found + 1))
  if [ "$owner" = "actions" ]; then
    continue
  fi
  if printf '%s' "$sha" | grep -qE '^[0-9a-f]{40}$'; then
    echo "ok   $file $ref"
    continue
  fi
  echo "FAIL $file の $ref はタグ参照です。commit SHA に固定してください" >&2
  fail=$((fail + 1))
done < <(grep -RInE --include='*.yml' 'uses:[[:space:]]*[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@' "$ROOT/.github/workflows" || true)

if [ "$found" -eq 0 ]; then
  echo "FAIL uses: の抽出が0件です。検査条件が壊れています" >&2
  exit 1
fi
if [ "$fail" -gt 0 ]; then
  echo "FAIL ($fail 件)" >&2
  exit 1
fi
echo "PASS (サードパーティ Actions は SHA 固定)"
