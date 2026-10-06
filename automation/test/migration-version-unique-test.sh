#!/usr/bin/env bash
# マイグレーションの版番号が重複していないか検査する。
#
# golang-migrate は同じ版番号のファイルが2つあると source を開く時点で落ちる。
#   ERR: duplicate migration file: 000041_xxx.down.sql
# deployment.yml は staging でも本番でも migrate up を実行し、失敗したら止まるので、
# 重複したままマージすると両方のデプロイが通らなくなる。
#
# 並行してブランチを切ると次の番号を取り合うので、人の注意では防げない。
set -euo pipefail

MIGRATIONS_DIR="${1:-Backend/migrations}"

if [ ! -d "$MIGRATIONS_DIR" ]; then
  echo "::error::マイグレーションのディレクトリが見つかりません: $MIGRATIONS_DIR"
  exit 1
fi

fail=0

# 版番号（先頭の数字）ごとに、up/down それぞれが1つずつであることを確かめる。
for suffix in up down; do
  dups=$(
    find "$MIGRATIONS_DIR" -name "*.${suffix}.sql" -type f \
      | sed -E 's|.*/([0-9]+)_.*|\1|' \
      | sort | uniq -d
  )
  if [ -n "$dups" ]; then
    fail=1
    while IFS= read -r version; do
      [ -z "$version" ] && continue
      echo "::error::版番号 ${version} の .${suffix}.sql が重複しています"
      find "$MIGRATIONS_DIR" -name "${version}_*.${suffix}.sql" -type f | sed 's/^/    /'
    done <<< "$dups"
  fi
done

# up に対応する down が無いものも拾う（down が無いと巻き戻せない）。
while IFS= read -r up; do
  down="${up%.up.sql}.down.sql"
  if [ ! -f "$down" ]; then
    fail=1
    echo "::error::down が存在しません: $(basename "$up")"
  fi
done < <(find "$MIGRATIONS_DIR" -name '*.up.sql' -type f)

if [ "$fail" -ne 0 ]; then
  echo "::error::採番し直すか、down を追加してください"
  exit 1
fi

count=$(find "$MIGRATIONS_DIR" -name '*.up.sql' -type f | wc -l | tr -d ' ')
echo "✓ マイグレーション ${count} 件: 版番号の重複なし、down も揃っている"
