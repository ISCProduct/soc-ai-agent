# infra/

## 現在の方針（2026-08-05）

| 環境 | 場所 | 備考 |
|------|------|------|
| **staging** | `terraform/environments/staging`（**AWS ECS on EC2 + ALB**） | **常時起動**が正。ECR(`soc-backend`/`soc-frontend`)もここで作成。`stg.shukatsu-ai.jp` / `api-stg.shukatsu-ai.jp` |
| **production** | `terraform/environments/prod`（**AWS ECS on Fargate + ALB**） | **既定は停止**。同一アカウントでは staging の ECR を参照（`manage_ecr=false`） |
| OCI | `terraform/environments/oci` | **現行では使わない** |

決定の詳細: [`docs/architecture/infra-decision-oci-stg-aws-prod.md`](../docs/architecture/infra-decision-oci-stg-aws-prod.md)

## ディレクトリ

```text
infra/
  terraform/
    bootstrap/           # AWS remote state
    environments/
      staging/           # ★ staging（AWS ECS on EC2 + ALB・常時）
      prod/              # 本番（AWS ECS on Fargate + ALB・既定停止）
      oci/               # 非正（アーカイブ候補）
    modules/
      network, rds, s3, secrets, alb, ecs_cluster, ecs_service, ecs_service_fargate   # AWS
      *_legacy, oci-*                    # レガシー / OCI
  ecs/                   # 旧 task-def JSON（参照用）
  nginx/
  scripts/
```

## すぐ触るとき

- AWS staging（常時）: `terraform/environments/staging/README.md`
- 方針全体: `docs/architecture/infra-decision-oci-stg-aws-prod.md`
