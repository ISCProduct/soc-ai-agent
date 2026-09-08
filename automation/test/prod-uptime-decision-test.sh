#!/bin/bash
# prod-uptime-scheduler.yml の「今日 本番を起動するか」の判定をテストする。
#
# この判定を間違えると、展示会当日に本番が落ちたまま、または止めたはずの本番が
# 課金され続ける。ワークフロー内のシェルは実行するまで壊れていても気づけないため、
# 判定部分だけを切り出して実際に走らせる。
#
# カバー範囲は TODAY_JST の決定から DESIRED の確定まで。
# その先(RDSの起動待ち、ECSのdesired_count更新、RDS停止)は対象外なので、
# そちらを直したときにこのテストが緑でも保証にはならない。
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

# JST判定が UTC に化けると、JST 00:00〜09:00 が前日扱いになり、
# 展示会初日の朝9時まで本番が落ちたままになる。date をスタブするテストでは
# 検出できないので、抽出したコードを直接確認する。
if ! grep -q 'TZ=Asia/Tokyo' "$FRAGMENT"; then
  echo "NG: JST(TZ=Asia/Tokyo)での日付判定が失われています。" >&2
  exit 1
fi

fail=0
ran=0

# aws / date を差し替えて判定部分だけを走らせる。
# case のパターンを "(" 始まりにするのは bash 3.2 (macOS既定) が $( ) 内の case を
# 誤解釈するため。CI(bash 5)とローカルの両方で動かすのに必要。
_stub_and_source() {
  export PROJECT_NAME=soc-app
  export STUB_OVERRIDE="$1" STUB_DATES="$2" STUB_TODAY="$3"
  aws() {
    case "$*" in
      (*prod-uptime-override*)
        [ "$STUB_OVERRIDE" == "__missing__" ] && { echo "ParameterNotFound" >&2; return 255; }
        [ "$STUB_OVERRIDE" == "__error__" ] && { echo "AccessDeniedException" >&2; return 255; }
        echo "$STUB_OVERRIDE" ;;
      (*prod-uptime-dates*)
        [ "$STUB_DATES" == "__missing__" ] && { echo "ParameterNotFound" >&2; return 255; }
        [ "$STUB_DATES" == "__error__" ] && { echo "ThrottlingException" >&2; return 255; }
        echo "$STUB_DATES" ;;
      (*) return 0 ;;
    esac
  }
  date() { echo "$STUB_TODAY"; }
  # shellcheck disable=SC1090
  source "$FRAGMENT" >/dev/null 2>&1
}

# 起動するか(DESIRED)を確認する
run_case() {
  local name="$1" override="$2" dates="$3" today="$4" want="$5"
  local out
  ran=$((ran + 1))
  out=$( _stub_and_source "$override" "$dates" "$today"; echo "$DESIRED" )
  if [ "$out" == "$want" ]; then
    echo "ok   $name (DESIRED=$out)"
  else
    echo "NG   $name: DESIRED=$out, want=$want"
    fail=1
  fi
}

# AWSエラー(スロットリング/権限不足)を握り潰すと、その瞬間に本番が
# 無条件起動/停止しうる。DESIRED ではなく終了ステータスを見る。
run_exit_case() {
  local name="$1" override="$2" dates="$3" want="$4"
  local rc
  ran=$((ran + 1))
  ( _stub_and_source "$override" "$dates" "2026-09-08" )
  rc=$?
  if { [ "$want" == "nonzero" ] && [ "$rc" != 0 ]; } || { [ "$want" == "zero" ] && [ "$rc" == 0 ]; }; then
    echo "ok   $name (exit=$rc)"
  else
    echo "NG   $name: exit=$rc, want=$want"
    fail=1
  fi
}

# オーバーライドが読めない場合は日付リストの反映まで巻き添えにせず、
# 縮退して継続しつつ失敗フラグを立てる。
run_degraded_case() {
  local name="$1" dates="$2" want="$3"
  local out
  ran=$((ran + 1))
  out=$( _stub_and_source "__error__" "$dates" "2026-09-08"; echo "$DESIRED/$OVERRIDE_READ_FAILED" )
  if [ "$out" == "$want/1" ]; then
    echo "ok   $name (DESIRED=$want, 失敗フラグあり)"
  else
    echo "NG   $name: '$out', want '$want/1'"
    fail=1
  fi
}

TODAY="2026-09-08"
NOT_TODAY="2026-12-31"

run_case "override=on は日付リストに無くても起動する"      "on"          "2026-01-01"  "$TODAY" 1
run_case "override=off は日付リストに有っても停止する"      "off"         "$TODAY"      "$TODAY" 0
run_case "auto かつ今日が対象日なら起動する"                "auto"        "$TODAY"      "$TODAY" 1
run_case "auto かつ今日が対象外なら停止する"                "auto"        "$NOT_TODAY"  "$TODAY" 0
# 日付リストに今日を入れると auto でも on でも 1 になり区別できないため、
# 今日を含まないリストで「起動しないこと」を見る
run_case "override 未設定(初回)は auto として扱う"          "__missing__" "$NOT_TODAY"  "$TODAY" 0
run_case "未知の override は auto に倒す(勝手に起動しない)" "yes"         "$NOT_TODAY"  "$TODAY" 0
run_case "onで始まる別の値では起動しない(前方一致にしない)" "onn"         "$NOT_TODAY"  "$TODAY" 0
run_case "大文字ONも on として扱う(表示と実挙動を揃える)"   "ON"          "$NOT_TODAY"  "$TODAY" 1
run_case "前後に空白があっても on として扱う"               " on "        "$NOT_TODAY"  "$TODAY" 1
run_case "日付リスト未設定でも override=on なら起動する"    "on"          "__missing__" "$TODAY" 1
run_case "部分一致で誤判定しない(前方一致の別日)"           "auto"        "2026-09-081" "$TODAY" 0

# 日付リストのAWSエラーは握り潰してはいけない(誤って全サービスを0にしないため)
run_exit_case "日付リスト取得のAWSエラーで失敗する"         "auto"        "__error__"   nonzero
run_exit_case "正常系は0で終了する"                         "auto"        "$TODAY"      zero

run_degraded_case "オーバーライドが読めなければ日付リストに縮退する" "$TODAY"     1
run_degraded_case "縮退時も今日が対象外なら起動しない"               "$NOT_TODAY" 0

# 縮退時にジョブを落とす処理は抽出範囲の外にあるため静的に確認する
if awk '/if \[ -n "\$OVERRIDE_READ_FAILED" \]/,/fi/' "$WORKFLOW" | grep -q 'exit 1'; then
  echo "ok   オーバーライド読み取り失敗でジョブを落とす(静的確認)"
else
  echo "NG   オーバーライド読み取り失敗でジョブを落とす処理が見つかりません"
  fail=1
fi
ran=$((ran + 1))

# 関数名の打ち間違い等でケースが丸ごと実行されないまま PASS になるのを防ぐ
EXPECTED_CASES=16
if [ "$ran" != "$EXPECTED_CASES" ]; then
  echo "NG   実行されたケースが $ran 件。想定は $EXPECTED_CASES 件(ケースの追加時は EXPECTED_CASES も更新すること)"
  fail=1
fi

if [ "$fail" != 0 ]; then
  echo "FAILED"
  exit 1
fi
echo "PASS ($ran ケース)"
