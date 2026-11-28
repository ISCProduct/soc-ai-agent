#!/bin/bash
# develop が進んだあとに、開いている PR の版番号を再検査する仕組みが残っていること。
#
# 検査本体（migration-version-unique-test.sh）は PR 更新時にしか走らない。
# 別 PR が同じ番号を先にマージすると、残った PR は緑のままになる（SOCAIAGENT-503）。
# 再検査ワークフローと、衝突メッセージの自己チェックをここで固定する。
#
# 使い方: ./automation/test/migration-vs-develop-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT" || exit 1
fail=0

WF=".github/workflows/migration-vs-develop.yml"
if [ ! -f "$WF" ]; then
  echo "FAIL $WF が無い" >&2
  exit 1
fi

# コメントを除いた本体だけで、起動条件と権限を見る。
code=$(grep -vE '^[[:space:]]*#' "$WF")
if ! grep -q 'develop' <<< "$code"; then
  echo "FAIL develop への push で起動する指定が無い" >&2
  fail=$((fail + 1))
fi
if ! grep -q 'checks: write' <<< "$code"; then
  echo "FAIL PR の head に check を付ける checks: write が無い" >&2
  fail=$((fail + 1))
fi
if grep -q 'contents: write' <<< "$code"; then
  echo "FAIL contents: write がある。検査は読み取りと check の作成だけで足りる" >&2
  fail=$((fail + 1))
fi
if ! grep -q 'migration_vs_develop.py' <<< "$code"; then
  echo "FAIL 再検査スクリプトを呼んでいない" >&2
  fail=$((fail + 1))
fi

if ! python3 .github/scripts/migration_vs_develop.py --self-check; then
  echo "FAIL 再検査スクリプトの自己チェック" >&2
  fail=$((fail + 1))
fi

# 既存の検査が、別名の同一版番号をファイル名つきで落とすこと。
# ワークフローはこの出力を PR の check に載せる。
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
git -C "$fixture" init -q -b base
mkdir -p "$fixture/Backend/migrations"
printf 'SELECT 1;\n' > "$fixture/Backend/migrations/20260101000000_alpha.up.sql"
printf 'SELECT 1;\n' > "$fixture/Backend/migrations/20260101000000_alpha.down.sql"
git -C "$fixture" add Backend/migrations
# 検査用の一時リポジトリなので、実行環境の identity には依存させない。
GIT_AUTHOR_NAME=migration-test GIT_AUTHOR_EMAIL=migration-test@example.com \
  GIT_COMMITTER_NAME=migration-test GIT_COMMITTER_EMAIL=migration-test@example.com \
  git -C "$fixture" commit -q -m base
git -C "$fixture" checkout -q -b pr
rm "$fixture/Backend/migrations/20260101000000_alpha.up.sql" "$fixture/Backend/migrations/20260101000000_alpha.down.sql"
printf 'SELECT 1;\n' > "$fixture/Backend/migrations/20260101000000_beta.up.sql"
printf 'SELECT 1;\n' > "$fixture/Backend/migrations/20260101000000_beta.down.sql"
git -C "$fixture" add -A
GIT_AUTHOR_NAME=migration-test GIT_AUTHOR_EMAIL=migration-test@example.com \
  GIT_COMMITTER_NAME=migration-test GIT_COMMITTER_EMAIL=migration-test@example.com \
  git -C "$fixture" commit -q -m pr

set +e
collision=$(cd "$fixture" && BASE_REF=base "$ROOT/automation/test/migration-version-unique-test.sh" Backend/migrations 2>&1)
collision_code=$?
set -e
if [ "$collision_code" -eq 0 ]; then
  echo "FAIL 別名の同一版番号を見落とした" >&2
  fail=$((fail + 1))
fi
if ! grep -q '20260101000000_alpha' <<< "$collision" || ! grep -q '20260101000000_beta' <<< "$collision"; then
  echo "FAIL 衝突メッセージに両方のファイル名が無い" >&2
  echo "$collision" >&2
  fail=$((fail + 1))
else
  echo "ok   衝突時は両方のファイル名が出る"
fi

if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)" >&2
  exit 1
fi
echo "PASSED"
