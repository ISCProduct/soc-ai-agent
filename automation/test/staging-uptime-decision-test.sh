#!/bin/bash
# staging-uptime-scheduler.yml の「ステージングを起動するか」の判定をテストする。
#
# 判定を間違えると、開発中にステージングが理由不明で落ちる（またはコスト削減の
# ために止めたはずが上がったまま）。ワークフロー内のシェルは実行するまで
# 壊れていても気づけないため、判定部分だけを切り出して実際に走らせる。
#
# カバー範囲は SSM の値の解釈から MIN / DESIRED の確定まで。
# その先(update-auto-scaling-group の実行)は対象外。
#
# 使い方: ./automation/test/staging-uptime-decision-test.sh

set -uo pipefail

WORKFLOW="$(dirname "$0")/../../.github/workflows/staging-uptime-scheduler.yml"
FRAGMENT=$(mktemp)
STUB_DIR=$(mktemp -d)
trap 'rm -rf "$FRAGMENT" "$STUB_DIR"' EXIT

# 判定ロジックだけを抜き出す（PARAM の決定から DESIRED 確定まで）。
# 抜き出せなければ「テストしたつもり」になるので必ず失敗させる。
awk '/^          PARAM=/,/^          echo "state=/' "$WORKFLOW" | sed 's/^          //' > "$FRAGMENT"
if ! grep -q 'echo "state=' "$FRAGMENT" || ! grep -q "ParameterNotFound" "$FRAGMENT"; then
  echo "NG: 判定ロジックを抽出できませんでした。staging-uptime-scheduler.yml の構造変更を確認してください。" >&2
  exit 1
fi

# ASG は desired < min を受け付けない。min を動かさない実装に退行していないか、
# 抽出したコードを直接確認する（スタブでは検出できない）。
if ! grep -q "MIN=0" "$FRAGMENT" || ! grep -q "MIN=1" "$FRAGMENT"; then
  echo "NG: min_size を動かしていません。desired だけ 0 にすると ValidationError になります。" >&2
  exit 1
fi

# 適用部分（ASG と RDS を実際に動かすところ）も抜き出す。
# 起動時にASGを先に上げると backend がDB接続に失敗し続け、ELBヘルスチェックが
# 通らないままASGにインスタンスを置換され続けるため、順序まで検証する。
APPLY=$(mktemp)
trap 'rm -rf "$FRAGMENT" "$STUB_DIR" "$APPLY"' EXIT
awk '/^          apply_asg\(\) \{/,/^      - name:/' "$WORKFLOW" | sed '$d' | sed 's/^          //' > "$APPLY"
if ! grep -q "wait_rds_available" "$APPLY" || ! grep -q "stop-db-instance" "$APPLY"; then
  echo "NG: 適用ロジックを抽出できませんでした。staging-uptime-scheduler.yml の構造変更を確認してください。" >&2
  exit 1
fi

pass=0
fail=0

# aws コマンドのスタブ。STUB_VALUE / STUB_ERR で応答を作る。
cat > "$STUB_DIR/aws" <<'STUB'
#!/bin/bash
# 適用部分のテスト用: 呼び出し順を CALLS に記録し、RDS の状態は STUB_DB で作る。
case "$*" in
  *"update-auto-scaling-group"*) [ -n "${CALLS:-}" ] && echo "ASG" >> "$CALLS"; exit 0 ;;
  *"start-db-instance"*)         [ -n "${CALLS:-}" ] && echo "RDS_START" >> "$CALLS"; echo "available" > "${STUB_DB_FILE:-/dev/null}"; exit 0 ;;
  *"stop-db-instance"*)          [ -n "${CALLS:-}" ] && echo "RDS_STOP" >> "$CALLS"; exit 0 ;;
  *"describe-db-instances"*)     cat "${STUB_DB_FILE:-/dev/null}"; exit 0 ;;
esac
if [ -n "${STUB_ERR:-}" ]; then
  echo "$STUB_ERR" >&2
  exit 255
fi
echo "${STUB_VALUE:-on}"
STUB
chmod +x "$STUB_DIR/aws"

