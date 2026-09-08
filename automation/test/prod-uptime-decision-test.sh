#!/bin/bash
# prod-uptime-scheduler.yml の「今日 本番を起動するか」の判定をテストする。
#
# この判定を間違えると、展示会当日に本番が落ちたまま、または止めたはずの本番が
# 課金され続ける。ワークフロー内のシェルは実行するまで壊れていても気づけないため、
# 判定部分だけを切り出して実際に走らせる。
#
# 使い方: ./automation/test/prod-uptime-decision-test.sh

set -uo pipefail

WORKFLOW="$(dirname "$0")/../../.github/workflows/prod-uptime-scheduler.yml"
FRAGMENT=$(mktemp)
trap 'rm -f "$FRAGMENT"' EXIT

# 判定ロジックだけを抜き出す（TODAY_JST の決定から DESIRED 確定まで）。
# 抜き出せなければ「テストしたつもり」になるので必ず失敗させる。
awk '/^          TODAY_JST=/,/^          esac$/' "$WORKFLOW" | sed 's/^          //' > "$FRAGMENT"
if ! grep -q "^esac$" "$FRAGMENT" || ! grep -q "prod-uptime-override" "$FRAGMENT"; then
  echo "NG: 判定ロジックを抽出できませんでした。prod-uptime-scheduler.yml の構造変更を確認してください。" >&2
  exit 1
fi

fail=0

# aws CLI を差し替え、SSM の応答だけを再現する。
run_case() {
  local name="$1" override="$2" dates="$3" today="$4" want="$5"
  local out
  out=$(
    export PROJECT_NAME=soc-app
    export STUB_OVERRIDE="$override" STUB_DATES="$dates" STUB_TODAY="$today"
    aws() {
      # パターンを "(" 始まりにするのは bash 3.2 (macOS既定) が $( ) 内の case を
      # 誤解釈するため。CI(bash 5)とローカルの両方で動かすのに必要。
      case "$*" in
        (*prod-uptime-override*)
          [ "$STUB_OVERRIDE" == "__missing__" ] && { echo "ParameterNotFound" >&2; return 255; }
          echo "$STUB_OVERRIDE" ;;
        (*prod-uptime-dates*)
          [ "$STUB_DATES" == "__missing__" ] && { echo "ParameterNotFound" >&2; return 255; }
          echo "$STUB_DATES" ;;
        (*) return 0 ;;
      esac
    }
    date() { echo "$STUB_TODAY"; }
    export -f aws date 2>/dev/null || true
    # shellcheck disable=SC1090
    source "$FRAGMENT" >/dev/null 2>&1
    echo "$DESIRED"
  )
  if [ "$out" == "$want" ]; then
    echo "ok   $name (DESIRED=$out)"
  else
    echo "NG   $name: DESIRED=$out, want=$want"
    fail=1
  fi
}

TODAY="2026-09-08"

run_case "override=on は日付リストに無くても起動する"      "on"          "2026-01-01"          "$TODAY" 1
run_case "override=off は日付リストに有っても停止する"      "off"         "$TODAY"              "$TODAY" 0
run_case "auto かつ今日が対象日なら起動する"                "auto"        "$TODAY,2026-12-31"   "$TODAY" 1
run_case "auto かつ今日が対象外なら停止する"                "auto"        "2026-12-31"          "$TODAY" 0
run_case "override 未設定(初回)は auto として扱う"          "__missing__" "$TODAY"              "$TODAY" 1
run_case "未知の override は auto に倒す(勝手に起動しない)" "yes"         "2026-12-31"          "$TODAY" 0
run_case "日付リスト未設定でも override=on なら起動する"    "on"          "__missing__"         "$TODAY" 1
run_case "部分一致で誤判定しない(前方一致の別日)"           "auto"        "2026-09-081"         "$TODAY" 0

if [ "$fail" != 0 ]; then
  echo "FAILED"
  exit 1
fi
echo "PASS"
