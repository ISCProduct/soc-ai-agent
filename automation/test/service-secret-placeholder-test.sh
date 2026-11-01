#!/bin/bash
# SERVICE_SECRET のプレースホルダーが「例示されている値」と「弾かれる値」で一致していることを固定する。
#
# SERVICE_SECRET は X_Admin-Secret の照合値で、一致するだけで
# /api/admin/whats-new/ingest と /api/admin/training/stats が通る(#1158)。
# 例示値をそのままコピーして動いてしまうと、リポジトリを読める第三者がそのまま叩ける。
#
# 塞ぎ方が2系統ある:
#   - Backend: config.go が serviceSecretPlaceholder と一致したら未設定扱い(503 fail-close)
#   - terraform(staging): variables.tf の validation が plan で止める
# 例示値がどちらにも弾かれる文字列であることを見る。example だけ書き換えて
# 片方をすり抜けるのが、この検査で防ぎたい事故。
#
# 使い方: ./automation/test/service-secret-placeholder-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

CONFIG="$ROOT/Backend/internal/config/config.go"
ENVEX="$ROOT/Backend/.env.example"
TFVARS="$ROOT/infra/terraform/environments/staging/terraform.tfvars.example"
TFVARIABLES="$ROOT/infra/terraform/environments/staging/variables.tf"

# コメント行を落とす。説明コメントに同じ語が出るだけで検査が通ってしまうと、
# 実際の判定を消しても落ちない「固めたつもり」のテストになる（#1909 レビュー指摘）。
# 落とすのは行まるごとのコメントだけ。行末コメントは残るが、ここで突き合わせるのは
# いずれもコード構造の一部（grep のコマンド行・HCL の式）なので行末コメントには現れない。
strip_comments() { grep -vE '^[[:space:]]*#' || true; }

fail=0

# 正になるのは Backend の定数。ここを変えたら全部追従させる。
placeholder=$(strip_comments < "$CONFIG" | grep -E '^const serviceSecretPlaceholder = ' | sed -E 's/.*= "([^"]*)".*/\1/')
if [ -z "$placeholder" ]; then
  echo "FAIL config.go に serviceSecretPlaceholder が無い（未設定扱いにする値が決まらない）"
  exit 1
fi
echo "ok   config.go のプレースホルダー: $placeholder"

# .env.example の例示値が同じであること（違うと開発環境で本物として通る）。
if ! strip_comments < "$ENVEX" | grep -qx "SERVICE_SECRET=$placeholder"; then
  echo "FAIL Backend/.env.example の SERVICE_SECRET が config.go のプレースホルダーと一致しない"
  fail=$((fail + 1))
else
  echo "ok   Backend/.env.example が同じ値を使っている"
fi

# tfvars の例示値が同じであること。
tfvalue=$(strip_comments < "$TFVARS" | grep -E '^service_secret_plain[[:space:]]*=' | sed -E 's/.*=[[:space:]]*"([^"]*)".*/\1/')
if [ -z "$tfvalue" ]; then
  echo "FAIL $TFVARS に service_secret_plain の例示が無い（必須変数なので example に無いと apply で詰まる）"
  fail=$((fail + 1))
elif [ "$tfvalue" != "$placeholder" ]; then
  echo "FAIL terraform.tfvars.example の例示値が config.go のプレースホルダーと違う: '$tfvalue'"
  echo "     (この値は validation も config.go も弾かないので、コピーしたまま apply すると第三者が叩ける)"
  fail=$((fail + 1))
else
  echo "ok   terraform.tfvars.example が同じ値を使っている"
fi

# validation がその値を弾くこと。
if ! strip_comments < "$TFVARIABLES" | grep -q "var.service_secret_plain != \"$placeholder\""; then
  echo "FAIL staging/variables.tf の validation が '$placeholder' を弾いていない"
  fail=$((fail + 1))
else
  echo "ok   staging/variables.tf の validation が同じ値を弾いている"
