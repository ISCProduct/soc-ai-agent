#!/bin/bash
# リポジトリのファイルを読むワークフローに actions/checkout があることを検査する。
#
# backlog-deleted-to-github-issue.yml が checkout を置かないまま
# .github/scripts/backlog_client.py を import しており、毎時の実行が
# ModuleNotFoundError で失敗し続けていた（2026-09-08〜09-09 で直近5回すべて失敗）。
#
# schedule で動くワークフローは誰も結果を見ないため、壊れても気づけない。
# 静的に検出する。
#
# 使い方: ./automation/test/workflow-checkout-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF_DIR="$ROOT/.github/workflows"

fail=0
checked=0
bl=0
au=0

for f in "$WF_DIR"/*.yml "$WF_DIR"/*.yaml; do
  [ -f "$f" ] || continue
  name=$(basename "$f")

  # リポジトリのファイルに依存する記述
  needs_repo=""
  grep -q "sys.path.insert(0, \".github/scripts\")" "$f" && needs_repo="${needs_repo}backlog_client "
  grep -qE "\./automation/(test|discord|ops)/" "$f" && needs_repo="${needs_repo}automation/ "
  grep -qE "^\s*run:.*\./(scripts|automation)/" "$f" && needs_repo="${needs_repo}script "

  [ -z "$needs_repo" ] && continue
  checked=$((checked + 1))
  case "$needs_repo" in *backlog_client*) bl=$((bl + 1));; esac
  case "$needs_repo" in *automation/*) au=$((au + 1));; esac

  if grep -q "actions/checkout@" "$f"; then
    echo "ok   $name (依存: ${needs_repo%% })"
  else
    echo "FAIL $name はリポジトリのファイル（${needs_repo%% }）を読むのに actions/checkout がありません"
    fail=$((fail + 1))
  fi
done

# ここからジョブ単位の検査。
#
# 上のファイル単位の判定は、同じワークフロー内の別ジョブに checkout があると
# 通ってしまう。実際 deployment.yml の sync-whats-new が
# .github/scripts/collect_whats_new_sources.py を checkout せずに実行し、
# 本番デプロイ後の更新情報の取り込みが No such file or directory で落ちた。
# checkout はジョブごとに要るので、ジョブ単位でも見る。
job_fail=0
job_checked=0
for f in "$WF_DIR"/*.yml "$WF_DIR"/*.yaml; do
  [ -f "$f" ] || continue
  out=$(WF="$f" python3 - <<'PYEOF'
import os, re, sys

path = os.environ["WF"]
src = open(path, encoding="utf-8").read()
name = os.path.basename(path)

m = re.search(r'^jobs:\s*$', src, re.M)
if not m:
    sys.exit(0)
body = src[m.end():]

# 2スペースインデントのキーでジョブを切る
starts = [(mm.start(), mm.group(1)) for mm in re.finditer(r'^  ([A-Za-z0-9_-]+):\s*$', body, re.M)]
fails = 0
checked = 0
for i, (pos, job) in enumerate(starts):
    end = starts[i + 1][0] if i + 1 < len(starts) else len(body)
    chunk = body[pos:end]
    # リポジトリのファイルを実行/参照しているか
    needs = re.search(r'(\.github/scripts/[\w.-]+|\./automation/[\w./-]+|\./scripts/[\w./-]+)', chunk)
    if not needs:
        continue
    checked += 1
    if "actions/checkout@" not in chunk:
        print(f"FAIL {name} のジョブ {job} は {needs.group(1)} を使うのに actions/checkout がありません")
        fails += 1
print(f"__COUNT__ {checked} {fails}")
PYEOF
)
  echo "$out" | grep -v "^__COUNT__" | grep -q . && echo "$out" | grep -v "^__COUNT__"
  nums=$(echo "$out" | grep "^__COUNT__" | awk '{print $2" "$3}')
  job_checked=$((job_checked + $(echo "$nums" | awk '{print $1}')))
  job_fail=$((job_fail + $(echo "$nums" | awk '{print $2}')))
done

# ジョブ単位の抽出が壊れると素通りする。最低件数で守る。
MIN_JOBS=3
if [ "$job_checked" -lt "$MIN_JOBS" ]; then
  echo "NG: リポジトリのファイルを使うジョブが $job_checked 件しか見つかりません（期待 $MIN_JOBS 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi
fail=$((fail + job_fail))

# 抽出条件がひとつでも壊れると「検査したつもり」で素通りする。
# 依存の種類ごとに最低件数を固定して、条件の欠落を検出する。
# 対象を減らすときはこの数も一緒に下げること。
MIN_BACKLOG_CLIENT=5
MIN_AUTOMATION=3
if [ "$bl" -lt "$MIN_BACKLOG_CLIENT" ]; then
  echo "NG: backlog_client を読むワークフローが $bl 件しか見つかりません（期待 $MIN_BACKLOG_CLIENT 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi
if [ "$au" -lt "$MIN_AUTOMATION" ]; then
  echo "NG: automation/ を読むワークフローが $au 件しか見つかりません（期待 $MIN_AUTOMATION 件以上）。抽出条件が壊れています。" >&2
  exit 1
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS (ファイル単位 $checked 件 / ジョブ単位 $job_checked 件を検査: backlog_client=$bl automation=$au)"
