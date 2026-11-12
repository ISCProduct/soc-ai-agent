#!/usr/bin/env bash
# このブランチが追加したマイグレーションを、BASE_REF の次の空き番号へ振り直す。
#
# 並行してブランチを切ると次の番号を取り合うため、採番し直しは繰り返し発生する。
# golang-migrate は適用済みより小さい版番号を二度と実行しないので（本番で静かに
# スキップされる）、版番号は BASE_REF の最大値より大きく、かつ欠番なしである必要がある。
# その制約自体は migration-version-unique-test.sh と migrations_test.go が検査するが、
# 直す作業は手でやると取り違えやすい。ここを1コマンドにする。
#
#   使い方:  ./automation/migration-renumber.sh [--dry-run] [BASE_REF]
#   既定の BASE_REF は origin/develop
#
# 複数本を追加しているブランチでは、現在の版番号の昇順を保ったまま詰めて振り直す。
# up / down の両方と、ファイル名を参照している箇所（コメント等）も直す。
set -euo pipefail

DRY_RUN=0
if [ "${1:-}" = "--dry-run" ]; then DRY_RUN=1; shift; fi
BASE_REF="${1:-origin/develop}"
MIGRATIONS_DIR="Backend/migrations"

[ -d "$MIGRATIONS_DIR" ] || { echo "error: $MIGRATIONS_DIR が見つかりません（リポジトリのルートで実行してください）" >&2; exit 1; }
git rev-parse --verify --quiet "$BASE_REF" >/dev/null || { echo "error: BASE_REF が解決できません: $BASE_REF（git fetch origin が必要かもしれません）" >&2; exit 1; }

# BASE_REF 側に存在する up のファイル名一覧（版番号 名前）
base_list=$(
  git ls-tree -r --name-only "$BASE_REF" -- "$MIGRATIONS_DIR" 2>/dev/null \
    | grep -E '\.up\.sql$' \
    | sed -E 's|.*/([0-9]+)_(.*)\.up\.sql|\1 \2|' \
    | sort || true
)
[ -n "$base_list" ] || { echo "error: $BASE_REF 側にマイグレーションが見つかりません" >&2; exit 1; }

base_max=$(echo "$base_list" | awk '{print $1}' | sort -n | tail -1)

# BASE_REF を取り込んでいないブランチで振り直すと、BASE_REF 側にしか無い版番号が
# このツリーでは欠番になり、連番検査（Backend/migrations/migrations_test.go）が落ちる。
# 例: develop=000066 を未取り込みのまま 000063 を 000067 へ振ると、63〜66 が欠番になる。
if ! git merge-base --is-ancestor "$BASE_REF" HEAD 2>/dev/null; then
  cat >&2 <<MSG
error: このブランチは $BASE_REF を取り込んでいません。

  先に取り込んでください:  git merge $BASE_REF

  取り込まずに振り直すと、$BASE_REF 側にしか無い版番号がこのツリーで欠番になり、
  連番検査が落ちます。
MSG
  exit 1
fi

# このブランチが追加した up（名前が BASE_REF 側に無いもの）を、現在の版番号の昇順で拾う
added=$(
  find "$MIGRATIONS_DIR" -name '*.up.sql' -type f \
    | sed -E 's|.*/([0-9]+)_(.*)\.up\.sql|\1 \2|' \
    | sort -n \
    | while read -r version name; do
        echo "$base_list" | awk -v n="$name" '$2==n {found=1} END {exit found?0:1}' || echo "$version $name"
      done
)

if [ -z "$added" ]; then
  echo "振り直す対象はありません（$BASE_REF に無いマイグレーションがこのブランチにありません）"
  exit 0
fi

echo "BASE_REF: $BASE_REF（最大 $base_max）"
echo "このブランチが追加したマイグレーション:"
echo "$added" | sed 's/^/    /'
echo

next=$((10#$base_max))
moved=0
while read -r version name; do
  [ -z "$version" ] && continue
  next=$((next + 1))
  new=$(printf '%06d' "$next")
  if [ "$version" = "$new" ]; then
    echo "  = ${version}_${name}（変更なし）"
    continue
  fi
  for suffix in up down; do
    old_file="$MIGRATIONS_DIR/${version}_${name}.${suffix}.sql"
    new_file="$MIGRATIONS_DIR/${new}_${name}.${suffix}.sql"
    [ -f "$old_file" ] || { echo "error: $old_file が見つかりません（down の欠落？）" >&2; exit 1; }
    [ -e "$new_file" ] && { echo "error: 移動先が既に存在します: $new_file" >&2; exit 1; }
    if [ "$DRY_RUN" -eq 1 ]; then
      echo "  → ${version}_${name}.${suffix}.sql => ${new}_${name}.${suffix}.sql （dry-run）"
    else
      git mv "$old_file" "$new_file"
      echo "  → ${version}_${name}.${suffix}.sql => ${new}_${name}.${suffix}.sql"
    fi
  done
  # 版番号への参照は「警告するだけ」。自動で書き換えない。
  # 番号だけで照合すると、同じ番号を使う別のマイグレーションを指すコメントまで
  # 巻き込む（migrations/000065 が別PRの 000065_ai_budget_settings を指していても、
  # こちらの 000065 の改番で書き換わってしまう）。どれが対象かは人が判断する。
  refs=$(grep -rln "migrations/${version}" --include='*.go' --include='*.md' . 2>/dev/null || true)
  if [ -n "$refs" ]; then
    while IFS= read -r f; do
      [ -z "$f" ] && continue
      echo "     !! ${f} が migrations/${version} を参照しています"
      echo "        その ${version} が ${name} を指すなら ${new} へ手で直してください"
      echo "        別のマイグレーションを指すなら、触らないでください"
    done <<< "$refs"
  fi
  moved=$((moved + 1))
done <<< "$added"

if [ "$DRY_RUN" -eq 1 ]; then
  echo
  echo "dry-run のため変更していません。"
  exit 0
fi

echo
echo "${moved} 件を振り直しました。検査を実行します。"
echo
BASE_REF="$BASE_REF" ./automation/test/migration-version-unique-test.sh "$MIGRATIONS_DIR"
(cd Backend && go test ./migrations/... )

cat <<'MSG'

検査が通りました。コミットしてください。版番号は BASE_REF のその時点の値に対してのみ有効です。
他のマイグレーションPRが先にマージされたら、もう一度このコマンドを実行してください。
MSG
