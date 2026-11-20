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

# 理由の無い例外は設定エラー（理由と期限を付けた例外だけを許す）
mkdir -p "$WORK/clean-src"
echo 'import chromadb' > "$WORK/clean-src/app.py"
for reason in '""' '"  "' 'null'; do
  cat > "$WORK/allow-noreason.json" <<EOF
[{"id": "PYSEC-1", "package": "chromadb", "reason": $reason, "expires": "2026-12-31", "forbidden_patterns": ["chromadb_app_unused"]}]
EOF
  out=$(python3 "$FILTER" --report "$WORK/listed.json" --allowlist "$WORK/allow-noreason.json" --source-root "$WORK/clean-src" --today 2026-11-19 2>&1)
  if [ $? -ne 1 ] || ! grep -qF "reason" <<< "$out"; then
    echo "FAIL 理由が空の例外は設定エラーにする[$reason]"; echo "$out" | sed 's/^/    /'; fail=$((fail + 1))
  else
    echo "ok   理由が空の例外は設定エラーにする[$reason]"
  fi
done

# 期限が30日以内なら警告を出す（通るが、ログで気づける）
out=$(python3 "$FILTER" --report "$WORK/listed.json" --allowlist "$WORK/allow.json" --source-root "$WORK/clean-src" --today 2026-12-10 2>&1)
if [ $? -eq 0 ] && grep -qF "::warning::" <<< "$out"; then echo "ok   期限が近い例外は警告を出す"; else echo "FAIL 期限が近い例外は警告を出す"; echo "$out" | sed 's/^/    /'; fail=$((fail + 1)); fi
out=$(python3 "$FILTER" --report "$WORK/listed.json" --allowlist "$WORK/allow.json" --source-root "$WORK/clean-src" --today 2026-11-01 2>&1)
if [ $? -eq 0 ] && ! grep -qF "::warning::" <<< "$out"; then echo "ok   期限まで余裕があれば警告しない"; else echo "FAIL 期限まで余裕があれば警告しない"; echo "$out" | sed 's/^/    /'; fail=$((fail + 1)); fi

# 本物の例外リストの禁止パターンが、サーバーを動かす書き方を拾うこと
REAL_ALLOW="$ROOT/.github/pip-audit-allowlist.json"
cat > "$WORK/real.json" <<'EOF'
{"dependencies": [{"name": "chromadb", "version": "0.6.3", "vulns": [
  {"id": "PYSEC-2026-3813", "fix_versions": [], "aliases": []},
  {"id": "PYSEC-2026-3814", "fix_versions": [], "aliases": []},
  {"id": "PYSEC-2026-3815", "fix_versions": [], "aliases": []}]}], "fixes": []}
EOF
real_check() {
  local name="$1" file="$2" content="$3" out rc
  rm -rf "$WORK/real-src"; mkdir -p "$WORK/real-src"
  printf '%s\n' "$content" > "$WORK/real-src/$file"
  out=$(python3 "$FILTER" --report "$WORK/real.json" --allowlist "$REAL_ALLOW" --source-root "$WORK/real-src" --today 2026-11-19 2>&1)
  rc=$?
  if [ "$rc" -ne 1 ] || ! grep -qF "例外の前提である使い方" <<< "$out"; then
    echo "FAIL 禁止パターン: $name"; echo "$out" | sed 's/^/    /'; fail=$((fail + 1))
  else
    echo "ok   禁止パターン: $name"
  fi
}
real_check "Dockerfile の exec 形式の chroma run" Dockerfile 'CMD ["chroma", "run", "--path", "/data"]'
real_check "シェルの chroma run" start.sh 'exec chroma run --host 0.0.0.0'
real_check "from chromadb import app" server.py 'from chromadb import app'
real_check "from chromadb.app import" server.py 'from chromadb.app import app'
real_check "大文字の環境変数の認可プロバイダ" settings.toml 'CHROMA_SERVER_AUTHZ_PROVIDER = "x"'
real_check "trust_remote_code" embed.py 'fn = Embedder(trust_remote_code=True)'

if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
