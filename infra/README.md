# infra/

## 現在の方針（2026-08-05）

| 環境 | 場所 | 備考 |
|------|------|------|
| **staging** | `terraform/environments/staging`（**AWS ECS**） | **常時起動**が正 |
| **production** | AWS ECS（今後 `environments/prod` を ECS 化） | **既定は停止**。本番反映時に明示起動。**指定日は終日稼働** |
| OCI | `terraform/environments/oci` | **現行では使わない** |

決定の詳細: [`docs/architecture/infra-decision-oci-stg-aws-prod.md`](../docs/architecture/infra-decision-oci-stg-aws-prod.md)

## ディレクトリ

```text
infra/
  terraform/
    bootstrap/           # AWS remote state
    environments/
      staging/           # ★ staging（AWS ECS・常時）
      prod/              # 本番（今後 ECS 化。現状は旧 EC2 雛形）
      oci/               # 非正（アーカイブ候補）
    modules/
      network, rds, s3, secrets, ecs_*   # AWS
      *_legacy, oci-*                    # レガシー / OCI
  ecs/                   # 旧 task-def JSON（参照用）
  nginx/
  scripts/
```

## すぐ触るとき

- AWS staging（常時）: `terraform/environments/staging/README.md`
- 方針全体: `docs/architecture/infra-decision-oci-stg-aws-prod.md`
