#!/bin/bash
# pip-audit の例外判定（.github/scripts/pip_audit_filter.py）を検査する（SOCAIAGENT-578）。
#
# 例外が広がりすぎると、本物の脆弱性を見逃したまま CI が通る。
# 「例外が効かなくなる条件」を1つずつ固定する（govulncheck-filter-test.sh と同じ観点）。
#
# 使い方: ./automation/test/pip-audit-filter-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FILTER="$ROOT/.github/scripts/pip_audit_filter.py"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/src/tests"
cat > "$WORK/src/app.py" <<'EOF'
import chromadb

# chromadb.app はコメントなので検出しない
client = chromadb.HttpClient(host="chroma", port=8000)
EOF
cat > "$WORK/src/tests/test_app.py" <<'EOF'
import chromadb.app  # テストは見ない
EOF

cat > "$WORK/allow.json" <<'EOF'
[{"id": "PYSEC-1", "package": "chromadb", "reason": "test", "expires": "2026-12-31", "forbidden_patterns": ["chromadb\\.app"]}]
EOF

# $1: 脆弱性 ID, $2: 修正版（空なら無し）, $3: パッケージ
report() {
  local fixed="[]"
  [ -n "$2" ] && fixed="[\"$2\"]"
  cat <<EOF
{"dependencies": [
  {"name": "$3", "version": "0.6.3", "vulns": [{"id": "$1", "fix_versions": $fixed, "aliases": []}]},
  {"name": "fastapi", "version": "0.115.0", "vulns": []}
], "fixes": []}
EOF
}

fail=0
# $1: 名前, $2: 期待する終了コード, $3: 出力に含まれる文字列, $4: レポート, $5: 日付
check() {
  local out rc
  out=$(python3 "$FILTER" --report "$4" --allowlist "$WORK/allow.json" --source-root "$WORK/src" --today "${5:-2026-11-19}" 2>&1)
  rc=$?
  if [ "$rc" -ne "$2" ] || ! grep -qF "$3" <<< "$out"; then
    echo "FAIL ${1}: rc=${rc}（期待 ${2}）、「${3}」を含むはず"
    echo "$out" | sed 's/^/    /'
    fail=$((fail + 1))
  else
    echo "ok   $1"
  fi
}

report PYSEC-1 "" chromadb > "$WORK/listed.json"
check "例外に載っていれば通る" 0 "例外を適用: PYSEC-1" "$WORK/listed.json"
check "期限の当日は通る" 0 "PASSED" "$WORK/listed.json" 2026-12-31
check "期限を過ぎたら落ちる" 1 "期限" "$WORK/listed.json" 2027-01-01

report PYSEC-2 "" chromadb > "$WORK/other.json"
check "例外にない脆弱性は落ちる" 1 "例外リストにありません" "$WORK/other.json"

report PYSEC-1 0.7.0 chromadb > "$WORK/fixed.json"
check "修正版が出たら落ちる" 1 "修正版" "$WORK/fixed.json"

report PYSEC-1 "" ChromaDB > "$WORK/case.json"
check "パッケージ名の大文字小文字は区別しない" 0 "PASSED" "$WORK/case.json"

report PYSEC-1 "" requests > "$WORK/package.json"
check "別のパッケージなら落ちる" 1 "対象外のパッケージ" "$WORK/package.json"

cat > "$WORK/clean.json" <<'EOF'
{"dependencies": [{"name": "fastapi", "version": "0.115.0", "vulns": []}], "fixes": []}
EOF
check "検出なしなら通り、古い例外を知らせる" 0 "検出されなくなりました" "$WORK/clean.json"

: > "$WORK/blank.json"
check "空の出力を脆弱性なしと読まない" 1 "読めません" "$WORK/blank.json"
echo '{"dependencies": [], "fixes": []}' > "$WORK/nodeps.json"
check "依存が0件の出力も読めない扱いにする" 1 "読めません" "$WORK/nodeps.json"

cp "$WORK/allow.json" "$WORK/allow.bak"
for entry_patterns in '' ', "forbidden_patterns": []'; do
  cat > "$WORK/allow.json" <<EOF
[{"id": "PYSEC-1", "package": "chromadb", "reason": "test", "expires": "2026-12-31"$entry_patterns}]
EOF
  check "禁止パターンが無い例外は設定エラーにする[$entry_patterns]" 1 "設定エラー" "$WORK/listed.json"
done
mv "$WORK/allow.bak" "$WORK/allow.json"

cat >> "$WORK/src/app.py" <<'EOF'
from chromadb.app import app
EOF
check "前提の使い方を始めたら落ちる" 1 "例外の前提である使い方" "$WORK/listed.json"

if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
