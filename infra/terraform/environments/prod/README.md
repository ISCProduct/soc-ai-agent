# Terraform prod（ECS on Fargate / ALB / RDS あり / ドメイン紐付け）

> **方針:** 本番は **既定停止**。反映時に明示起動、または指定日は終日起動。
> 詳細: [`docs/architecture/infra-decision-oci-stg-aws-prod.md`](../../../docs/architecture/infra-decision-oci-stg-aws-prod.md)
>
> コンピュートは **ECS on Fargate**（EC2/ASGなし）。「展示会・説明会時のみ起動」する断続運用のため、
> 稼働した分だけ課金されるFargateを採用（staging は常時起動用途のため ECS on EC2 のまま据え置き）。
>
> **重要:** 本ディレクトリは旧 EC2 直起動構成（`compute_legacy` + `network_legacy`）から刷新した。
> 既存 state に対して `terraform apply` すると、旧 EC2 インスタンス／Route53 レコードが破棄され
> 新しい Fargate 基盤に置き換わる（破壊的変更）。実際に流す前に必ず `terraform plan` の内容をチームでレビューすること。

## 構成

- VPC public subnet（NAT Gateway なし）
- ALB（HTTP→HTTPSリダイレクト、ACM証明書DNS検証、ホストヘッダーでfrontend/backendにルーティング）。**常時起動・固定課金**（NAT/EC2/ASGなしでも月$16〜20程度は発生）
- ECS Cluster（Fargate/Fargate Spot）。ECS Service: `backend` (:8080) / `frontend` (:3000)
  - 既定 `backend_desired_count=0` / `frontend_desired_count=0`（停止）。起動時のみ稼働課金
- RDS MySQL（`db.t4g.micro`、`deletion_protection=true` が既定）
- S3 + Secrets Manager
- Route53: `<domain_name>`（apex, frontend） / `api.<domain_name>`（backend）を ALB に ALIAS レコードで紐付け

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
# bucket を bootstrap 出力に合わせて編集（key は prod/terraform.tfstate）
terraform init -backend-config=backend.hcl
```

### 4. plan / apply

既定（`*_desired_count=0`）で apply すると ALB/ECS/RDS/S3/Route53 は作られるが Fargate タスクは0台 = アプリは起動しない。ALB自体は常時課金される点に注意。

```bash
terraform plan -out=prod.plan
terraform apply prod.plan
terraform output
```

### 5. 起動する場合

```bash
terraform apply -var="backend_desired_count=1" -var="frontend_desired_count=1"
```

停止に戻す場合は両方 `0` で再度 apply（ALB自体は destroy しない限り課金され続ける）。

## 注意

- RDS は `deletion_protection=true` / `skip_final_snapshot=false` が既定（本番保護）
- DB パスワードは output に出さない（Secrets Manager 参照）
- Fargateタスクは NAT なしの public subnet で `assign_public_ip=true`（ECR pull / CloudWatch Logs 到達のため）。ALBのSGからのみ受信を許可
- 起動・停止の自動化（指定日終日起動など）は別タスク。本 README の手動 apply が現時点の運用手段
