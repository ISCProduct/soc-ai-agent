#!/bin/bash
# staging の「SSM 経由で全台へ配るシークレット」が、稼働中インスタンスにも届くことを固定する。
#
# ASG は version=$Latest で Launch Template を更新しても既存インスタンスを置換しないため、
# app_user_data.sh.tftpl へ1行足しただけでは稼働中の /opt/app/.env に反映されない。
# そのために deployment.yml の「Deploy via SSH」には SSM から値を読んで .env を
# 書き換える継ぎ足し処理がある(#1407 で BFF_INTERNAL_TOKEN に導入)。
#
# 新しいシークレットを SSM パラメータで配るとき、この継ぎ足しを足し忘れると
# 「新規インスタンスだけ動く」状態になり、ASG が増えるまで誰も気づけない。
# aws_ssm_parameter を1つ足したら継ぎ足しも足す、という対応関係を機械的に見る。
#
# 使い方: ./automation/test/staging-env-topup-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows/deployment.yml"
TF="$ROOT/infra/terraform/environments/staging/main.tf"
TPL="$ROOT/infra/terraform/environments/staging/app_user_data.sh.tftpl"

fail=0
checked=0

# 「Deploy via SSH」ステップの本体だけを見る。他のジョブの SSM 参照を拾わないため。
SSH_STEP=$(grep -n 'name: Deploy via SSH' "$WF" | head -1 | cut -d: -f1)
if [ -z "$SSH_STEP" ]; then
  echo "FAIL deployment.yml に staging の「Deploy via SSH」ステップが無い"
  exit 1
fi
SSH_END=$(awk -v s="$SSH_STEP" 'NR > s && /^      - name: /{print NR; exit}' "$WF")
[ -n "$SSH_END" ] || SSH_END=$(wc -l < "$WF")
SSH_BODY=$(sed -n "${SSH_STEP},${SSH_END}p" "$WF")

# staging が作る SSM パラメータのリソース名を列挙する。
resources=$(grep -oE 'resource "aws_ssm_parameter" "[a-z0-9_]+"' "$TF" | sed -E 's/.*"([a-z0-9_]+)"$/\1/')
if [ -z "$resources" ]; then
  echo "FAIL staging に aws_ssm_parameter が1つも無い（検査条件が壊れている）"
  exit 1
fi

for res in $resources; do
  # name = "/${var.project_name}/<param>" からパラメータ名を取る。
  param=$(awk -v r="$res" '
    $0 ~ "resource \"aws_ssm_parameter\" \"" r "\"" { inside = 1 }
    inside && /name[[:space:]]*=/ { print; exit }
  ' "$TF" | sed -E 's@.*/\$\{var\.project_name\}/([a-z0-9-]+)".*@\1@')
  if [ -z "$param" ] || [ "${param#*=}" != "$param" ]; then
    echo "FAIL aws_ssm_parameter.$res の name からパラメータ名を取れない（name の書き方が変わった）"
    fail=$((fail + 1))
    continue
  fi
  # /${project}/bff-internal-token -> BFF_INTERNAL_TOKEN
  envvar=$(printf '%s' "$param" | tr 'a-z-' 'A-Z_')
  checked=$((checked + 1))

  # 新規インスタンス向け(user_data)に配られていること。
  if ! grep -qE "^${envvar}=" "$TPL"; then
    echo "FAIL $envvar が app_user_data.sh.tftpl に無い（新規インスタンスへ配られない）"
    fail=$((fail + 1))
  fi

  # 稼働中インスタンス向けに SSM から読んでいること。
  if ! grep -q "/\${{ env.PROJECT_NAME }}/$param" <<< "$SSH_BODY"; then
    echo "FAIL SSM /\${{ env.PROJECT_NAME }}/$param を Deploy via SSH が読んでいない（稼働中インスタンスに届かない）"
    fail=$((fail + 1))
    continue
  fi

  # 既存の値を消してから書き直すこと。append だけだと古い行が勝ち、全台が揃わない。
  if ! grep -q "/^${envvar}=/d" <<< "$SSH_BODY"; then
    echo "FAIL $envvar の継ぎ足しが既存行を削除していない（古い値が残り全台で揃わない）"
    fail=$((fail + 1))
  fi
  if ! grep -qE "echo \"?${envvar}=" <<< "$SSH_BODY"; then
    echo "FAIL $envvar を .env へ書き足していない"
    fail=$((fail + 1))
  fi

  # 読めなかったときに .env を壊さないこと（新規インスタンスは user_data が既に正しい値を書いている）。
  if ! grep -q "WARN: SSM /\${{ env.PROJECT_NAME }}/$param" <<< "$SSH_BODY"; then
    echo "FAIL SSM /$param が読めなかった場合の警告が無い（不一致に気づけない）"
    fail=$((fail + 1))
  fi

  # インスタンスロールに読み取り許可があること。無いと本番相当の環境で必ず空になる。
  if ! grep -q "aws_ssm_parameter.$res.arn" "$TF"; then
    echo "FAIL aws_ssm_parameter.$res の読み取り許可が IAM ポリシーに無い"
    fail=$((fail + 1))
  fi

  [ "$fail" -eq 0 ] && echo "ok   $envvar: user_data / SSM継ぎ足し / IAM が揃っている"
done

# .env を書くヒアドキュメントの区切り文字が引用されていること。
# 引用が無いと bash が値の $ / `...` / \ を展開してから書き込むため、
# シークレットに $(...) が含まれると cloud-init の root 権限で実行され、
# 未定義の $name が含まれると set -u でそこで落ちて docker compose まで到達しない。
if grep -qE "cat > /opt/app/\.env <<'[A-Za-z]+'" "$TPL"; then
  echo "ok   .env のヒアドキュメント区切り文字が引用されている"
else
  echo "FAIL .env のヒアドキュメント区切り文字が引用されていない（値の \$ や \$(...) が bash に展開される）"
  fail=$((fail + 1))
fi

# .env へ値を書く echo が引用されていること。
# 引用が無いと値の連続した空白やタブで単語分割され、echo が単一スペースへ詰め直すので
# .env の値が SSM / user_data の値と一致しなくなる（検証が静かに失敗する）。
unquoted=$(grep -nE '^[[:space:]]*echo [A-Z_]+=' <<< "$SSH_BODY")
if [ -n "$unquoted" ]; then
  echo "FAIL .env へ書く echo が引用されていない（値の空白で単語分割される）:"
  sed 's/^/     /' <<< "$unquoted"
  fail=$((fail + 1))
else
  echo "ok   .env へ書く echo がすべて引用されている"
fi

echo "検査した SSM パラメータ: $checked 件"
if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
