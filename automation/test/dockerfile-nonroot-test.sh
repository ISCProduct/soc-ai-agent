#!/bin/bash
# 全 Dockerfile の最終ステージに非rootの USER があることを検査する。
#
# #1470 で Backend だけ非root化したが、frontend / rag / rag(finetune) /
# company-graph は root のまま残っていた（#1477）。コンテナ内でのRCEや
# パストラバーサルが起きたとき、rootのままだとホスト側の攻撃面が広がる。
# 本番は ECS on Fargate で動くため、タスク定義に user 指定が無い限り
# イメージの USER がそのまま実行ユーザーになる。
#
# 最終ステージだけを見るのは、builder ステージの USER は実行時に効かないため。
#
# 使い方: ./automation/test/dockerfile-nonroot-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

fail=0
checked=0

while IFS= read -r f; do
  name="${f#"$ROOT"/}"
  checked=$((checked + 1))

  # 最終ステージ = 最後の FROM 以降。builder の USER を数えないための切り出し。
  last_from=$(grep -n -iE '^[[:space:]]*FROM[[:space:]]' "$f" | tail -1 | cut -d: -f1)
  if [ -z "$last_from" ]; then
    echo "FAIL $name に FROM がありません（抽出条件が壊れています）"
    fail=$((fail + 1))
    continue
  fi
  stage=$(tail -n "+$last_from" "$f")

  user=$(grep -iE '^[[:space:]]*USER[[:space:]]' <<< "$stage" | tail -1 | awk '{print $2}')
  if [ -z "$user" ]; then
    echo "FAIL $name の最終ステージに USER がありません（root で動きます）"
    echo "     → 非rootユーザーを作って USER を指定してください（Backend/Dockerfile が見本）"
    fail=$((fail + 1))
    continue
  fi

  # "app:app" 形式でも uid/uname 側だけを見る
  uid="${user%%:*}"
  case "$uid" in
    root | 0)
      echo "FAIL $name の USER が root です（${user}）"
      fail=$((fail + 1))
      ;;
    *)
      echo "ok   $name (USER $user)"
      ;;
  esac
done < <(find "$ROOT" -name 'Dockerfile*' -type f \
           -not -path '*/node_modules/*' -not -path '*/.git/*' | sort)

# 抽出条件が壊れると「検査したつもり」で素通りする。
# Backend / frontend / rag / rag(finetune) / company-graph の5つが最低ライン。
MIN_DOCKERFILES=5
if [ "$checked" -lt "$MIN_DOCKERFILES" ]; then
  echo "NG: Dockerfile が $checked 件しか見つかりません（期待 $MIN_DOCKERFILES 件以上）。検索条件が壊れています。" >&2
  exit 1
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS ($checked Dockerfile を検査)"
