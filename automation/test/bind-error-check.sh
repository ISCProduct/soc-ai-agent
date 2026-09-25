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
# 許容する形:
#   if err := ctx.Bind(&x); err != nil { ... }
#   err := ctx.Bind(&x)
#   if err != nil { ... }              ← 次の非空行で見ていれば可
#   _ = ctx.Bind(&x)                   ← 意図的に無視（意図が読めるため）
#
# 検出できない形: Bind を複数行に分けて書いた場合（行単位の検査の限界）。
# 現状そう書かれた箇所は無い。
#
# 使い方: ./automation/test/bind-error-check.sh [検査対象ディレクトリ]
# 対象を指定した場合は件数の下限を1件に緩める（自己テスト用）。

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TARGET="${1:-$ROOT/Backend}"

# echo の Context に対する Bind だけを見る。`.Bind(` 全部を対象にすると
# DIコンテナや sql ドライバの Bind まで巻き込む。
BIND_RE='(ctx|c)\\.Bind\\('

# 抽出条件が壊れると「検査したつもり」で素通りする。件数の下限を固定して検出する。
if [ "$#" -eq 0 ]; then
  MIN_CHECKED=50
else
  MIN_CHECKED=1
fi

total=0
bad=""
while IFS= read -r f; do
  out=$(awk -v file="$f" -v re="$BIND_RE" '
    { l[NR] = $0 }
    END {
      for (i = 1; i <= NR; i++) {
        s = l[i]
        if (s ~ /^[[:space:]]*\/\//) continue          # 行コメント
        if (s !~ re) continue
        printf "HIT\n"
        if (s ~ /if err :?=/) continue                 # 同じ行で検査している
        if (s ~ /_ =/) continue                        # 意図的に無視
        if (s ~ /err :?=/) {                           # 代入だけ。次の非空行で見ているか
          j = i + 1
          while (j <= NR && l[j] ~ /^[[:space:]]*$/) j++
          if (j <= NR && l[j] ~ /if err != nil/) continue
        }
        printf "BAD\t%s:%d:%s\n", file, i, s
      }
    }
  ' "$f")
  hits=$(printf '%s\n' "$out" | grep -c '^HIT$' || true)
  total=$((total + hits))
  bad_lines=$(printf '%s\n' "$out" | sed -n 's/^BAD\t//p')
  if [ -n "$bad_lines" ]; then
    bad="${bad}${bad_lines}
"
  fi
done < <(find "$TARGET" -name '*.go' -type f)

if [ "$total" -lt "$MIN_CHECKED" ]; then
  echo "NG: Bind の呼び出しが $total 件しか見つかりません（期待 $MIN_CHECKED 件以上）。抽出条件が壊れています。"
  exit 1
fi

if [ -n "$(printf '%s' "$bad" | tr -d '[:space:]')" ]; then
  echo "FAIL: Bind のエラーを捨てている箇所があります。if err := ctx.Bind(&x); err != nil で 400 を返すか、意図的なら _ = を付けてください。"
  printf '%s' "$bad" | sed "s|$ROOT/||"
  exit 1
fi

echo "PASS (Bind 呼び出し $total 件すべてがエラーを処理、または意図的に無視)"
