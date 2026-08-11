# Terraform 実装計画: AWS staging（ECS on EC2）

最終更新: 2026-08-04  
実装コード: `infra/terraform/bootstrap/` / `infra/terraform/environments/staging/` / `infra/terraform/modules/{network,rds,s3,secrets,ecs_cluster,ecs_service}`  
適用手順: `infra/terraform/environments/staging/README.md`

## 0. 目的

Terraform で **staging 環境だけ**を AWS に再現可能に作る。

| 採用 | 不採用（初期） |
|------|----------------|
| ECS on EC2 | Fargate |
| NAT なし（public EC2） | NAT Gateway |
| RDS MySQL（staging 専用） | EC2 同居 MySQL |
| ALB なし（EIP 公開） | ALB / ACM / Route53（後続） |
| backend + frontend | rag / chroma / company-graph |

---

## 1. 現状との関係

| パス | 扱い |
|------|------|
| `environments/prod/` + `modules/compute` | **レガシー**（単一 EC2）。staging では使わない |
| `modules/network` | **作り直し**（`aws_gateway_all_traffic` 等の不整合あり・単一 public のみ） |
| `environments/oci/` | 対象外（触らない） |
| `infra/ecs/task-def*.json` | Terraform `aws_ecs_task_definition` の初期値ソース |

---

## 2. ディレクトリとファイル

```text
infra/terraform/
  bootstrap/
    main.tf                 # S3 bucket + DynamoDB（state）
    versions.tf
    outputs.tf
  environments/
    staging/
      versions.tf           # required_version / providers / backend
      providers.tf
      main.tf               # module 組み立て
      variables.tf
      outputs.tf
      terraform.tfvars.example
      README.md
  modules/
    network/
      main.tf / variables.tf / outputs.tf
    ecs_cluster/
      main.tf / variables.tf / outputs.tf / user_data.sh.tftpl
    ecs_service/
      main.tf / variables.tf / outputs.tf
    rds/
      main.tf / variables.tf / outputs.tf
    s3/
      main.tf / variables.tf / outputs.tf
    secrets/
      main.tf / variables.tf / outputs.tf
```

命名規則:

- リソース Name タグ: `soc-stg-<component>`
- 共通タグ: `Project=soc-ai-agent`, `Env=staging`, `ManagedBy=terraform`

---

## 3. モジュール別リソース設計

### 3.1 `bootstrap`（ローカル state で一度だけ）

| リソース | 用途 |
|----------|------|
| `aws_s3_bucket` | tfstate |
| `aws_s3_bucket_versioning` | 履歴 |
| `aws_s3_bucket_server_side_encryption_configuration` | SSE |
| `aws_s3_bucket_public_access_block` | 公開禁止 |
| `aws_dynamodb_table` | state lock（`LockID`） |

適用後、`environments/staging/versions.tf` の backend を S3 に切り替える。

### 3.2 `modules/network`（NAT なし）

| リソース | 設定 |
|----------|------|
| `aws_vpc` | CIDR 例 `10.10.0.0/16`、DNS hostnames ON |
| `aws_internet_gateway` | |
| `aws_subnet` public ×1〜2 | `map_public_ip_on_launch=true` |
| `aws_route_table` + association | `0.0.0.0/0 → IGW` |
| `aws_security_group` ecs | ingress: 80, 443, 3000, 8080（必要最小） / egress all |
| `aws_security_group` rds | ingress: 3306 from ecs SG only |

**作らないもの:** NAT Gateway / NAT Instance / private subnet（初期）

outputs: `vpc_id`, `public_subnet_ids`, `ecs_security_group_id`, `rds_security_group_id`

### 3.3 `modules/rds`

| リソース | 設定 |
|----------|------|
| `aws_db_subnet_group` | public subnet でも可（アクセスは SG で制限）※後で private 化しやすい形 |
| `aws_db_instance` | engine `mysql` 8.x、`db.t4g.micro`、20GB gp3、Single-AZ、`publicly_accessible=false` |
| `random_password` | master パスワード生成（state に入るため後述の扱い注意） |
| （任意）`aws_db_parameter_group` | 必要なら |

