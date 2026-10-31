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
| `soc-app/service-secret` | `service_secret` | 自動生成（`random_password`） | 初回のみ |

`admin_secret` は **staging と別値**にする。staging が漏れても本番の管理者権限に波及させないため。

## `admin_secret` と `service_secret` の役割分担（#1158）

| | `admin_secret`（`ADMIN_SECRET`） | `service_secret`（`SERVICE_SECRET`） |
|---|---|---|
| 用途 | 管理者/職員トークン（`X-Admin-Token`）のHMAC鍵 | CI・運用からのサービス間呼び出し（`X-Admin-Secret`）の共有シークレット |
| 漏えい時の影響 | `/api/admin/*` の全権限 | 更新情報の取り込みと学習データの件数取得のみ |
| CI が知るか | **知らない**（渡さない） | 知る（本番は Secrets Manager から読む） |

もともとこの2つは `ADMIN_SECRET` 1本で兼用されていた。CI が既知の値で
`/api/admin/whats-new/ingest` を叩く必要があるため、その要件のために
HMAC鍵の露出面まで CI へ広がっていた。シークレットを分けて解消済み。

`SERVICE_SECRET` が未設定でもバックエンドは起動する（Terraform apply より先に
新しいイメージがデプロイされても本番が落ちないようにするため）。その場合は
サービス間エンドポイントが 503 で fail-close し、取り込みは次回の実行で回収される。
**`ADMIN_SECRET` へのフォールバックは実装されていない。**

プレースホルダー（`change-me-service-secret`）のまま、または `ADMIN_SECRET` と同値の場合も
**未設定と同じ扱い**（503 で fail-close）になる。`.env.example` の値は公開されているので、
通ってしまうと `/api/admin/whats-new/ingest` と `/api/admin/training/stats` を誰でも叩ける。

`ignore_changes` は `secret_string` 全体にかかるため、**秘密でない値も Terraform からは変更できない**。
`google_client_id` / `github_client_id` は tfvars を書き換えても plan に差分が出ず無言で無視される。
OAuth アプリを作り直して client_id が変わった場合も、下の CLI 手順で書き換えること。

## 値を更新する（ローテーション）

`terraform apply` では更新されない。AWS CLI で直接書き換える。

`admin_secret` は CI に渡していないので、ローテーションしても CI 側の設定変更は要らない
（管理者は再ログインが必要になる）。

`service_secret` を変える場合は、**先に CI が Secrets Manager を読めることを確認する**。
読めないと CI は本番への取り込みを見送る（警告のみ。次回の本番起動時に回収される）。
値を書き換える対象は `soc-app/service-secret` で、キーは `service_secret`（下の手順の
`--secret-id` とキー名を読み替える）。手元で確認するなら次のとおり。

```bash
aws secretsmanager get-secret-value --secret-id soc-app/service-secret --query SecretString --output text >/dev/null \
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
unset NEW_SECRET
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

## `service_secret` を変えたあとの後始末

リポジトリシークレット `SERVICE_SECRET` は **staging 専用**（本番値を入れない）。
CI は本番向けには Secrets Manager だけを見る（フォールバックは無い）ので、
本番値を GitHub 側に置く理由は無い。

リポジトリシークレット `ADMIN_SECRET` は**不要**になった。CI は参照していないので削除してよい。

取り込みが 401/403/503 になった場合、CI は警告では流さずエラーにする。
- `prod-uptime-scheduler`（本番起動時の回収）はジョブを失敗させ、Discord へ通知する
- `deployment` の `sync-whats-new` はジョブを失敗表示にする（ジョブ自体が
  `continue-on-error` なのでデプロイは止まらない。通知は飛ばないので、
  デプロイ後にジョブの結果を見ること）

「停止中で届かない」と区別するための設計なので、これらが出たら鍵の不一致か
本番側の未設定を疑うこと。

## ローカルに平文が残っていないことの確認

```bash
grep -nE "openai_api_key|resend_api_key|client_secret|admin_secret" \
  infra/terraform/environments/prod/terraform.tfvars
```

値が入っていれば削除し、`secret_values_managed_outside = true` を設定する。
この変数を入れないと、`openai_api_key` が空の状態で precondition に止められる。

## tfvars を空にする前に値が入っていることを確認する

Terraform は値を管理しないので、**Secrets Manager 側が空のままでも plan は通る**。
`secret_values_managed_outside = true` にする前に、実値が入っていることを確かめること。
初回 apply を空の tfvars で通した環境では `{"openai_api_key": ""}` が固定されており、
気づかないまま「起動はするが AI 機能が全部無効」になる。

```bash
for id in soc-app/openai soc-app/email soc-app/oauth soc-app/admin soc-app/service-secret; do
  echo "--- $id"
  aws secretsmanager get-secret-value --secret-id "$id" --query SecretString --output text \
    | python3 -c "import json,sys; print({k: ('設定済み' if v else '空') for k, v in json.load(sys.stdin).items()})"
done
```

空のキーがあれば、上のローテーション手順で値を入れてから進める。

## 新規環境を作る場合

`secret_values_managed_outside` は `false`（既定）のまま、tfvars に値を入れて一度 apply する。
作成後は tfvars から値を消し、`secret_values_managed_outside = true` にする。
