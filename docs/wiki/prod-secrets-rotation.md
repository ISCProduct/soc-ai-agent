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

`ignore_changes` は `secret_string` 全体にかかるため、**秘密でない値も Terraform からは変更できない**。
`google_client_id` / `github_client_id` は tfvars を書き換えても plan に差分が出ず無言で無視される。
OAuth アプリを作り直して client_id が変わった場合も、下の CLI 手順で書き換えること。

## 値を更新する（ローテーション）

`terraform apply` では更新されない。AWS CLI で直接書き換える。

`admin_secret` を変える場合は、**先に CI が Secrets Manager を読めることを確認する**。
読めないまま回すと CI は旧値（staging 共用の値）でフォールバックし、本番への
更新情報の取り込みが 403 になる。手元で確認するなら次のとおり。

```bash
aws secretsmanager get-secret-value --secret-id soc-app/admin --query SecretString --output text >/dev/null \
  && echo "読めた（CI のロールでも同じ権限があるか確認すること）"
```

```bash
umask 077  # 一時ファイルを他ユーザーに読ませない

# 1. 現在の値を取得（JSONまるごと。画面共有中は実行しないこと）
aws secretsmanager get-secret-value --secret-id soc-app/admin \
  --query SecretString --output text > /tmp/admin.json

# 2. 変えたいキーだけ差し替える（例: admin_secret を新しいランダム値へ）
#    新しい値は環境変数で渡す。argv だと同じマシンの ps から見える。
export NEW_SECRET=$(python3 -c "import secrets; print(secrets.token_urlsafe(36))")
python3 - <<'PY' /tmp/admin.json
import json, os, sys
p = sys.argv[1]
d = json.load(open(p))
d["admin_secret"] = os.environ["NEW_SECRET"]
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
停止中（`desired_count=0`）に書き換えた場合は次回起動時に自動で反映されるため、下の手順は不要。

## 起動中にローテーションする場合の順序

1. 新しい値を Secrets Manager へ書く
2. `aws ecs update-service --cluster soc-app --service backend --force-new-deployment`
3. `/health` が 200 を返すことを確認
4. 外部サービス側（OpenAI / Resend / OAuth）の旧キーを失効させる

**旧キーの失効は必ず最後**。先に失効させると、新デプロイが失敗したときに戻す先が無くなる。

## `admin_secret` を変えたあとの後始末

リポジトリシークレット `ADMIN_SECRET` は **staging 専用**に戻す（本番値を入れない）。
CI は本番向けには Secrets Manager から読み、読めないときだけこのシークレットへ
フォールバックする。本番値が残っていると、権限不調に気づかないまま動いてしまう。

取り込みが 401/403 になった場合、CI はジョブを失敗させる（警告では流さない）。
「停止中で届かない」と区別するための設計なので、失敗したら鍵の不一致を疑うこと。

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
