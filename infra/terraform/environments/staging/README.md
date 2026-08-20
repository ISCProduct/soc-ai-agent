# Terraform staging（ECS on EC2 / NAT なし / RDS あり）

> **方針 (2026-08-05):** 本ディレクトリが **アプリケーション staging の正**です（**AWS・常時起動**）。  
> 本番は別途、既定停止＋反映時明示起動＋指定日終日。  
> 詳細: [`docs/architecture/infra-decision-oci-stg-aws-prod.md`](../../../docs/architecture/infra-decision-oci-stg-aws-prod.md)

## 構成

- VPC public subnet（NAT Gateway なし）
- ECS Cluster + ASG（`t4g.small` 目安）+ EIP
- ALB（HTTP→HTTPSリダイレクト、ACM証明書DNS検証、ホストヘッダーでfrontend/backendにルーティング）
- ECS Service: `backend` (:8080) / `frontend` (:3000)。ALB配下のため直接ポート公開はしない
- RDS MySQL（`db.t4g.micro`）
- S3 + Secrets Manager
- Route53: `stg.<domain_name>`（frontend） / `api-stg.<domain_name>`（backend）を ALB に ALIAS レコードで紐付け（既定 `domain_name=shukatsu-ai.jp`）

詳細計画: `docs/architecture/aws-terraform-staging-implementation-plan.md`

## 前提

- AWS CLI 認証済み（`ap-northeast-1`）
- Terraform >= 1.5
- ECR に backend / frontend イメージがあること

## 手順

### 1. bootstrap（初回のみ・ローカル state）

```bash
cd infra/terraform/bootstrap
terraform init
terraform apply
terraform output backend_hcl_example
```

出力の bucket / dynamodb_table を控える。

### 2. staging の tfvars

```bash
cd ../environments/staging
cp terraform.tfvars.example terraform.tfvars
# ACCOUNT_ID / イメージタグ / openai_secret_arn を編集
```

`*.tfvars` は gitignore 済み。

### 3. remote state（推奨）

`versions.tf` の `backend "s3"` ブロックコメントを外し:

```bash
cp backend.hcl.example backend.hcl
# bucket / dynamodb_table を bootstrap 出力に合わせて編集
terraform init -backend-config=backend.hcl
```

初回だけローカル state で試す場合は backend をコメントのまま:

```bash
terraform init
```

### 4. plan / apply

```bash
terraform plan -out=stg.plan
terraform apply stg.plan
terraform output
```

公開 URL（ALB経由・HTTPS）:

- Frontend: `https://stg.shukatsu-ai.jp`
- Backend: `https://api-stg.shukatsu-ai.jp`

`domain_name`（既定 `shukatsu-ai.jp`）の Route53 ホストゾーンが対象 AWS アカウントに存在しない場合、`aws_route53_zone` の data lookup と ACM 証明書のDNS検証レコード作成で apply が失敗する。その場合はゾーンを先に用意するか、`domain_name` を保有ドメインに合わせて上書きすること。

ALB追加により月額コストが staging に上乗せされる（目安 $16〜20/月、常時起動）。`aws-ecs-ec2-cost-estimate.md` は ALBなし試算のため要再計算。

### 5. 破棄（検証後）

```bash
terraform destroy
```

RDS は staging 既定で `skip_final_snapshot=true`。本番では変更すること。

## エラーページ（staging 停止 / 起動中）

**現行方針:** CloudFront は使わず、**nginx edge のみ**で OGP 付き HTML を返す。

| 状態 | 経路 | 表示 |
|------|------|------|
| EC2 停止（ALB ターゲット全滅） | ALB | **生 503**（意図的に許容） |
| EC2 起動中（nginx UP、frontend 未応答） | nginx `error_page` | `service-starting.html`（OGP 付き） |

CloudFront+S3 フェイルオーバー用モジュール（`enable_error_fallback`）はコード上残しているが、当面 `false` のまま。

### 検証

```bash
# ローカル: 静的 HTML の OGP タグ
bash infra/scripts/verify-error-pages.sh

# 起動直後（EC2 は UP、frontend 起動待ち）
curl -s https://stg.shukatsu-ai.jp/ | grep -E 'og:title|起動'
```

## 注意

- **既存本番 EC2 / RDS は触らない**（staging 新規作成）
- NAT Gateway は作られない
- DB パスワードは output に出さない（Secrets Manager 参照）
- EIP は ASG 置換時に user_data で再関連付け
- frontend の `NEXT_PUBLIC_*` はビルド時埋め込みの場合がある。イメージ側の設定も確認すること

## デプロイ（アプリ更新）

イメージを ECR に push したあと:

```bash
aws ecs update-service --cluster soc-stg --service backend --force-new-deployment
aws ecs update-service --cluster soc-stg --service frontend --force-new-deployment
```

（cluster / service 名は `terraform output` を優先）
