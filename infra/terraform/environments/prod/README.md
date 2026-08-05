# Terraform prod（ECS on EC2 / NAT なし / RDS あり / ドメイン紐付け）

> **方針:** 本番は **既定停止**。反映時に明示起動、または指定日は終日起動。
> 詳細: [`docs/architecture/infra-decision-oci-stg-aws-prod.md`](../../../docs/architecture/infra-decision-oci-stg-aws-prod.md)
>
> **重要:** 本ディレクトリは旧 EC2 直起動構成（`compute_legacy` + `network_legacy`）から
> `staging` と同じ ECS on EC2 構成に刷新した。既存 state に対して `terraform apply` すると
> 旧 EC2 インスタンス／Route53 レコードが破棄され、新しい ECS 基盤に置き換わる（破壊的変更）。
> 実際に流す前に必ず `terraform plan` の内容をチームでレビューすること。

## 構成

- VPC public subnet（NAT Gateway なし）
- ECS Cluster + ASG（`t4g.small` 目安）+ EIP。`ecs_desired_capacity` 既定 `0`（停止）
- ECS Service: `backend` (:8080) / `frontend` (:3000)
- RDS MySQL（`db.t4g.micro`、`deletion_protection=true` が既定）
- S3 + Secrets Manager
- Route53: `<domain_name>`（apex, frontend） / `api.<domain_name>`（backend）を ECS EIP に A レコードで紐付け
  - ALB なし構成のため、実アクセスはポート付き（`http://shukatsu-ai.jp:3000` など）。常時 443 化する場合は別途 ALB + ACM を追加すること

## 前提

- AWS CLI 認証済み（`ap-northeast-1`）
- Terraform >= 1.5
- ECR に backend / frontend の本番用イメージがあること
- `domain_name`（既定 `shukatsu-ai.jp`）の Route53 ホストゾーンが対象 AWS アカウントに既に存在すること

## 手順

### 1. bootstrap（staging と共用。未実施なら）

```bash
cd ../../bootstrap
terraform init
terraform apply
terraform output backend_hcl_example
```

### 2. tfvars

```bash
cd ../environments/prod
cp terraform.tfvars.example terraform.tfvars
# ACCOUNT_ID / イメージタグ / openai_secret_arn を編集
```

`*.tfvars` は gitignore 済み。

### 3. remote state

```bash
cp backend.hcl.example backend.hcl
# bucket / dynamodb_table を bootstrap 出力に合わせて編集（key は prod/terraform.tfstate）
terraform init -backend-config=backend.hcl
```

### 4. plan / apply

既定（`ecs_desired_capacity=0`）で apply すると ECS/RDS/S3/Route53 は作られるが ASG は 0 台 = アプリは起動しない。

```bash
terraform plan -out=prod.plan
terraform apply prod.plan
terraform output
```

### 5. 起動する場合

```bash
terraform apply -var="ecs_desired_capacity=1" -var="ecs_min_size=1"
```

停止に戻す場合は `ecs_desired_capacity=0` / `ecs_min_size=0` で再度 apply。

## 注意

- RDS は `deletion_protection=true` / `skip_final_snapshot=false` が既定（本番保護）
- DB パスワードは output に出さない（Secrets Manager 参照）
- EIP は ASG 置換時に user_data で再関連付け
- 起動・停止の自動化（指定日終日起動など）は別タスク。本 README の手動 apply が現時点の運用手段
