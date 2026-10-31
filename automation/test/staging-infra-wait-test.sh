#!/bin/bash
# staging デプロイの "Wait for staging infra ready" の待機判定をテストする。
#
# 前回デプロイの auto-stop（1時間後に ASG desired を 0 にする）が、次のデプロイと
# 重なると起動キックの後に desired を 0 へ戻す。desired=0 の直後でも終了処理が
# 始まるまで EC2 は running のままなので、state だけを見て待機を抜けると
# 「これから落とされる台」へデプロイしてしまう（SOCAIAGENT-436 / #1693）。
# 2026-10-08 07:19 の develop デプロイはこれで失敗した。
#
# カバー範囲は待機ループの判定（抜ける条件・タイムアウト時の成否・上げ直しの回数）。
# auto-stop 側の concurrency によるキャンセルや、実際の AWS の挙動は対象外。
#
# 使い方: ./automation/test/staging-infra-wait-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows/deployment.yml"

WORK=$(mktemp -d)
FRAGMENT="$WORK/fragment.sh"
trap 'rm -rf "$WORK"' EXIT

# 待機ステップの run: 本体だけを抜き出す。
# 抜き出せなければ「テストしたつもり」になるので必ず失敗させる。
START=$(grep -n '^      - name: Wait for staging infra ready$' "$WF" | head -1 | cut -d: -f1)
if [ -z "$START" ]; then
  echo "NG: 待機ステップが見つかりません。deployment.yml の構造変更を確認してください。" >&2
  exit 1
fi
# YAML のキー行（run: 等）を含めると source した時点で構文エラーになり、
# 全ケースが「判定前に落ちた」結果になる。run: | の次行から、
# 本体のインデント(10スペース)が切れるまでを取る。
awk -v s="$START" '
  NR <= s { next }
  !inbody && /^        run: \|$/ { inbody = 1; next }
  !inbody { next }
  /^[[:space:]]*$/ { print ""; next }
  /^          / { sub(/^          /, ""); print; next }
  { exit }
' "$WF" > "$FRAGMENT"

if ! grep -q 'describe-db-instances' "$FRAGMENT" || ! grep -q 'describe-instances' "$FRAGMENT"; then
  echo "NG: 待機ロジックを抽出できませんでした。" >&2
  exit 1
fi
if grep -q '{{' "$FRAGMENT"; then
  echo "NG: 展開されていない GitHub Actions 式が残っています。" >&2
  exit 1
fi
if grep -qE '^ *(if|run|name|env|uses|with):' "$FRAGMENT"; then
  echo "NG: シェル以外の行(YAMLキー)が混ざっています。" >&2
  exit 1
fi

fail=0
ran=0

