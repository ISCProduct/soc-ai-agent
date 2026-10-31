#!/bin/bash
# 本番デプロイ後の「起動状態を 0 へ戻すか」の判定をテストする。
#
# 停止日のデプロイは確認のために本番を一時起動する。その後始末を見送ると、
# 次の毎時5分(prod-uptime-scheduler)まで Fargate 4タスクと RDS が課金される。
# 実際に 2026-10-31 のデプロイで踏んだ: SSM に `on:2026-10-16`（復帰日を過ぎて
# 失効済み）が残っていたのを、この判定が「起動指示」と読んで停止を見送った。
# スケジューラ側は同じ値を失効扱いにしていたので、片方だけ直すと再発する。
#
# カバー範囲は override / 日付リストから「停止するか」の確定まで。
# prod-scale.sh の中身（desired_count 更新やドレイン待ち）は対象外。
#
# 使い方: ./automation/test/prod-deploy-scale-down-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows/deployment.yml"

WORK=$(mktemp -d)
FRAGMENT="$WORK/fragment.sh"
trap 'rm -rf "$WORK"' EXIT

# 判定ロジックだけを抜き出す（TODAY_JST の決定から prod-scale.sh の呼び出しまで）。
# 抜き出せなければ「テストしたつもり」になるので必ず失敗させる。
START=$(grep -n "Scale production services back to zero" "$WF" | head -1 | cut -d: -f1)
if [ -z "$START" ]; then
  echo "NG: 0へ戻すステップが見つかりません。deployment.yml の構造変更を確認してください。" >&2
  exit 1
fi
# YAML の `if:` / `run:` 行を含めると source した時点で構文エラーになり、
# 全ケースが「判定前に落ちた」結果になる（最初に書いたとき実際に踏んだ）。
# シェルの先頭 TODAY_JST= から始める。
awk -v s="$START" 'NR > s && !started && /^          TODAY_JST=/ { started = 1 }
                   started { print; if ($0 ~ /prod-scale\.sh 0$/) exit }' "$WF" \
  | sed -e 's/^          //' \
        -e 's/${{ steps\.prod-svc\.outputs\.override_at_start }}/$STUB_AT_START/' \
  > "$FRAGMENT"
if ! grep -q 'prod-scale.sh 0' "$FRAGMENT" || ! grep -q 'prod-uptime-override' "$FRAGMENT"; then
  echo "NG: 判定ロジックを抽出できませんでした。" >&2
  exit 1
fi
if grep -q '{{' "$FRAGMENT"; then
  echo "NG: 展開されていない GitHub Actions 式が残っています。" >&2
  exit 1
fi
# YAML のキー行が混ざっていると source が構文エラーで即死し、
# 「全部 kept」が正常に見えてしまう。混入を検知して止める。
if grep -qE '^ *(if|run|name|env):' "$FRAGMENT"; then
  echo "NG: シェル以外の行(YAMLキー)が混ざっています。" >&2
  exit 1
fi
# 判定の各分岐が残っているか。消えたまま緑になるのを防ぐ。
# 復帰日の失効は「実装がある」ではなく下の挙動テストで確かめる（存在チェックにすると
# 変数名を書くだけで緑になり、失効を見ていないコードを通してしまう）。
for needle in 'TODAY_JST=' 'prod-uptime-dates'; do
  grep -q "$needle" "$FRAGMENT" || { echo "NG: $needle が抽出結果に無い" >&2; exit 1; }
done
# JST判定が UTC に化けると、JST 00:00〜09:00 が前日扱いになり復帰日の境界がずれる。
if ! grep -q 'TZ=Asia/Tokyo' "$FRAGMENT"; then
  echo "NG: JST(TZ=Asia/Tokyo)での日付判定が失われています。" >&2
  exit 1
fi

# prod-scale.sh はパス指定で呼ばれるためシェル関数では差し替えられない。
# 呼ばれたことが分かるスタブを置き、そこを cwd にして実行する。
mkdir -p "$WORK/automation/ops"
cat > "$WORK/automation/ops/prod-scale.sh" <<'STUB'
#!/bin/bash
echo "$1" > "$SCALE_MARKER"
STUB
chmod +x "$WORK/automation/ops/prod-scale.sh"

fail=0
ran=0

# 判定を1回走らせ、prod-scale.sh が呼ばれたかを返す。
# case のパターンを "(" 始まりにするのは bash 3.2 (macOS既定) 対策（既存テストと同じ）。
_run() {
  export PROJECT_NAME=soc-app
  export STUB_OVERRIDE="$1" STUB_DATES="$2" STUB_TODAY="$3" STUB_AT_START="$4"
  export SCALE_MARKER="$WORK/scaled"
  rm -f "$SCALE_MARKER"
  (
    cd "$WORK" || exit 1
    aws() {
      case "$*" in
        (*prod-uptime-override*)
          [ "$STUB_OVERRIDE" == "__error__" ] && return 255
          echo "$STUB_OVERRIDE" ;;
        (*prod-uptime-dates*) echo "$STUB_DATES" ;;
        (*) return 0 ;;
      esac
    }
    date() { echo "$STUB_TODAY"; }
    # shellcheck disable=SC1090
    source "$FRAGMENT" >/dev/null 2>&1
  )
  [ -f "$SCALE_MARKER" ] && echo "stopped" || echo "kept"
}

# want: stopped = 0へ戻す / kept = 起動状態に触らない
check() {
  local desc="$1" want="$2" override="$3" dates="$4" today="$5"
  local at_start="${6:-$3}"
  ran=$((ran + 1))
  local got
  got=$(_run "$override" "$dates" "$today" "$at_start")
  if [ "$got" == "$want" ]; then
    echo "ok   $desc"
  else
    echo "FAIL $desc (want=$want got=$got / override=$override dates=$dates today=$today)"
    fail=$((fail + 1))
  fi
}

DATES="2026-10-16,2026-12-24"

# 失効済みの on を起動指示として読まないこと（このテストを足した理由）。
check "on:<過去日> は失効扱いで 0 へ戻す"         stopped "on:2026-10-16" "$DATES" "2026-11-01"
check "on:<当日> は復帰日に達しているので戻す"     stopped "on:2026-11-01" "$DATES" "2026-11-01"
# 有効な起動指示を踏み潰さないこと。
check "on:<未来日> は起動指示として触らない"       kept    "on:2026-12-01" "$DATES" "2026-11-01"
check "復帰日なしの on は触らない"                 kept    "on"            "$DATES" "2026-11-01"
# 判断できないときは止めない（fail-closed）。
check "実在しない復帰日は失効させず触らない"       kept    "on:2026-09-31" "$DATES" "2026-11-01"
check "override を読めないときは触らない"          kept    "__error__"     "$DATES" "2026-11-01" "__unreadable__"
check "デプロイ中に override が変わったら触らない" kept    "auto"          "$DATES" "2026-11-01" "on"
# 日付リストと off/auto の既存挙動を壊さないこと。
check "auto かつ今日が起動日なら触らない"          kept    "auto"          "$DATES" "2026-12-24"
check "auto かつ今日が起動日でなければ戻す"        stopped "auto"          "$DATES" "2026-11-01"
check "off なら戻す"                               stopped "off"           "$DATES" "2026-11-01"

echo
if [ "$fail" -gt 0 ]; then
  echo "NG: $fail/$ran 件失敗"
  exit 1
fi
echo "OK: $ran 件すべて通過"
