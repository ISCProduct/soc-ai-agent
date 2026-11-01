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

# 空文字も弾くこと（空のまま apply すると SSM パラメータまで空になる）。
if ! strip_comments < "$TFVARIABLES" | grep -q "length(var.service_secret_plain) > 0"; then
  echo "FAIL staging/variables.tf の validation が空文字を弾いていない"
  fail=$((fail + 1))
else
  echo "ok   staging/variables.tf の validation が空文字を弾いている"
fi

# 山括弧（`<...>` 形のプレースホルダー）を弾くこと。
if ! strip_comments < "$TFVARIABLES" | grep -q 'regex("\[<>\]", var.service_secret_plain)'; then
  echo "FAIL staging/variables.tf の validation が山括弧を含む値を弾いていない"
  echo "     (example を `<...>` 形に書き換えた人が同じ穴を開けられる)"
  fail=$((fail + 1))
else
  echo "ok   staging/variables.tf の validation が山括弧を含む値を弾いている"
fi

# 改行を弾くこと。引用では防げず、.env の2行目以降が別の環境変数として読まれる。
if ! strip_comments < "$TFVARIABLES" | grep -q 'regex("\[.r.n\]", var.service_secret_plain)'; then
  echo "FAIL staging/variables.tf の validation が改行を弾いていない"
  echo "     (.env の1行として書かれるので、改行が入ると2行目以降が別の変数になる)"
  fail=$((fail + 1))
else
  echo "ok   staging/variables.tf の validation が改行を弾いている"
fi

# $ を弾くこと。Compose が .env の引用なしの値を展開するので、含むと値がずれる。
if ! strip_comments < "$TFVARIABLES" | grep -q 'regex("\[[$]\]", var.service_secret_plain)'; then
  echo "FAIL staging/variables.tf の validation が \$ を含む値を弾いていない"
  echo "     (Compose が .env の値を展開するので、コンテナが受け取る値とCIの値がずれる)"
  fail=$((fail + 1))
else
  echo "ok   staging/variables.tf の validation が \$ を含む値を弾いている"
fi

if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
