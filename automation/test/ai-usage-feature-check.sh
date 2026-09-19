#!/bin/bash
# AI を呼ぶ経路が機能名を付けずに記録されていないかを静的に検出する（#1294 / DesignDoc §6）。
#
# 機能名を付け忘れた経路は api_call_logs.feature='unknown' として記録される。
# 「計測されているつもりで、機能別に割れていない」状態は、集計を見るまで気づけない。
# #1193 で「Turn 経路が一切記録されていない」ことが後から発覚したのと同じ壊れ方なので、
# 追加時点で落とす。
#
# 判定: AI クライアント呼び出しの直前5行以内に usagectx.WithFeature があること。
# 除外: internal/openai（記録層そのもの）と internal/ai（ctx を素通しするアダプタ）。
#
# 使い方: ./automation/test/ai-usage-feature-check.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

hits=$(grep -rn -E "\.(ChatCompletionJSON|ResponsesWithTemperature|CreateEmbeddings?)\(" \
  "$ROOT/Backend/internal" "$ROOT/Backend/cmd" --include="*.go" \
  | grep -v "_test.go" \
  | grep -v "/internal/openai/" \
  | grep -v "/internal/ai/" || true)

total=$(printf '%s\n' "$hits" | grep -c . || true)

# 抽出条件が壊れると「検査したつもり」で素通りする。件数の下限を固定する。
MIN_CHECKED=15
if [ "$total" -lt "$MIN_CHECKED" ]; then
  echo "NG: AI 呼び出しが $total 件しか見つかりません（期待 $MIN_CHECKED 件以上）。抽出条件が壊れています。"
  exit 1
fi

fail=0
while IFS= read -r hit; do
  [ -z "$hit" ] && continue
  file="${hit%%:*}"
  rest="${hit#*:}"
  line="${rest%%:*}"

  from=$((line - 5))
  [ "$from" -lt 1 ] && from=1
  if ! sed -n "${from},${line}p" "$file" | grep -q "usagectx.WithFeature"; then
    echo "FAIL ${file#"$ROOT/"}:$line 機能名が付いていません（直前に usagectx.WithFeature を入れてください）"
    fail=$((fail + 1))
  fi
done <<< "$hits"

if [ "$fail" -gt 0 ]; then
  echo "----"
  echo "機能名の無い AI 呼び出しが $fail 件あります。api_call_logs.feature='unknown' で記録され、機能別に割れません。"
  exit 1
fi

echo "PASS (AI 呼び出し $total 件すべてに機能名が付いています)"
