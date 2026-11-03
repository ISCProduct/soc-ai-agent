#!/bin/bash
# automation/test 配下のスクリプトが、どれかのワークフローから呼ばれていることを検査する。
#
# test.yml にはステップを1本ずつ手で並べている。新しい検査スクリプトを足して
# 登録を忘れると、CI で一度も走らないまま「固めたつもり」になる。
#
# 使い方: ./automation/test/test-registration-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT" || exit 1

# コメント行を落とす。説明コメントに名前が出るだけで「登録済み」と判定しないため。
workflow_code=$(cat .github/workflows/*.yml | grep -vE '^[[:space:]]*#')

fail=0
checked=0
for f in automation/test/*.sh automation/test/*.py; do
  [ -e "$f" ] || continue
  checked=$((checked + 1))
  name=$(basename "$f")
  if ! grep -qF "$name" <<< "$workflow_code"; then
    echo "FAIL $name がどのワークフローからも呼ばれていない（test.yml にステップを足すこと）"
    fail=$((fail + 1))
  fi
done

if [ "$checked" -eq 0 ]; then
  echo "FAIL automation/test にスクリプトが1つも無い（検査条件が壊れている）"
  exit 1
fi
echo "検査したスクリプト: $checked 件"
if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