outputs: `address`, `port`, `db_name`, `master_username`（password は Secrets へ）

### 3.4 `modules/secrets`

| リソース | 設定 |
|----------|------|
| `aws_secretsmanager_secret` | `soc-stg/db`（JSON: host/port/user/password/dbname） |
| `aws_secretsmanager_secret_version` | RDS 出力から組み立て |
| （参照）既存 OpenAI 秘密 | data source または変数で ARN を渡す（staging 用キー推奨） |

### 3.5 `modules/s3`

| リソース | 設定 |
|----------|------|
| `aws_s3_bucket` | `soc-stg-app-...`（一意名） |
| versioning（任意） | staging は OFF でも可 |
| encryption | SSE-S3 |
| public access block | 全拒否 |
| （任意）lifecycle | 古いオブジェクト削除 |

outputs: `bucket_id`, `bucket_arn`

### 3.6 `modules/ecs_cluster`

| リソース | 設定 |
|----------|------|
| `aws_ecs_cluster` | `soc-stg` |
| `aws_iam_role` + instance profile | ECS EC2、ECR pull、SSM、CloudWatch、Secrets |
| `aws_launch_template` | ECS-optimized AMI（Amazon Linux 2023）、`t4g.small`、user_data で cluster join |
| `aws_autoscaling_group` | desired=1, min=1, max=1〜2、public subnet |
| `aws_ecs_capacity_provider` + attachment | managed scaling は staging では控えめでも可 |
| `aws_eip` + association | 固定公開 IP（ALB なし運用） |

ネットワークモード方針（初期）:

- Task は既存 JSON に合わせ **host/bridge + hostPort**（8080/3000）を踏襲しやすい
- 将来 ALB + awsvpc に移行可能な変数化（`network_mode`）を残す

### 3.7 `modules/ecs_service`

サービスごとに module 呼び出し（frontend / backend）または 1 モジュールで両コンテナ。

| リソース | 設定 |
|----------|------|
| `aws_cloudwatch_log_group` | `/ecs/soc-stg-backend` 等、保持 7〜14 日 |
| `aws_ecs_task_definition` | image URI・cpu/memory・secrets・env |
| `aws_ecs_service` | desired=1、capacity provider strategy、deployment 最小 |
| IAM task execution role | ECR + Secrets + Logs |
| IAM task role | S3 読み書き |

env 例:

- backend: `APP_ENV=staging`, `DB_HOST`（または secret）, `AWS_REGION`
- frontend: `APP_ENV=staging`, `NEXT_PUBLIC_API_BASE_URL`（EIP or 後のドメイン）

---

## 4. `environments/staging` の組み立て

`main.tf` イメージ:

```hcl
module "network" { ... }
module "s3"      { ... }
module "rds"     {
  subnet_ids         = module.network.public_subnet_ids
  security_group_ids = [module.network.rds_security_group_id]
}
module "secrets" {
  db = module.rds
}
module "ecs_cluster" {
  subnet_ids        = module.network.public_subnet_ids
  security_group_id = module.network.ecs_security_group_id
}
module "backend" {
  source            = "../../modules/ecs_service"
  cluster_id        = module.ecs_cluster.cluster_id
  # ...
}
module "frontend" { ... }
```

主要 variables（`terraform.tfvars.example`）:

| 変数 | 例 |
|------|-----|
| `region` | `ap-northeast-1` |
| `project_name` | `soc-stg` |
| `vpc_cidr` | `10.10.0.0/16` |
| `instance_type` | `t4g.small` |
| `db_instance_class` | `db.t4g.micro` |
| `backend_image` | `xxxx.dkr.ecr..../soc-backend:staging` |
| `frontend_image` | `xxxx.dkr.ecr..../soc-frontend:staging` |
| `openai_secret_arn` | staging 用 ARN |
| `allowed_cidr_http` | 社内 IP に絞るのが望ましい |

