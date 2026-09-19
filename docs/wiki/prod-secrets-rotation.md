# 本番シークレットの管理とローテーション

本番の実シークレットは **Secrets Manager を正**とし、ローカルの `terraform.tfvars` に平文で置かない（#1158）。

Terraform は「シークレットの入れ物（`aws_secretsmanager_secret`）」だけを管理し、**値は管理しない**。
値を書き込む `aws_secretsmanager_secret_version` には `lifecycle { ignore_changes = [secret_string] }` が入っているため、
tfvars が空でも plan に差分は出ない。

## どの値がどこから来るか

| Secrets Manager | キー | 出どころ | Terraform が初回に書くか |
|---|---|---|---|
| `soc-app/openai` | `openai_api_key` | OpenAI ダッシュボード | 初回のみ（以降は無視） |
| `soc-app/email` | `resend_api_key` | Resend ダッシュボード | 初回のみ |
| `soc-app/oauth` | `google_client_secret` / `github_client_secret` | 各 OAuth アプリ | 初回のみ |
| `soc-app/admin` | `admin_secret` | 自動生成（`random_password`） | 初回のみ |
| `soc-app/admin` | `user_secret` / `company_user_secret` / `oauth_state_secret` / `token_encryption_key` | 自動生成 | 初回のみ |

`admin_secret` は **staging と別値**にする。staging が漏れても本番の管理者権限に波及させないため。
CI（`sync-whats-new`）は本番向けには Secrets Manager から読むので、既知の固定値である必要はない。

## 値を更新する（ローテーション）

`terraform apply` では更新されない。AWS CLI で直接書き換える。

```bash
# 1. 現在の値を取得（JSONまるごと。画面共有中は実行しないこと）
aws secretsmanager get-secret-value --secret-id soc-app/admin \
  --query SecretString --output text > /tmp/admin.json

# 2. 変えたいキーだけ差し替える（例: admin_secret を新しいランダム値へ）
NEW=$(python3 -c "import secrets; print(secrets.token_urlsafe(36))")
python3 - <<'PY' /tmp/admin.json "$NEW"
import json, sys
p, new = sys.argv[1], sys.argv[2]
d = json.load(open(p))
d["admin_secret"] = new
json.dump(d, open(p, "w"))
PY

# 3. 書き戻す
aws secretsmanager put-secret-value --secret-id soc-app/admin \
  --secret-string "file:///tmp/admin.json" >/dev/null

# 4. 後始末（重要）
shred -u /tmp/admin.json 2>/dev/null || rm -P /tmp/admin.json
```

**反映にはタスクの再起動が必要。** ECS は起動時にしか Secrets Manager を読まない。
本番は稼働日のみ起動する運用なので、**停止中に書き換えるのが最も安全**（落ちるユーザーがいない）。

## 起動中にローテーションする場合の順序

1. 新しい値を Secrets Manager へ書く
2. `aws ecs update-service --cluster soc-app --service backend --force-new-deployment`
3. `/health` が 200 を返すことを確認
4. 外部サービス側（OpenAI / Resend / OAuth）の旧キーを失効させる

**旧キーの失効は必ず最後**。先に失効させると、新デプロイが失敗したときに戻す先が無くなる。

## ローカルに平文が残っていないことの確認

```bash
grep -nE "openai_api_key|resend_api_key|client_secret|admin_secret" \
  infra/terraform/environments/prod/terraform.tfvars
```

値が入っていれば削除し、`secret_values_managed_outside = true` を設定する。
この変数を入れないと、`openai_api_key` が空の状態で precondition に止められる。

## 新規環境を作る場合

`secret_values_managed_outside` は `false`（既定）のまま、tfvars に値を入れて一度 apply する。
作成後は tfvars から値を消し、`secret_values_managed_outside = true` にする。
