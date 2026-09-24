#!/bin/bash
# 運用手順に「リビジョンを省略した update-service」が混ざらないことを固定する。
#
# `aws ecs update-service --task-definition soc-app-backend`（リビジョン省略）は最新の
# ACTIVE リビジョンを選ぶ。terraform apply 直後はそれが var.backend_image / var.frontend_image
# （tfvars 例では `:latest`）から登録した定義になるが、本番デプロイ(deployment.yml)は
# SHA タグしか push せず latest を更新しない。手順どおりに叩くと稼働中のイメージが
# 巻き戻るか、存在しないタグでデプロイが落ちる。
# 必ず register-task-definition で得た ARN / family:revision を渡すこと。
#
# 検査する内容:
#   A. docs / automation / .github の実行行に、リビジョン省略の update-service が無い
#   B. 判定ロジックと抽出条件自体の自己検査（誤検知・検査漏れの両方向）
#
# 抽出は「実際にシェルが実行しうる行」だけを対象にする。コメントや Markdown 本文で
# 危険な形を説明することは正当なので、それを FAIL にすると全PRが落ちる（#1503 レビュー指摘）。
#
# 使い方: ./automation/test/ecs-update-service-revision-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

fail=0
found=0
cmd_count=0

# executable_lines は実行されうる行だけを出す。
#   - .md はフェンス付きコードブロックの中だけ（本文の「やってはいけない例」を拾わない）
#   - `#` から行末はコメント（シェル / YAML / コードブロック内の注釈）なので落とす。
#     行頭か空白直後の `#` だけを見て、`soc-app#1` のようなトークン内の `#` は残す。
executable_lines() {
  local f="$1"
  case "$f" in
    *.md) awk '/^[[:space:]]*```/ { in_block = !in_block; next } in_block' "$f" ;;
    *) cat "$f" ;;
  esac | sed -e 's/^#.*$//' -e 's/[[:space:]]#.*$//'
}

# extract_commands は行継続(\)を畳んでから update-service のコマンドを1行として取り出す。
# コメント除去を先に済ませるのは、シェルではコメント行末の `\` が継続にならないため。
extract_commands() {
  executable_lines "$1" \
    | sed -e :a -e '/\\$/N; s/\\\n/ /; ta' \
    | grep 'aws ecs update-service'
}

# is_pinned は --task-definition の値がリビジョンまで固定されているかを返す。
#   soc-app-backend:42            -> ok
#   arn:aws:ecs:...:task-definition/soc-app-backend:42 -> ok
#   "$new_td"                     -> ok（値全体が変数。register-task-definition の戻り値を渡す形）
#   soc-app-backend               -> NG（最新 ACTIVE が選ばれる）
#   soc-app-$svc                  -> NG（変数を含むが family のまま。リビジョンが無い）
#   arn:aws:ecs:...:task-definition/soc-app-backend    -> NG（ARN もリビジョン省略できる）
#
# 値全体が変数の形だけは静的に中身を追えないので通している。手順側は必ず
# register-task-definition が返した ARN を入れること（ここでは保証できない）。
is_pinned() {
  local td="$1" name
  # ARN なら最後の `/` 以降が family[:revision]。ARN 自体のコロンを数えないための切り出し。
  name="${td##*/}"
  case "$name" in
    *:*) return 0 ;;
  esac
  [[ "$name" =~ ^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?$ ]] && return 0
  return 1
}

# check_file は1ファイルを検査し、危険な形があれば 1 を返す。
# 見つかったコマンド数はグローバルの cmd_count に入れる（抽出条件が壊れた検知用）。
check_file() {
  local file="$1" name="${2:-$1}" bad=0 cmd td
  cmd_count=0
  while IFS= read -r cmd; do
    # `--task-definition foo` と `--task-definition=foo` の両方を拾う
    td=$(printf '%s\n' "$cmd" | sed -n 's/.*--task-definition[[:space:]=]\{1,\}\([^[:space:]]*\).*/\1/p' | tr -d '"'"'")
    if [ -z "$td" ]; then
      # --desired-count や --force-new-deployment だけの更新は稼働中の定義を保つので対象外
      continue
    fi
    cmd_count=$((cmd_count + 1))
    if is_pinned "$td"; then
      echo "ok   $name: --task-definition '$td'"
    else
      echo "FAIL $name: --task-definition '$td' はリビジョン省略。稼働中イメージを巻き戻す"
      bad=1
    fi
  done < <(extract_commands "$file")
  return $bad
}

echo "--- A. 運用手順の update-service ---"
while IFS= read -r file; do
  check_file "$file" "${file#"$ROOT"/}" || fail=$((fail + 1))
  found=$((found + cmd_count))
done < <(grep -rl 'aws ecs update-service' "$ROOT/docs" "$ROOT/automation" "$ROOT/.github" 2>/dev/null \
           | grep -v 'ecs-update-service-revision-test.sh')

