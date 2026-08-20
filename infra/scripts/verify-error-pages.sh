#!/usr/bin/env bash
# ponytail: 静的エラーページに OGP 必須タグがあるかの最小チェック
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
fail=0

check() {
  local file="$1"
  local want_title="$2"
  if ! grep -q 'property="og:title"' "$file"; then
    echo "FAIL: missing og:title in $file"
    fail=1
    return
  fi
  if ! grep -q "$want_title" "$file"; then
    echo "FAIL: expected og title fragment '$want_title' in $file"
    fail=1
    return
  fi
  if ! grep -q 'property="og:description"' "$file"; then
    echo "FAIL: missing og:description in $file"
    fail=1
    return
  fi
  echo "OK: $file"
}

check "$ROOT/static/service-unavailable.html" "接続できません"
check "$ROOT/static/service-starting.html" "起動中"

exit "$fail"
