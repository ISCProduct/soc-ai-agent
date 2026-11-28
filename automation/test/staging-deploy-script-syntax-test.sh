#!/bin/bash
# staging の「Deploy via SSH」の手順（deploy-staging-job.yml の script ブロック）が、シェルとして正しく読めることを固定する。
#
# 手順全体が sudo bash -c の単一引用符の中にあるため、途中に単一引用符を1つ書くだけで外側の引用が閉じ、
# 続きがシェル構文として解釈されて、staging のデプロイが構文エラーで止まる（SOCAIAGENT-726 のレビュー指摘）。
# YAML としては正しいので、他の検査やレビューで気づきにくい。bash -n で構文だけを確かめる。
#
# 使い方: ./automation/test/staging-deploy-script-syntax-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows/deploy-staging-job.yml"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

# 「script: |」の次の行から、インデントが浅くなる手前までを取り出す。${{ ... }} は Actions が埋めるのでダミーにする。
awk '
  /^[[:space:]]*script: \|[[:space:]]*$/ && !done { inblock=1; match($0, /^[[:space:]]*/); base=RLENGTH; next }
  inblock {
    if ($0 ~ /^[[:space:]]*$/) { print ""; next }
    match($0, /^[[:space:]]*/)
    if (RLENGTH <= base) { inblock=0; done=1; next }
    print substr($0, base + 3)
  }
' "$WF" | sed -E 's/\$\{\{[^}]*\}\}/X/g' > "$TMP"

lines=$(grep -c . "$TMP" || true)
# 抽出が壊れると空のスクリプトを検査して素通りする。行数の下限を固定する。
if [ "$lines" -lt 50 ]; then
  echo "FAIL Deploy via SSH の手順を取り出せませんでした（$lines 行）。抽出条件を見直してください" >&2
  exit 1
fi
if ! out=$(bash -n "$TMP" 2>&1); then
  echo "FAIL staging のデプロイ手順に構文エラーがあります（sudo bash -c の単一引用符の中に単一引用符を書いていないか確認）:" >&2
  echo "$out" >&2
  exit 1
fi
echo "PASS staging のデプロイ手順の構文（$lines 行）"
