#!/bin/bash
# 「同一 apply 内で作られるシークレットの ARN」を count の判定に使っていないことを固定する。
#
# ECSサービスのモジュールは secret_arns を読む IAM ポリシーを
# `count = length(var.secret_arns) > 0 ? 1 : 0` で作るものがある。
# secret_arns が compact() を通っていて、まだ state に無いリソースの ARN を含むと、
# compact() は「その要素を落とすかどうか」が決まらずリスト長ごと unknown になる。
# Terraform は unknown な count を許さないので plan が `Invalid count argument` で止まる。
# シークレットを1つ足した apply で初めて起きるため、気づくのは本番の apply 当日になる。
#
# modules/ecs_service には create_secrets_policy で件数判定を呼び出し側へ出す前例がある
# (modules/ecs_service/variables.tf のコメントが理由も書いている)。
# 同じ構図のモジュールと呼び出し側がその前例に従っていることを見る。
#
# 使い方: ./automation/test/ecs-secrets-policy-count-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

fail=0
checked=0

for tf in "$ROOT"/infra/terraform/environments/*/main.tf; do
  [ -f "$tf" ] || continue
  envname=$(basename "$(dirname "$tf")")

  # module ブロックごとに「名前 / source / secret_arns の式 / create_secrets_policy の有無」を出す。
  while IFS=$'\t' read -r name source arns_expr has_flag; do
    case "$source" in
      *ecs_service*) ;;
      *) continue ;;
    esac
    [ -n "$arns_expr" ] || continue

    # compact() が直接見えるか、local 経由で見えるかを判定する。
    needs_flag=0
    if [[ "$arns_expr" == *"compact("* ]]; then
      needs_flag=1
    elif [[ "$arns_expr" =~ local\.([a-z0-9_]+) ]]; then
      localname="${BASH_REMATCH[1]}"
      if grep -qE "^[[:space:]]*${localname}[[:space:]]*=[[:space:]]*compact\(" "$tf"; then
        needs_flag=1
      fi
    fi
    [ "$needs_flag" = "1" ] || continue

    checked=$((checked + 1))
    if [ "$has_flag" = "1" ]; then
      echo "ok   $envname module.$name: create_secrets_policy を明示している"
    else
      echo "FAIL $envname module.$name: secret_arns が compact() を通るのに create_secrets_policy を渡していない"
      echo "     (未作成シークレットのARNが混ざると count が unknown になり plan が Invalid count argument で止まる)"
      fail=$((fail + 1))
    fi
  done < <(awk '
    /^module "/ { name = $2; gsub(/"/, "", name); src = ""; arns = ""; flag = 0; inside = 1; next }
    inside && /^}/ {
      if (src != "") printf "%s\t%s\t%s\t%d\n", name, src, arns, flag
      inside = 0; next
    }
    inside {
      if ($0 ~ /source[[:space:]]*=/) src = $0
      if ($0 ~ /secret_arns[[:space:]]*=/) arns = $0
      if ($0 ~ /create_secrets_policy[[:space:]]*=/) flag = 1
    }
  ' "$tf")
done

if [ "$checked" -eq 0 ]; then
  echo "FAIL compact() を通る secret_arns の呼び出しが1件も見つからない（抽出条件が壊れている）"
  fail=$((fail + 1))
fi

# モジュール側に逃げ道（create_secrets_policy）があること。
for mod in "$ROOT"/infra/terraform/modules/ecs_service "$ROOT"/infra/terraform/modules/ecs_service_fargate; do
  [ -d "$mod" ] || continue
  if ! grep -q 'variable "create_secrets_policy"' "$mod/variables.tf"; then
    echo "FAIL $(basename "$mod"): create_secrets_policy が無い（呼び出し側が件数判定を明示できない）"
    fail=$((fail + 1))
  elif ! grep -q "create_secrets_policy" "$mod/main.tf"; then
    echo "FAIL $(basename "$mod"): create_secrets_policy を count の判定に使っていない"
    fail=$((fail + 1))
  else
    echo "ok   $(basename "$mod"): create_secrets_policy で count を判定できる"
  fi
done

echo "検査した compact() 付き secret_arns: $checked 件"
if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
