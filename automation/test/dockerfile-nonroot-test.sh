#!/bin/bash
# コンテナが非rootで動く条件を検査する。
#
# #1470 で Backend だけ非root化したが、frontend / rag / rag(finetune) /
# company-graph は root のまま残っていた（#1477）。コンテナ内でのRCEや
# パストラバーサルが起きたとき、rootのままだとホスト側の攻撃面が広がる。
# 本番は ECS on Fargate で動くため、タスク定義に user 指定が無い限り
# イメージの USER がそのまま実行ユーザーになる。
#
# 検査する内容:
#   A. 全 Dockerfile の最終ステージに非rootの USER がある
#   B. 非rootで書ける出力先が用意されている（root 実行前提だったパス）
#   C. compose の frontend ボリュームが匿名でない（旧root所有を引き継がない）
#   D. A の判定ロジック自体の自己検査
#
# 最終ステージだけを見るのは、builder ステージの USER は実行時に効かないため。
#
# 使い方: ./automation/test/dockerfile-nonroot-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

fail=0
checked=0

# check_dockerfile は Dockerfile 1つを検査し、問題があれば 1 を返す。
# 表示名は $2（省略時はパスそのもの）。
check_dockerfile() {
  local f="$1" name="${2:-$1}"

  # 最終ステージ = 最後の FROM 以降。builder の USER を数えないための切り出し。
  local last_from
  last_from=$(grep -n -iE '^[[:space:]]*FROM[[:space:]]' "$f" | tail -1 | cut -d: -f1)
  if [ -z "$last_from" ]; then
    echo "FAIL $name に FROM がありません（抽出条件が壊れています）"
    return 1
  fi
  local stage
  stage=$(tail -n "+$last_from" "$f")

  local user
  user=$(grep -iE '^[[:space:]]*USER[[:space:]]' <<< "$stage" | tail -1 | awk '{print $2}')
  if [ -z "$user" ]; then
    echo "FAIL $name の最終ステージに USER がありません（root で動きます）"
    echo "     → 非rootユーザーを作って USER を指定してください（Backend/Dockerfile が見本）"
    return 1
  fi

  # 変数展開は静的に解決できない。`ARG RUN_USER=root` + `USER ${RUN_USER}` は
  # 実際には root で起動するのに、文字列比較だけだと素通りする（#1477 のレビュー指摘）。
  # 非root保証を名乗る以上、解決できない形は通さない。
  case "$user" in
    *'$'*)
      echo "FAIL $name の USER が変数です（${user}）。ビルド引数次第で root になり得ます"
      echo "     → 非rootユーザー名か uid をリテラルで書いてください"
      return 1
      ;;
  esac

  # "app:app" 形式でも uid/uname 側だけを見る
  local uid="${user%%:*}"
  case "$uid" in
    root | 0)
      echo "FAIL $name の USER が root です（${user}）"
      return 1
      ;;
  esac

  echo "ok   $name (USER $user)"
  return 0
}

echo "--- A. Dockerfile の最終ステージが非root ---"
while IFS= read -r f; do
  checked=$((checked + 1))
  check_dockerfile "$f" "${f#"$ROOT"/}" || fail=$((fail + 1))
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
echo "--- B. 非rootで書ける出力先 ---"
# lora_train.py の使用例は --output-dir /models/lora。root 実行だった頃は
# os.makedirs が /models を作れたが、非rootでは作れず PermissionError になる。
if grep -qE '^[[:space:]]*(RUN|&&)[^#]*mkdir[^#]*/models' "$ROOT/rag/Dockerfile.finetune" \
   && grep -qE '^[[:space:]]*(RUN|&&)[^#]*chown[^#]*/models' "$ROOT/rag/Dockerfile.finetune"; then
  echo "ok   rag/Dockerfile.finetune が /models を app 所有で用意している"
else
  echo "FAIL rag/Dockerfile.finetune に /models の作成と chown がありません"
  echo "     → 非rootでは --output-dir /models/lora の os.makedirs が PermissionError になります"
  fail=$((fail + 1))
fi

echo
echo "--- C. compose の frontend ボリューム ---"
# 匿名ボリューム（`- /app/.next`）は、root で動いていた旧コンテナが作ったものを
# `up --build` の再作成でそのまま引き継ぐ。イメージ内の chown はマウントに隠れるので
# 非root化した途端に EACCES になる。名前付きなら新規作成されイメージの所有権を継ぐ。
for path in /app/node_modules /app/.next; do
  if grep -qE "^[[:space:]]*-[[:space:]]*${path//\//\\/}[[:space:]]*$" "$ROOT/compose.yml"; then
    echo "FAIL compose.yml の ${path} が匿名ボリュームです（旧root所有を引き継ぎます）"
    echo "     → 名前付きボリューム（frontend_next 等）にしてください"
    fail=$((fail + 1))
  else
    echo "ok   compose.yml の ${path} は匿名ボリュームではない"
  fi
done

echo
echo "--- D. 判定ロジックの自己検査 ---"
# 検査スクリプトは「壊れていても緑のまま」になりやすい。実際に Dockerfile を置いて確かめる。
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

selfcheck() {
  local name="$1" expected="$2" content="$3"
  printf '%s\n' "$content" > "$WORK/Dockerfile"
  check_dockerfile "$WORK/Dockerfile" "fixture" > /dev/null 2>&1
  local actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL 自己検査: $name（期待 exit=$expected, 実際 exit=$actual）"
    fail=$((fail + 1))
  else
    echo "ok   自己検査: $name"
  fi
}

# --- 通すべき形 ---
selfcheck "ユーザー名指定は通す" 0 'FROM alpine
USER app'
selfcheck "uid:gid 指定は通す" 0 'FROM alpine
USER 10001:10001'
selfcheck "builder の root は最終ステージに影響しない" 0 'FROM alpine AS builder
USER root
FROM alpine
USER app'

# --- 落とすべき形 ---
selfcheck "USER が無い" 1 'FROM alpine
CMD ["sh"]'
selfcheck "USER root" 1 'FROM alpine
USER root'
selfcheck "USER 0" 1 'FROM alpine
USER 0'
selfcheck "USER root:root" 1 'FROM alpine
USER root:root'
# ARG 次第で root になる形。文字列比較だけだと素通りしていた（#1477 レビュー指摘）
selfcheck "USER \${VAR}" 1 'FROM alpine
ARG RUN_USER=root
USER ${RUN_USER}'
selfcheck "USER \$VAR" 1 'FROM alpine
ARG RUN_USER=root
USER $RUN_USER'
# 最終ステージに USER が無ければ、builder に USER app があっても落ちる
selfcheck "最終ステージに USER が無い（builder だけ非root）" 1 'FROM alpine AS builder
USER app
FROM alpine
CMD ["sh"]'

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS ($checked Dockerfile を検査)"
