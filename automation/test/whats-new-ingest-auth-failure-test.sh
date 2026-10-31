#!/bin/bash
# 更新情報の取り込み(whats-new/ingest)で「鍵の問題」が成功扱いにならないことを固定する。
#
# このジョブは continue-on-error なので、失敗を自分で error へ昇格させないと緑で終わる。
# 「停止中で届かない」は放っておけば次回で回収されるが、次の3つは放置しても直らない:
#   - 送った鍵が違う / 未登録 (401/403)
#   - Secrets Manager から鍵を読めない (IAM権限不足・シークレット未作成)
#   - バックエンドに SERVICE_SECRET が入っていない (503。設定不備の方)
# 503 はステータスだけでは判断できない。api.* は ALB 直結(prod/main.tf の
# aws_route53_record.backend)で、ALB も healthy target ゼロなら 503 を返すため。
# 設定不備の方は EchoStaticSecretAuth が本文へ SERVICE_SECRET_NOT_CONFIGURED を入れる。
# これらを warning で流すと、更新情報が永久に反映されないまま誰も気づかない(#1290)。
# 鍵が ADMIN_SECRET から SERVICE_SECRET へ分離された(#1158)ことで、
# 「リポジトリシークレットの登録漏れ」という新しい失敗の仕方も増えている。
#
# 使い方: ./automation/test/whats-new-ingest-auth-failure-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

fail=0
checked=0

# 本文に入るコードは Backend 側が正。ワークフローの grep と食い違ったら検査が無意味になる。
NOT_CONFIGURED_CODE=$(grep -oE '"SERVICE_SECRET_NOT_CONFIGURED"' "$ROOT/Backend/internal/routes/echo_adapter.go" | head -1 | tr -d '"')
if [ -z "$NOT_CONFIGURED_CODE" ]; then
  echo "FAIL echo_adapter.go が設定不備の503にコードを入れていない（CIが停止中と区別できない）"
  exit 1
fi
echo "ok   設定不備の503のコード: $NOT_CONFIGURED_CODE"

check_step() {
  local label="$1" body="$2" bad=0
  checked=$((checked + 1))

  # HTTPステータスを取らずに curl -sf で済ませていると、401/403 と「届かない」が区別できない。
  if grep -q "curl -sf" <<< "$body"; then
    echo "FAIL $label: ingest を curl -sf で呼んでいる（401/403 と停止中を区別できない）"
    fail=$((fail + 1)); bad=1
  fi
  if ! grep -q '%{http_code}' <<< "$body"; then
    echo "FAIL $label: ingest の HTTP ステータスを取得していない"
    fail=$((fail + 1)); bad=1
  fi

  # 401/403 は error へ昇格させること。
  if ! grep -q '401|403)' <<< "$body"; then
    echo "FAIL $label: 401/403 を個別に扱っていない"
    fail=$((fail + 1))
    return
  fi
  # 401|403 の分岐の中に ::error:: があること（コメント込みでも数行以内に出る書き方に揃えてある）。
  if ! grep -A6 '401|403)' <<< "$body" | grep -q '::error::'; then
    echo "FAIL $label: 401/403 を ::error:: にしていない（warning だと誰も気づかない）"
    fail=$((fail + 1)); bad=1
  fi

  # 設定不備の503を本文で見分けていること。ステータスだけで一律 warning にすると、
  # SERVICE_SECRET が未注入のまま更新情報が永久に反映されない。
  if ! grep -q "$NOT_CONFIGURED_CODE" <<< "$body"; then
    echo "FAIL $label: 設定不備の503($NOT_CONFIGURED_CODE)を本文で見分けていない"
    fail=$((fail + 1)); bad=1
  fi

  # Secrets Manager から鍵を読む経路があるなら、読めなかった場合も error にすること。
  if grep -q "get-secret-value" <<< "$body"; then
    if ! grep -B2 -A6 "get-secret-value" <<< "$body" | grep -q '::error::'; then
      # 読み取り失敗の分岐はブロック後半にあることもあるので、ブロック全体も見る。
      if ! grep -q '::error::.*読めませんでした' <<< "$body"; then
        echo "FAIL $label: Secrets Manager から鍵を読めなかった場合が ::error:: になっていない"
        fail=$((fail + 1)); bad=1
      fi
    fi
  fi
  [ "$bad" = "0" ] && echo "ok   $label: 鍵の問題を error へ昇格させている"
  return 0
}

for wf in "$ROOT/.github/workflows/deployment.yml" "$ROOT/.github/workflows/prod-uptime-scheduler.yml"; do
  [ -f "$wf" ] || continue
  wfname=$(basename "$wf")
  # ステップ単位（行頭 "      - name:" 区切り）に切って、ingest を呼ぶものだけ見る。
  while read -r start; do
    end=$(awk -v s="$start" 'NR > s && /^      - name: /{print NR - 1; exit}' "$wf")
    [ -n "$end" ] || end=$(wc -l < "$wf")
    body=$(sed -n "${start},${end}p" "$wf")
    grep -q "whats-new/ingest" <<< "$body" || continue
    name=$(sed -n "${start}p" "$wf" | sed 's/^ *- name: //')
    check_step "$wfname / $name" "$body"
  done < <(grep -n '^      - name: ' "$wf" | cut -d: -f1)
done

if [ "$checked" -eq 0 ]; then
  echo "FAIL ingest を呼ぶステップが1件も見つからない（抽出条件が壊れている）"
  fail=$((fail + 1))
fi

echo "検査した ingest 呼び出しステップ: $checked 件"
if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
