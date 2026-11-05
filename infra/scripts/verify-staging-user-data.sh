#!/bin/bash
# staging の user_data が .env を正しく組み立てるかをローカルで検証する(#1158)。
#
# staging の起動経路は手元では動かせない（EC2 / cloud-init / 実シークレットが要る）。
# ここで検証するのは user_data のシェルロジックだけ:
#   1. .env が 0600 で作られ、シークレット行が入る
#   2. 値は Secrets Manager の JSON から来る（user_data には平文が無い）
#   3. 取得に失敗したら .env を書かずに異常終了する（空の値で起動しない）
#   4. シークレットが set -x のトレース(/var/log/cloud-init-output.log)に出ない
#
# 実際の起動確認は terraform apply 後に staging 上で行うこと（PR本文の手順）。
#
# 使い方: bash infra/scripts/verify-staging-user-data.sh
set -euo pipefail

TPL="$(cd "$(dirname "$0")/.." && pwd)/terraform/environments/staging/app_user_data.sh.tftpl"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/opt/app"

# user_data の「シークレット取得〜.env 生成」部分だけをテンプレートから切り出し、
# terraform の ${...} をダミー値へ、/opt/app を作業ディレクトリへ置き換える。
python3 - "$TPL" "$WORK" <<'PY'
import re, sys, pathlib
tpl, work = sys.argv[1], sys.argv[2]
src = pathlib.Path(tpl).read_text()
start = src.index("# --- シークレットの取得")
end = src.index("cat > /opt/app/docker-compose.yml")
block = src[start:end]
sub = {
    "aws_region": "ap-northeast-1",
    "app_env_secret_id": "arn:aws:secretsmanager:ap-northeast-1:1:secret:soc-stg/app-env-x",
    "db_secret_id": "arn:aws:secretsmanager:ap-northeast-1:1:secret:soc-stg/db-x",
}
block = re.sub(r"\$\{([a-z0-9_]+)\}", lambda m: sub.get(m.group(1), "DUMMY"), block)
pathlib.Path(work + "/run.sh").write_text("#!/bin/bash\nset -eux\n" + block.replace("/opt/app", work + "/opt/app"))
PY

# aws CLI のスタブ。$1 のモードで返す内容を変える。
write_aws_stub() {
  cat > "$WORK/bin/aws" <<EOS
#!/bin/bash
mode="$1"
prev=""
for a in "\$@"; do [ "\$prev" = "--secret-id" ] && sid="\$a"; prev="\$a"; done
case "\$mode" in
  ok)
    case "\$sid" in
      *app-env*) echo '{"openai_api_key":"sk-dummy","admin_secret":"adm\$ecret","sentry_dsn":"","bff_internal_token":"tok","token_encryption_key":"dead"}';;
      *) echo '{"host":"h","port":3306,"dbname":"app_db","username":"admin","password":"p@ss word"}';;
    esac;;
  denied) echo "AccessDeniedException" >&2; exit 254;;
  empty) ;;
  nopassword)
    case "\$sid" in
      *app-env*) echo '{"openai_api_key":"sk-dummy"}';;
      *) echo '{"host":"h"}';;
    esac;;
esac
EOS
  chmod +x "$WORK/bin/aws"
}

fail() {
  echo "NG: $1" >&2
  exit 1
}

ENV_FILE="$WORK/opt/app/.env"

# --- 1. 正常系 ---
write_aws_stub ok
rm -f "$ENV_FILE"
PATH="$WORK/bin:$PATH" bash "$WORK/run.sh" >/dev/null 2>"$WORK/trace.log" || fail "正常系で user_data が異常終了した"

[ "$(stat -f '%Lp' "$ENV_FILE" 2>/dev/null || stat -c '%a' "$ENV_FILE")" = 600 ] || fail ".env のパーミッションが 0600 ではない"

for key in OPENAI_API_KEY ADMIN_SECRET BFF_INTERNAL_TOKEN TOKEN_ENCRYPTION_KEY DB_PASSWORD; do
  grep -qE "^$key=." "$ENV_FILE" || fail "$key が .env に入っていない"
done
grep -q '^DB_PASSWORD=p@ss word$' "$ENV_FILE" || fail "DB_PASSWORD の値が Secrets Manager の JSON と一致しない"
grep -q '^ADMIN_SECRET=adm\$ecret$' "$ENV_FILE" || fail "ADMIN_SECRET の値がシェル展開で壊れている"

# シークレットがトレース(= cloud-init-output.log)へ出ていないこと
if grep -qE 'sk-dummy|p@ss|adm\$ecret' "$WORK/trace.log"; then fail "シークレットが set -x のトレースに出ている"; fi

# --- 2. 取得できないときは .env を書かずに止まる ---
for mode in denied empty nopassword; do
  write_aws_stub "$mode"
  rm -f "$ENV_FILE"
  if PATH="$WORK/bin:$PATH" bash "$WORK/run.sh" >/dev/null 2>&1; then
    fail "シークレットを取得できない($mode)のに user_data が成功終了した"
  fi
  if [ -f "$ENV_FILE" ]; then fail "シークレットを取得できない($mode)のに .env が作られた"; fi
done

echo "OK: user_data の .env 生成ロジックは期待どおり（起動確認そのものは staging で行うこと）"
