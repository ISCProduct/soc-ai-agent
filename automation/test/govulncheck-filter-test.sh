#!/bin/bash
# govulncheck の例外判定（.github/scripts/govulncheck_filter.py）を検査する。
#
# 例外が広がりすぎると、本物の脆弱性を見逃したまま CI が通る。
# 「例外が効かなくなる条件」を1つずつ固定する。
#
# 使い方: ./automation/test/govulncheck-filter-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FILTER="$ROOT/.github/scripts/govulncheck_filter.py"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/src"
cat > "$WORK/src/app.go" <<'EOF'
package app

// c.RealIP() はコメントなので検出しない
func handler() { doWork() }
EOF
cat > "$WORK/src/app_test.go" <<'EOF'
package app

func helper() { c.RealIP() }
EOF

cat > "$WORK/allow.json" <<'EOF'
[{"id": "GO-1", "module": "example.com/echo", "reason": "test", "expires": "2026-12-31", "forbidden_patterns": ["\\.RealIP\\("]}]
EOF

# $1: OSV ID, $2: 修正版（空なら無し）, $3: モジュール
report() {
  local fixed=""
  [ -n "$2" ] && fixed="\"fixed_version\": \"$2\","
  cat <<EOF
{"config": {"protocol_version": "v1.0.0"}}
{"finding": {"osv": "$1", $fixed "trace": [{"module": "$3", "package": "$3/x", "function": "F"}]}}
{"finding": {"osv": "$1", "trace": [{"module": "$3"}]}}
EOF
}

fail=0
# $1: 名前, $2: 期待する終了コード, $3: 出力に含まれる文字列, $4: レポート, $5: 日付
check() {
  local out rc
  out=$(python3 "$FILTER" --report "$4" --allowlist "$WORK/allow.json" --source-root "$WORK/src" --today "${5:-2026-11-06}" 2>&1)
  rc=$?
  if [ "$rc" -ne "$2" ] || ! grep -qF "$3" <<< "$out"; then
    echo "FAIL $1: rc=$rc（期待 $2）、「$3」を含むはず"
    echo "$out" | sed 's/^/    /'
    fail=$((fail + 1))
  else
    echo "ok   $1"
  fi
}

report GO-1 "" example.com/echo > "$WORK/listed.json"
check "例外に載っていれば通る" 0 "例外を適用: GO-1" "$WORK/listed.json"
check "期限の当日は通る" 0 "PASSED" "$WORK/listed.json" 2026-12-31
check "期限を過ぎたら落ちる" 1 "期限" "$WORK/listed.json" 2027-01-01

report GO-2 "" example.com/echo > "$WORK/other.json"
check "例外にない脆弱性は落ちる" 1 "例外リストにありません" "$WORK/other.json"

report GO-1 v1.2.3 example.com/echo > "$WORK/fixed.json"
check "修正版が出たら落ちる" 1 "修正版" "$WORK/fixed.json"

report GO-1 "" example.com/other > "$WORK/module.json"
check "別のモジュールなら落ちる" 1 "対象外のモジュール" "$WORK/module.json"

cat > "$WORK/empty.json" <<'EOF'
{"config": {"protocol_version": "v1.0.0"}}
EOF
check "検出なしなら通り、古い例外を知らせる" 0 "検出されなくなりました" "$WORK/empty.json"

: > "$WORK/blank.json"
check "空の出力を脆弱性なしと読まない" 1 "読めません" "$WORK/blank.json"

cat > "$WORK/module-only.json" <<'EOF'
{"config": {"protocol_version": "v1.0.0"}}
{"finding": {"osv": "GO-9", "trace": [{"module": "example.com/unused"}]}}
EOF
check "コードから到達しないものは数えない" 0 "PASSED" "$WORK/module-only.json"

cat >> "$WORK/src/app.go" <<'EOF'

func ip(c ctx) string { return c.RealIP() }
EOF
check "前提の機能を使い始めたら落ちる" 1 "例外の前提である機能" "$WORK/listed.json"

if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