# 抽出条件が壊れると「検査したつもり」で素通りする。
# operations.md のロールバック手順(backend/frontend)と反映手順の3件が最低ライン。
MIN_COMMANDS=3
echo "検査した update-service: ${found}件"
if [ "$found" -lt "$MIN_COMMANDS" ]; then
  echo "NG: update-service が $found 件しか見つかりません（期待 $MIN_COMMANDS 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi

echo
echo "--- B. 判定ロジックの自己検査 ---"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# selfcheck は fixture を置いて check_file の終了コードを確かめる。
# $1 表示名 / $2 fixture のファイル名（拡張子で .md 判定が変わる） / $3 期待 exit / $4 中身
selfcheck() {
  local name="$1" fixture="$WORK/$2" expected="$3" content="$4" actual
  printf '%s\n' "$content" > "$fixture"
  check_file "$fixture" "fixture" > /dev/null 2>&1
  actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL 自己検査: $name（期待 exit=$expected, 実際 exit=$actual）"
    fail=$((fail + 1))
  else
    echo "ok   自己検査: $name"
  fi
}

# --- 通すべき形（誤検知しないこと） ---
selfcheck "family:revision" f.sh 0 'aws ecs update-service --cluster soc-app --service backend --task-definition soc-app-backend:42'
selfcheck "値全体が変数" f.sh 0 'aws ecs update-service --cluster "$CLUSTER" --service "$svc" --task-definition "$new_td"'
selfcheck "値全体が変数(波括弧)" f.sh 0 'aws ecs update-service --service backend --task-definition "${new_td}"'
selfcheck "変数リビジョン" f.sh 0 'aws ecs update-service --service backend --task-definition "soc-app-$svc:$rev"'
selfcheck "リビジョン付きARN" f.sh 0 'aws ecs update-service --service backend --task-definition arn:aws:ecs:ap-northeast-1:123456789012:task-definition/soc-app-backend:42'
selfcheck "--task-definition なし" f.sh 0 'aws ecs update-service --cluster soc-app --service backend --desired-count 1 --force-new-deployment'
# #1503 レビュー指摘。test.yml の説明コメントを実コマンドとして拾い、全PRが落ちていた。
selfcheck "YAMLコメント内の危険な形" f.yml 0 '      # 運用手順の `aws ecs update-service --task-definition <family>`（リビジョン省略）を検査する
      - name: 検査
        run: ./automation/test/ecs-update-service-revision-test.sh'
selfcheck "シェルコメント内の危険な形" f.sh 0 '# aws ecs update-service --task-definition soc-app-backend は禁止'
selfcheck "行末コメント内の危険な形" f.sh 0 'echo hi   # aws ecs update-service --task-definition soc-app-backend はNG'
selfcheck "Markdown本文の危険な形" f.md 0 '> **`aws ecs update-service --task-definition soc-app-backend`（リビジョン省略）で更新してはいけない。**'
selfcheck "正しいコマンド＋行末コメント" f.sh 0 'aws ecs update-service --service backend --task-definition "$new_td"   # リビジョンを明示する'

# --- 落とすべき形（検査漏れしないこと） ---
selfcheck "family 直書き" f.sh 1 'aws ecs update-service --cluster soc-app --service backend --task-definition soc-app-backend'
selfcheck "family 直書き(引用符付き)" f.sh 1 'aws ecs update-service --service backend --task-definition "soc-app-backend"'
selfcheck "family 直書き(=区切り)" f.sh 1 'aws ecs update-service --service backend --task-definition=soc-app-backend'
selfcheck "リビジョン無しARN" f.sh 1 'aws ecs update-service --service backend --task-definition arn:aws:ecs:ap-northeast-1:123456789012:task-definition/soc-app-backend'
# 変数を含むが family のままの形。`*$*` を一律に通すと素通りしていた
selfcheck "変数で組んだ family 直書き" f.sh 1 'aws ecs update-service --cluster "$CLUSTER" --service "$svc" --task-definition soc-app-$svc'
selfcheck "変数で組んだ family 直書き(波括弧)" f.sh 1 'aws ecs update-service --service backend --task-definition "${CLUSTER}-backend"'
selfcheck "行継続をまたぐ family 直書き" f.sh 1 'aws ecs update-service --cluster soc-app --service backend \
  --task-definition soc-app-backend'
selfcheck "Markdownコードブロック内の family 直書き" f.md 1 '手順:

```sh
aws ecs update-service --cluster soc-app --service backend --task-definition soc-app-backend
```'
selfcheck "危険なコマンド＋行末コメント" f.sh 1 'aws ecs update-service --service backend --task-definition soc-app-backend   # 最新が選ばれる'

# 抽出条件（コメント落とし・フェンス切り出し）が効きすぎて全部素通りしていないかを確かめる。
# 上の「落とすべき形」が1件でも通れば fail に出るので、ここは Markdown 抽出の有無だけ見る。
selfcheck "Markdownはコードブロック外を見ない" f.md 0 'aws ecs update-service --task-definition soc-app-backend'

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS (update-service ${found}件を検査)"
