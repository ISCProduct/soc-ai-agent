#!/bin/bash
# bind-error-check.sh 自体をテストする。
#
# 検査スクリプトは「壊れていても緑のまま」になりやすい。誤検知すれば無関係な PR が
# 赤くなり、見逃せば検査がある安心感だけが残る。どちらも実際に Go ファイルを
# 置いて確かめる。
#
# 使い方: ./automation/test/bind-error-check-test.sh

set -uo pipefail

CHECK="$(dirname "$0")/bind-error-check.sh"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

failures=0

# run は fixture を1つ置いて検査を走らせ、期待する終了コードと比べる。
run() {
  local name="$1" expected="$2" code="$3"
  rm -f "$WORK"/*.go
  printf '%s\n' "package fixture" "" "$code" > "$WORK/fixture.go"

  local out
  out=$(bash "$CHECK" "$WORK" 2>&1)
  local actual=$?

  if [ "$actual" -ne "$expected" ]; then
    echo "NG: $name (期待 exit=$expected, 実際 exit=$actual)"
    printf '%s\n' "$out" | sed 's/^/    /'
    failures=$((failures + 1))
  else
    echo "ok: $name"
  fi
}

# --- 通すべき形 ---
run "同じ行でエラーを検査していれば通す" 0 'func h(ctx C) error { if err := ctx.Bind(&req); err != nil { return err }; return nil }'
run "次の行でエラーを検査していれば通す" 0 'func h(ctx C) error {
	err := ctx.Bind(&req)
	if err != nil {
		return err
	}
	return nil
}'
run "空行を挟んで検査していても通す" 0 'func h(ctx C) error {
	err := ctx.Bind(&req)

	if err != nil {
		return err
	}
	return nil
}'
run "意図的に捨てる _ = は通す" 0 'func h(ctx C) { _ = ctx.Bind(&req) }'
# 経緯を日本語コメントに残すリポジトリなので、ここが誤検知すると無関係な PR が赤くなる
run "コメント行の記述では落とさない" 0 '// 以前は ctx.Bind(&req) と書いていて #1064 を踏んだ
func h(ctx C) error { if err := ctx.Bind(&req); err != nil { return err }; return nil }'
# echo の Context 以外の Bind（DIコンテナ・SQLドライバ等）は対象外
run "echo以外のBindは対象外" 0 'func h(ctx C) error {
	container.Bind(NewThing)
	stmt.Bind(1, "x")
	if err := ctx.Bind(&req); err != nil {
		return err
	}
	return nil
}'

# --- 落とすべき形 ---
run "戻り値を捨てていれば落とす" 1 'func h(ctx C) { ctx.Bind(&req) }'
run "代入するだけで検査しなければ落とす" 1 'func h(ctx C) { err := ctx.Bind(&req); log.Println(err.Error) }'
run "代入の次の行で検査していなければ落とす" 1 'func h(ctx C) error {
	err := ctx.Bind(&req)
	log.Println("bound")
	return nil
}'
run "ポインタ変数をそのまま渡す形も見る" 1 'func h(ctx C) { ctx.Bind(req) }'
# 行末コメントで逃げられると実バグを隠せてしまう
run "行末にコメントを付けても落とす" 1 'func h(ctx C) { ctx.Bind(&req) // あとで直す }'

# --- 検査対象が空でも緑にしない ---
rm -f "$WORK"/*.go
if bash "$CHECK" "$WORK" >/dev/null 2>&1; then
  echo "NG: 対象に Go ファイルが1つも無いのに PASS した（検査したつもりで素通りする）"
  failures=$((failures + 1))
else
  echo "ok: 対象が空なら失敗させる"
fi

if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures 件"
  exit 1
fi
echo "PASS (bind-error-check.sh の挙動 12 件)"
