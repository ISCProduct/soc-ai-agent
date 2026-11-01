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
# ジョブの作りも見る:
#   - environment: を宣言したジョブで secrets.SERVICE_SECRET を参照しないこと。
#     GitHub では environment secret が同名のリポジトリシークレットを上書きするため、
#     production 環境に SERVICE_SECRET が登録された時点で staging へ本番の値が飛ぶ。
#   - secrets.SERVICE_SECRET を使うステップは、未登録(空)なら error で落とすこと。
#     staging が停止中だと警告だけで終わり、設定漏れに気づけない。
#
# 使い方: ./automation/test/whats-new-ingest-auth-failure-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# コメント行を落とす。説明コメントに同じ語が出るだけで検査が通ってしまうと、
# 実際の判定を消しても落ちない「固めたつもり」のテストになる（#1909 レビュー指摘）。
# 落とすのは行まるごとのコメントだけ。行末コメントは残るが、ここで突き合わせるのは
# いずれもコード構造の一部（grep のコマンド行・HCL の式）なので行末コメントには現れない。
strip_comments() { grep -vE '^[[:space:]]*#' || true; }

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
  local label="$1" body="$2" bad=0 code
  checked=$((checked + 1))
  # 以降の検査はコメントを落とした行だけを見る。
  code=$(strip_comments <<< "$body")

  # HTTPステータスを取らずに curl -sf で済ませていると、401/403 と「届かない」が区別できない。
  if grep -q "curl -sf" <<< "$code"; then
    echo "FAIL $label: ingest を curl -sf で呼んでいる（401/403 と停止中を区別できない）"
    fail=$((fail + 1)); bad=1
  fi
  if ! grep -q '%{http_code}' <<< "$code"; then
    echo "FAIL $label: ingest の HTTP ステータスを取得していない"
    fail=$((fail + 1)); bad=1
  fi

  # 401/403 は error へ昇格させること。
  # 1ステップに本番と staging の2呼び出しが入ることがあるので、**すべての分岐**を見る。
  # 「どこかに ::error:: がある」で済ませると、片方を warning へ下げても通ってしまう。
  local arms
  arms=$(grep -c '401|403)' <<< "$code")
  if [ "$arms" -eq 0 ]; then
    echo "FAIL $label: 401/403 を個別に扱っていない"
    fail=$((fail + 1))
    return 0
  fi
  local arms_with_error
  # 分岐は `;;` までを1件として数える。正規表現のエスケープで取りこぼさないよう
  # index() で素の文字列として探す（awk の実装差で `\|` の扱いが変わる）。
  arms_with_error=$(awk '
    index($0, "401|403)") { inarm = 1; found = 0; next }
    inarm {
      if (index($0, "::error::")) found = 1
      if (index($0, ";;")) { if (found) n++; inarm = 0 }
    }
    END { if (inarm && found) n++; print n + 0 }
  ' <<< "$code")
  if [ "$arms_with_error" != "$arms" ]; then
    echo "FAIL $label: 401/403 の分岐 $arms 件のうち ::error:: にしているのは $arms_with_error 件（warning だと誰も気づかない）"
    fail=$((fail + 1)); bad=1
  fi

  # 設定不備の503を本文で見分けていること。ステータスだけで一律 warning にすると、
  # SERVICE_SECRET が未注入のまま更新情報が永久に反映されない。
  # ingest の curl が本文を落とす先（-o "$var"）を列挙し、**その変数ごとに**
  # 本文判定があることを見る。ステップ単位で「どこかにあればOK」にすると、
  # 本番側の判定だけ消しても staging 側の判定で通ってしまう。
  local respvars v
  respvars=$(grep -oE -- '-o +"\$[a-z_]+"' <<< "$code" | grep -oE '[a-z_]+' | grep -v '^o$' | sort -u)
  if [ -z "$respvars" ]; then
    echo "FAIL $label: ingest のレスポンス本文を保存していない（設定不備の503を見分けられない）"
    fail=$((fail + 1)); bad=1
  fi
  for v in $respvars; do
    if ! grep -qE "grep +(-[a-zA-Z]+ +)*$NOT_CONFIGURED_CODE +\"\\\$$v\"" <<< "$code"; then
      echo "FAIL $label: \$$v の本文で設定不備の503($NOT_CONFIGURED_CODE)を見分けていない"
      fail=$((fail + 1)); bad=1
    fi
  done

  # Secrets Manager から鍵を読む経路があるなら、読めなかった場合も error にすること。
  if grep -q "get-secret-value" <<< "$code"; then
    if ! grep -B2 -A6 "get-secret-value" <<< "$code" | grep -q '::error::'; then
      # 読み取り失敗の分岐はブロック後半にあることもあるので、ブロック全体も見る。
      if ! grep -q '::error::.*読めませんでした' <<< "$code"; then
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

# --- ジョブの作り: environment を持つジョブは secrets.SERVICE_SECRET を参照しない ---
WF="$ROOT/.github/workflows/deployment.yml"
jobs_checked=0
while IFS=$'\t' read -r job has_env has_secret; do
  jobs_checked=$((jobs_checked + 1))
  if [ "$has_env" = "1" ] && [ "$has_secret" = "1" ]; then
    echo "FAIL $job: environment を宣言したジョブが secrets.SERVICE_SECRET を参照している"
    echo "     (environment secret が同名のリポジトリシークレットを上書きするため、"
    echo "      その環境に SERVICE_SECRET が登録されると別環境の値を送って黙って止まる)"
    fail=$((fail + 1))
  fi
done < <(awk '
  /^  [a-z0-9_-]+:[[:space:]]*$/ {
    if (job != "") printf "%s\t%d\t%d\n", job, env, sec
    job = $1; sub(/:$/, "", job); env = 0; sec = 0; next
  }
  job != "" {
    if ($0 ~ /^[[:space:]]*#/) next
    if ($0 ~ /^    environment:/) env = 1
    if ($0 ~ /secrets\.SERVICE_SECRET/) sec = 1
  }
  END { if (job != "") printf "%s\t%d\t%d\n", job, env, sec }
' "$WF")
if [ "$jobs_checked" -eq 0 ]; then
  echo "FAIL deployment.yml からジョブを1件も抽出できない（検査条件が壊れている）"
  fail=$((fail + 1))
else
  echo "ok   environment 付きジョブが secrets.SERVICE_SECRET を参照していない（$jobs_checked ジョブ）"
fi

# --- secrets.SERVICE_SECRET を使うステップは未登録(空)なら error で落とす ---
secret_steps=0
while read -r start; do
  end=$(awk -v s="$start" 'NR > s && /^      - name: /{print NR - 1; exit}' "$WF")
  [ -n "$end" ] || end=$(wc -l < "$WF")
  body=$(sed -n "${start},${end}p" "$WF" | strip_comments)
  grep -q 'secrets.SERVICE_SECRET' <<< "$body" || continue
  secret_steps=$((secret_steps + 1))
  name=$(sed -n "${start}p" "$WF" | sed 's/^ *- name: //')
  if ! grep -q 'if \[ -z "\$SERVICE_SECRET" \]' <<< "$body"; then
    echo "FAIL $name: SERVICE_SECRET が未登録(空)のときに落としていない"
    fail=$((fail + 1))
  elif ! grep -A3 'if \[ -z "\$SERVICE_SECRET" \]' <<< "$body" | grep -q '::error::'; then
    echo "FAIL $name: SERVICE_SECRET 未登録を ::error:: にしていない"
    fail=$((fail + 1))
  else
    echo "ok   $name: SERVICE_SECRET 未登録で落とす"
  fi
done < <(grep -n '^      - name: ' "$WF" | cut -d: -f1)
if [ "$secret_steps" -eq 0 ]; then
  echo "FAIL secrets.SERVICE_SECRET を使うステップが1件も見つからない（検査条件が壊れている）"
  fail=$((fail + 1))
fi

echo "検査した ingest 呼び出しステップ: $checked 件"
if [ "$fail" -gt 0 ]; then
  echo "FAILED ($fail)"
  exit 1
fi
echo "PASSED"