# wait_rds_available のリトライで待たされないようにする
printf '#!/bin/bash\nexit 0\n' > "$STUB_DIR/sleep"
chmod +x "$STUB_DIR/sleep"
export PATH="$STUB_DIR:$PATH"

run_case() {
  local name="$1" value="$2" err="$3" want_min="$4" want_desired="$5" want_exit="${6:-0}"
  local out rc
  out=$(STUB_VALUE="$value" STUB_ERR="$err" PROJECT_NAME=soc-app bash "$FRAGMENT" 2>&1)
  rc=$?

  if [ "$rc" != "$want_exit" ]; then
    echo "FAIL $name (終了コード $rc, want $want_exit)"; echo "  $out"; fail=$((fail + 1)); return
  fi
  if [ "$want_exit" != "0" ]; then
    echo "ok   $name"; pass=$((pass + 1)); return
  fi
  if [[ "$out" != *"min_size=$want_min desired_capacity=$want_desired"* ]]; then
    echo "FAIL $name"; echo "  期待: min_size=$want_min desired_capacity=$want_desired"; echo "  実際: $out"
    fail=$((fail + 1)); return
  fi
  echo "ok   $name"; pass=$((pass + 1))
}

# --- 明示的な指定 ---
run_case "on なら起動"            "on"    "" 1 1
run_case "off なら停止"           "off"   "" 0 0
run_case "大文字 OFF も停止"      "OFF"   "" 0 0
run_case "前後の空白は無視"       "  on " "" 1 1

# --- 判断できないときは止めない ---
# off に倒すと、パラメータ未作成や手書きミスでステージングが理由不明に落ちる
run_case "パラメータ未作成は起動" "" "An error occurred (ParameterNotFound) when calling GetParameter" 1 1
run_case "未知の値は起動"         "maybe" "" 1 1
run_case "auto と書かれても起動"  "auto"  "" 1 1
run_case "空文字は起動"           ""      "" 1 1

# --- 読めない場合は失敗させる ---
# 「読めなかったから止めた」は原因が追えない事故になる
run_case "権限エラーは失敗させる" "" "An error occurred (AccessDeniedException) when calling GetParameter" - - 1

# --- 適用の順序 ---
# 起動: DBが available になってからASGを上げる
# 停止: ASGを落としてからDBを止める（逆にするとアプリがDB無しで動く時間ができる）
run_apply_case() {
  local name="$1" state="$2" db="$3" want="$4"
  local calls db_file got min desired

  calls=$(mktemp); db_file=$(mktemp)
  echo "$db" > "$db_file"
  if [ "$state" = "off" ]; then min=0; desired=0; else min=1; desired=1; fi

  CALLS="$calls" STUB_DB_FILE="$db_file" STATE="$state" MIN="$min" DESIRED="$desired" \
    STAGING_ASG_NAME=soc-stg-app STAGING_DB_ID=soc-stg-mysql \
    bash "$APPLY" >/dev/null 2>&1

  got=$(tr '\n' ' ' < "$calls" | sed 's/ *$//')
  rm -f "$calls" "$db_file"

  if [ "$got" != "$want" ]; then
    echo "FAIL $name"; echo "  期待: [$want]"; echo "  実際: [$got]"; fail=$((fail + 1)); return
  fi
  echo "ok   $name"; pass=$((pass + 1))
}

run_apply_case "起動時はDBを先に上げる"     on  stopped   "RDS_START ASG"
run_apply_case "DB稼働中はASGだけ"          on  available "ASG"
run_apply_case "停止時はASGを先に落とす"    off available "ASG RDS_STOP"
run_apply_case "DB停止済みならstopしない"   off stopped   "ASG"

EXPECTED_CASES=13
echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
if [ "$pass" -ne "$EXPECTED_CASES" ]; then
  echo "NG: 実行ケース数が $pass 件です（期待 $EXPECTED_CASES 件）。ケースが素通りしていないか確認してください。" >&2
  exit 1
fi
echo "PASS ($pass ケース)"