fi

# 許可リスト1本で形式を縛っていること。
#
# `.env` の1行として Compose が読む値なので、壊れ方が何通りもある（$ / # / 改行 /
# 空白 / "None"）。禁止文字を1つずつ足す形は次の文字で同じ穴が開くため、
# 文字種と最小長の許可リストに置き換えてある。条件の構成要素を個別に見る:
#   - アンカー（^ と $）: 無いと部分一致になり、禁止文字を含む値が通る
#   - 文字クラス        : 緩めると $ や # が通る
#   - 最小長            : 無いと "None" や短い値が通る
allow=$(strip_comments < "$TFVARIABLES" | grep -oE 'regex\("[^"]*", var\.service_secret_plain\)' | head -1)
if [ -z "$allow" ]; then
  echo "FAIL staging/variables.tf に service_secret_plain の形式を縛る regex が無い"
  fail=$((fail + 1))
else
  pattern=${allow#regex(\"}
  pattern=${pattern%%\", var.service_secret_plain)}
  echo "ok   形式の許可リスト: $pattern"

  case "$pattern" in
    "^"*) ;;
    *) echo "FAIL 許可リストが先頭アンカー(^)で始まっていない（部分一致になり禁止文字が通る）"; fail=$((fail + 1)) ;;
  esac
  case "$pattern" in
    *'$') ;;
    *) echo "FAIL 許可リストが末尾アンカー(\$)で終わっていない（末尾に何でも付けられる）"; fail=$((fail + 1)) ;;
  esac
  # 文字クラスに $ / # / 空白 / 改行 を含む書き方になっていないこと。
  if printf '%s' "$pattern" | grep -qE '\[[^]]*[$#[:space:]\\][^]]*\]'; then
    echo "FAIL 許可リストの文字クラスが広すぎる（\$ / # / 空白 / エスケープを許している）"
    fail=$((fail + 1))
  fi
  # 最小長の指定があること（"None" のような短い値を落とすため）。
  if ! printf '%s' "$pattern" | grep -qE '\{[0-9]+,?[0-9]*\}'; then
    echo "FAIL 許可リストに最小長の指定が無い（\"None\" や短い値が通る）"
    fail=$((fail + 1))
  else
    minlen=$(printf '%s' "$pattern" | grep -oE '\{[0-9]+' | head -1 | tr -d '{')
    if [ "$minlen" -lt 24 ]; then
      echo "FAIL 許可リストの最小長が短すぎる（$minlen。本番は random_password length=48）"
      fail=$((fail + 1))
    else
      echo "ok   最小長 $minlen 文字以上を要求している"
    fi
  fi

  # 実際に代表的な値を分類できること。grep -E で同じパターンを当てて突き合わせる。
  # （terraform は RE2、grep -E は POSIX ERE だが、この許可リストが使う構文は共通部分に収まる）
  check_pattern() {
    local label="$1" value="$2" want="$3" got
    if printf '%s' "$value" | grep -qE "$pattern"; then got=通る; else got=弾く; fi
    if [ "$got" != "$want" ]; then
      echo "FAIL 許可リストが '$label' を $want べきだが $got"
      fail=$((fail + 1))
    fi
  }
  good=$(printf 'a%.0s' $(seq 1 48))
  check_pattern "英数字48文字(本番のrandom_password相当)" "$good" 通る
  check_pattern "空文字" "" 弾く
  check_pattern "None(AWS CLIがnullを出す文字列)" "None" 弾く
  check_pattern "\$ を含む値" "${good}\$x" 弾く
  check_pattern "# を含む値" "${good}#x" 弾く
  check_pattern "空白を含む値" "${good} x" 弾く
  check_pattern "山括弧のプレースホルダー" "<SERVICE_SECRET>" 弾く
  echo "ok   代表的な壊れた値(空 / None / \$ / # / 空白 / 山括弧)をすべて弾く"
fi

if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
