#!/bin/bash
# ctx.Bind() の戻り値を捨てているコードを静的に検出する（#1106 / #1064 横展開）。
#
# Bind のエラーを無視すると、壊れたJSONが「ゼロ値の構造体」として素通りし、
# 既定値や空文字で処理が続く。admin の再計算のように、ゼロ値が
# 「全件を別条件で処理する」に化ける経路がある。
#
# #1064 は resume_controller の2箇所を直す issue としてクローズされたが、
# 実際にはコードに修正が入っていなかった。レビューと issue のクローズだけでは
# 再発を止められないため、検査で固定する。
#
# 意図的に無視する場合は `_ = ctx.Bind(&x)` と書くこと（意図が読めるため許可する）。
#
# 使い方: ./automation/test/bind-error-check.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

hits=$(grep -rn "\.Bind(&" "$ROOT/Backend" --include="*.go" || true)
total=$(printf '%s\n' "$hits" | grep -c . || true)

# 抽出条件が壊れると「検査したつもり」で素通りする。件数の下限を固定して検出する。
MIN_CHECKED=50
if [ "$total" -lt "$MIN_CHECKED" ]; then
  echo "NG: Bind の呼び出しが $total 件しか見つかりません（期待 $MIN_CHECKED 件以上）。抽出条件が壊れています。"
  exit 1
fi

# エラーを受けている（if err := / err := / err = ）か、意図的に捨てている（_ = ）なら OK
bad=$(printf '%s\n' "$hits" | grep -vE "if err :?=|err :?= |_ = " || true)

if [ -n "$bad" ]; then
  echo "FAIL: Bind のエラーを捨てている箇所があります。400 を返すか、意図的なら _ = を付けてください。"
  printf '%s\n' "$bad" | sed "s|$ROOT/||"
  exit 1
fi

echo "PASS (Bind 呼び出し $total 件すべてがエラーを処理、または意図的に無視)"
