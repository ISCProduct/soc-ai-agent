#!/bin/bash
# 反映後スモークが、対象環境のデプロイと同じ concurrency group にいることを検査する。
#
# 別 group だとスモークの実行中に次のデプロイが開始でき、その docker compose down
# で対象環境が落ちてスモークが全滅する。2026-09-19〜09-20 の Deploy 失敗のうち
# 3/4 件がこれで、いずれも変更内容とは無関係な PR だった（#1377 #1390 #1392）。
#
# フレークは「またか」で片付けられて調査されないため、group がずれたら
# 静的に落とす。
#
# 使い方: ./automation/test/deploy-smoke-concurrency-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SMOKE="$ROOT/.github/workflows/deploy-smoke.yml"
DEPLOY="$ROOT/.github/workflows/deployment.yml"

fail=0

# スモーク側: group が deploy-<environment> であること
smoke_group=$(grep -A2 '^concurrency:' "$SMOKE" | grep -E '^[[:space:]]+group:' | sed -E 's|^[[:space:]]*group:[[:space:]]*||')
if [ "$smoke_group" = 'deploy-${{ inputs.environment }}' ]; then
  echo "ok   deploy-smoke.yml の group: $smoke_group"
else
  echo "FAIL deploy-smoke.yml の group が 'deploy-\${{ inputs.environment }}' ではありません: '$smoke_group'" >&2
  fail=$((fail + 1))
fi

# スモークが実行中に割り込まれないよう、キャンセルさせない
smoke_cancel=$(grep -A3 '^concurrency:' "$SMOKE" | grep -E '^[[:space:]]+cancel-in-progress:' | sed -E 's|^[[:space:]]*cancel-in-progress:[[:space:]]*||')
if [ "$smoke_cancel" = "false" ]; then
  echo "ok   deploy-smoke.yml の cancel-in-progress: false"
else
  echo "FAIL deploy-smoke.yml の cancel-in-progress が false ではありません: '$smoke_cancel'" >&2
  fail=$((fail + 1))
fi

# デプロイ側: スモークと突き合わせる group が実在すること
found=0
for env in staging production; do
  if grep -qE "^[[:space:]]+group:[[:space:]]*deploy-$env[[:space:]]*$" "$DEPLOY"; then
    echo "ok   deployment.yml に group: deploy-$env がある"
    found=$((found + 1))
  else
    echo "FAIL deployment.yml に group: deploy-$env がありません（スモークと group が噛み合いません）" >&2
    fail=$((fail + 1))
  fi
done

if [ "$found" -ne 2 ]; then
  echo "NG: 突き合わせ対象の group が $found 件しか見つかりません（期待 2 件）。抽出条件が壊れています。" >&2
  exit 1
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS (スモークとデプロイの concurrency group が一致)"
