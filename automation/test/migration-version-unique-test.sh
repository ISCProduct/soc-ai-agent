#!/usr/bin/env bash
# マイグレーションの版番号が重複していないか検査する。
#
# golang-migrate は同じ版番号のファイルが2つあると source を開く時点で落ちる。
#   ERR: duplicate migration file: 000041_xxx.down.sql
# deployment.yml は staging でも本番でも migrate up を実行し、失敗したら止まるので、
# 重複したままマージすると両方のデプロイが通らなくなる。
#
# 並行してブランチを切ると次の番号を取り合うので、人の注意では防げない。
#
# 検査は2段ある。
#
#   1. ツリー内の重複（同じ版番号のファイルが2つ）と down の欠落
#   2. BASE_REF（通常 origin/develop）に既にある版番号を別名で使っていないか
#
# 2 が無いと、自分のブランチ内では一意でも develop 側と衝突していることに
# マージまで気づけない。実際に #1585 と #1621 が両方 develop の 000042 と
# 衝突したまま開いていた。
set -euo pipefail

MIGRATIONS_DIR="${1:-Backend/migrations}"
# 比較対象。空ならツリー内の検査だけ行う（ローカル実行時の既定）。
BASE_REF="${BASE_REF:-}"

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

# ── 2. BASE_REF との衝突 ───────────────────────────────────────────
# 同じ版番号を「別の名前」で使っていたら衝突。名前まで同じなら、それは
# 既にマージ済みの同一マイグレーションなので問題ない（develop 自身を
# BASE_REF にしても落ちない）。
if [ -n "$BASE_REF" ]; then
  if ! git rev-parse --verify --quiet "$BASE_REF" >/dev/null; then
    echo "::error::BASE_REF が解決できません: ${BASE_REF}（git fetch が必要かもしれません）"
    exit 1
  fi

  base_list=$(
    git ls-tree -r --name-only "$BASE_REF" -- "$MIGRATIONS_DIR" 2>/dev/null \
      | grep -E '\.up\.sql$' \
      | sed -E 's|.*/([0-9]+)_(.*)\.up\.sql|\1 \2|' \
      | sort || true
  )

  while IFS= read -r up; do
    [ -z "$up" ] && continue
    base_name=$(basename "$up")
    version="${base_name%%_*}"
    name=$(echo "$base_name" | sed -E 's|^[0-9]+_(.*)\.up\.sql|\1|')

    base_name_for_version=$(echo "$base_list" | awk -v v="$version" '$1==v {print $2; exit}')
    [ -z "$base_name_for_version" ] && continue
    [ "$base_name_for_version" = "$name" ] && continue

    fail=1
    echo "::error::版番号 ${version} は ${BASE_REF} で既に使われています"
    echo "    ${BASE_REF}:   ${version}_${base_name_for_version}"
    echo "    このブランチ: ${version}_${name}"
  done < <(find "$MIGRATIONS_DIR" -name '*.up.sql' -type f | sort)

  if [ "$fail" -ne 0 ]; then
    next=$(echo "$base_list" | awk '{print $1}' | sort -n | tail -1)
    echo "::error::${BASE_REF} の最新は ${next} です。$(printf '%06d' $((10#$next + 1))) 以降へ採番し直してください"
    exit 1
  fi
fi

count=$(find "$MIGRATIONS_DIR" -name '*.up.sql' -type f | wc -l | tr -d ' ')
if [ -n "$BASE_REF" ]; then
  echo "✓ マイグレーション ${count} 件: 版番号の重複なし、down も揃っている、${BASE_REF} とも衝突なし"
else
  echo "✓ マイグレーション ${count} 件: 版番号の重複なし、down も揃っている"
fi