outputs:

- `ecs_public_ip`
- `rds_endpoint`
- `s3_bucket`
- `ecs_cluster_name`
- `backend_service_name` / `frontend_service_name`

---

## 5. 実装フェーズ（Terraform 作業単位）

### T0 — bootstrap + staging 骨格（半日）

1. `bootstrap/` 作成・apply（ローカル state）
2. `environments/staging` に backend S3 設定
3. `versions.tf` / `providers.tf` / 空の `main.tf`
4. `terraform init` 成功

**完了:** remote state に接続できる

### T1 — network モジュール

1. 新 `modules/network`（既存は触らず別名でも可: `network_v2` → 最終的に置換）
2. staging から module 呼び出し
3. `plan` → `apply`

**完了:** VPC / subnet / SG が AWS 上に存在。NAT が 0 件

### T2 — s3 + rds + secrets

1. 各モジュール実装
2. RDS password は Secrets Manager へ格納（tf 出力に password を出さない）
3. `plan` → `apply`

**完了:** RDS endpoint 取得、secret ARN 取得、S3 bucket 作成

### T3 — ecs_cluster + ecs_service

1. cluster / ASG / EIP
2. task definition + service ×2
3. 既存 ECR イメージタグを指定して起動確認

**完了:** EIP 経由で FE/BE 応答、BE→RDS/S3 疎通

### T4 — 運用ドキュメント・変数整備

1. `environments/staging/README.md`（init/plan/apply、必要な IAM）
2. `docs/wiki/` に短手順を追加（任意）
3. `.gitignore` に `*.tfvars`（秘密入り）を確認

**完了:** 他メンバーが README だけで apply 手順を追える

### T5 — 後続（本計画の外）

- `modules/alb` + ACM + Route53
- `environments/prod` の ECS 化と start/stop
- Terraform CI（PR で `plan`）

---

## 6. 適用順序（コマンド）

```bash
# 1) state 基盤
cd infra/terraform/bootstrap && terraform init && terraform apply

# 2) staging
cd ../environments/staging
# backend 設定後
terraform init
terraform plan -out=stg.plan
terraform apply stg.plan
```

破壊的変更は staging でも `plan` レビュー必須。  
**既存本番 EC2 / 既存 RDS は state に取り込まない**（新規 staging 専用）。

---

## 7. セキュリティ・状態管理ルール

| ルール | 内容 |
|--------|------|
| 秘密 | `terraform.tfvars` は git 管理外。例だけ `*.example` |
| 出力 | DB パスワードを `output` しない |
| state | S3 暗号化 + bucket 非公開 + DynamoDB lock |
| SG | RDS は internet から 3306 を開けない |
| SSH | 可能なら無効化し SSM。開けるなら `allowed_ssh_cidr` 必須 |

---

## 8. 受け入れ条件（Terraform）

- [ ] `bootstrap` / `staging` で `terraform plan` がエラーなく通る
- [ ] `apply` で staging の VPC・RDS・S3・ECS(EC2)・Task/Service が作成される
- [ ] `aws_nat_gateway` がリソース定義・実体ともに存在しない
- [ ] `terraform output` に平文パスワードが出ない
- [ ] EIP で frontend/backend に到達できる
- [ ] backend が staging RDS と S3 を利用できる
- [ ] `terraform destroy` が staging で安全に実行可能（検証後）

---

## 9. 工数目安

| フェーズ | 目安 |
|----------|------|
| T0 | 0.5 日 |
| T1 | 0.5〜1 日 |
| T2 | 1 日 |
| T3 | 1〜2 日 |
| T4 | 0.5 日 |
| **合計** | **約 4〜5 日** |

---

## 10. 次の実装ステップ

1. **T0**: `bootstrap/` と `environments/staging` スケルトンを追加  
2. **T1**: NAT なし `modules/network` を実装して staging に接続  
3. 続けて T2 → T3  

実装開始の指示があれば T0 からコード着手する。