# 判定を1回走らせ、"ok"（待機を抜けて後続へ進む）か "ng"（ステップを落とす）を返す。
# 上げ直し(update-auto-scaling-group)の回数は $WORK/raises に残す。
#
# STUB_RDS        : describe-db-instances が返すステータス（空白区切りの列。尽きたら最後の値）
# STUB_DESIRED    : DesiredCapacity の初期値（__error__ でAPIエラー）
# STUB_STICKY     : yes なら上げ直しても desired が 0 のまま（auto-stop が勝ち続ける状況）
# STUB_EC2        : describe-instances が返す state の列（__error__ でAPIエラー）
#
# case のパターンを "(" 始まりにするのは bash 3.2 (macOS既定) 対策（既存テストと同じ）。
_run() {
  export PROJECT_NAME=soc-app
  export STUB_RDS="$1" STUB_DESIRED="$2" STUB_STICKY="$3" STUB_EC2="$4"
  echo "$STUB_DESIRED" > "$WORK/desired"
  echo 0 > "$WORK/raises"
  echo 0 > "$WORK/rds_n"
  echo 0 > "$WORK/ec2_n"
  echo 0 > "$WORK/rds_starts"
  (
    # 列から n 番目（尽きたら最後）を取り出す。
    _nth() {
      local n="$1"; shift
      local items=("$@")
      [ "$n" -gt "${#items[@]}" ] && n="${#items[@]}"
      echo "${items[$((n - 1))]}"
    }
    aws() {
      local n
      case "$*" in
        (*rds*describe-db-instances*)
          n=$(( $(cat "$WORK/rds_n") + 1 )); echo "$n" > "$WORK/rds_n"
          _nth "$n" $STUB_RDS ;;
        (*rds*start-db-instance*)
          echo $(( $(cat "$WORK/rds_starts") + 1 )) > "$WORK/rds_starts" ;;
        (*describe-auto-scaling-groups*)
          local d
          d=$(cat "$WORK/desired")
          [ "$d" == "__error__" ] && return 255
          echo "$d" ;;
        (*update-auto-scaling-group*)
          echo $(( $(cat "$WORK/raises") + 1 )) > "$WORK/raises"
          [ "$STUB_STICKY" == "yes" ] || echo 1 > "$WORK/desired" ;;
        (*ec2*describe-instances*)
          n=$(( $(cat "$WORK/ec2_n") + 1 )); echo "$n" > "$WORK/ec2_n"
          local s
          s=$(_nth "$n" $STUB_EC2)
          [ "$s" == "__error__" ] && return 255
          echo "$s" ;;
        (*) return 0 ;;
      esac
    }
    # 実時間を待たない。GitHub Actions の既定シェルと同じ -e -o pipefail で走らせる
    # （待機ループ内の一時的なAPIエラーでステップごと落ちないかもここで見る）。
    # -u は外側の set -u が継いでしまうため明示的に外す。本番より厳しい条件で
    # 落とすと、ワークフローでは起きない失敗をテストだけが報告することになる。
    sleep() { :; }
    set -eo pipefail
    set +u
    # shellcheck disable=SC1090
    source "$FRAGMENT"
  ) >/dev/null 2>&1 && echo "ok" || echo "ng"
}

# want: ok = 待機を抜けてデプロイへ進む / ng = ステップを落とす
check() {
  local desc="$1" want="$2" want_raises="$3"
  local rds="$4" desired="$5" sticky="$6" ec2="$7"
  ran=$((ran + 1))
  local got raises
  got=$(_run "$rds" "$desired" "$sticky" "$ec2")
  raises=$(cat "$WORK/raises")
  if [ "$got" == "$want" ] && [ "$raises" == "$want_raises" ]; then
    echo "ok   $desc"
  else
    echo "FAIL $desc (want=$want/$want_raises got=$got/$raises"\
" rds=$rds desired=$desired sticky=$sticky ec2=$ec2)"
    fail=$((fail + 1))
  fi
}

AVAIL="available"

# 本題: desired が 0 のまま戻らないなら、running な台があってもデプロイへ進まない。
# これを許すと「これから auto-stop に落とされる台」へ配って、デプロイ中に落ちる。
check "desired が 0 のままなら running でも進まない" ng 20 "$AVAIL" 0 yes "running"
# 上げ直しが効けば進む。desired が戻った後に1回だけ上げ、二重に叩かない。
check "desired=0 を上げ直せたら進む"                 ok 1  "$AVAIL" 0 no  "running running"
# 既に起動していれば上げ直さない。
check "desired=1 で running なら即進む"              ok 0  "$AVAIL" 1 no  "running"
check "EC2 が後から running になれば進む"            ok 0  "$AVAIL" 1 no  "None None running"
check "EC2 が現れなければ落とす"                     ng 0  "$AVAIL" 1 no  "None"
# ASG を読めないときに勝手に上げ直さない（ASGを読めない＝0とは限らない）。
check "desired を読めなくても running なら進む"      ok 0  "$AVAIL" __error__ no "running"
# 一時的なAPIエラーで待機ループごと落ちない（自己修復のための待機が無意味になる）。
check "EC2 の一時APIエラーは次の周回で読み直す"      ok 0  "$AVAIL" 1 no  "__error__ running"
# RDS 側の終了条件を壊さないこと。
check "RDS が stopped のままなら落とす"              ng 0  "stopped" 1 no "running"
check "RDS が後から available になれば進む"          ok 0  "starting starting available" 1 no "running"

echo
if [ "$fail" -gt 0 ]; then
  echo "NG: $fail/$ran 件失敗"
  exit 1
fi
echo "OK: $ran 件すべて通過"
