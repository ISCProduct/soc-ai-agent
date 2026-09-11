# 実装計画: AWS ECS on EC2（staging 常時）

> **方針更新 (2026-08-05)**  
> **staging = AWS ECS（常時起動）**、**本番 = AWS ECS（既定停止・反映時明示起動・指定日は終日）**。  
> 現行の正は → [`infra-decision-oci-stg-aws-prod.md`](./infra-decision-oci-stg-aws-prod.md)  
> ~~staging = OCI~~ は撤回。`environments/staging`（AWS）が staging の正。

最終更新: 2026-08-05  
関連: #648 / コスト試算 `aws-ecs-ec2-cost-estimate.md` / 決定 `infra-decision-oci-stg-aws-prod.md`

## 1. 方針（合意事項）

| 項目 | 決定 |
|------|------|
| staging | **AWS ECS on EC2・常時起動**（`environments/staging`） |
| production | **AWS ECS on EC2**・既定停止。本番反映時に明示起動。**指定日は終日起動** |
| NAT Gateway | **なし**（staging コスト優先） |
| RDS | **あり** |
| ALB | **初期はなし可** |
| 独自ドメイン / ACM | ドメイン取得後（#758） |
| 初期アプリ | **backend + frontend** |
| OCI | staging としては **使わない** |

### コスト感（目安）

- AWS staging 常時: **約 ¥8,000〜15,000 / 月**（ALB なし・NAT なし）
- AWS 本番: 稼働日数に比例（指定日・反映時のみ）

---

## 2. 目標構成（staging）

> **現行 staging = AWS**（常時）。以下はその構成。

```text
Internet
  → (任意) Route53
  → EC2 public（EIP）上の ECS Agent
       ├ ECS Service: frontend (:3000)
       └ ECS Service: backend  (:8080)
  → RDS MySQL（SG で ECS/EC2 からのみ許可）
  → S3
  → Secrets Manager / SSM
```

- リージョン: `ap-northeast-1`
- 単一 AZ で開始可（コスト優先）。後から Multi-AZ 拡張可能なモジュール設計にする
- **常時起動**（ASG desired>=1 を維持）

---

## 3. ディレクトリ構成

```text
infra/terraform/
  bootstrap/                 # state 用 S3 + DynamoDB（全環境共通・最初に作成）
  environments/
    staging/                 # ★今回の主対象
    prod/                    # 既存雛形は残す。本番起動用は後続で整備
  modules/
    network/                 # VPC + public subnet(s) + SG（NAT なし）
    ecs_cluster/             # Cluster + ASG + Capacity Provider + IAM
    ecs_service/             # Task Definition + Service
    rds/                     # MySQL + subnet group + SG
    s3/                      # staging 用バケット
    secrets/                 # Secrets Manager
    # 後続: alb/ , acm_route53/
```

既存の単一 EC2 用 `modules/compute` はレガシー扱い。staging は新モジュールで組み立てる。

---

## 4. フェーズ計画

### Phase 0 — 前提・bootstrap（0.5日）

- [ ] Terraform remote state（S3 + DynamoDB lock）
- [ ] AWS アカウント / タグ規約（`Project=soc-ai-agent`, `Env=staging`）
- [ ] 既存 ECR（backend / frontend）の確認
- [ ] `environments/staging` のスケルトン作成

**完了条件:** `terraform init` が staging で通る

### Phase 1 — ネットワーク（NAT なし）（0.5〜1日）

- [ ] VPC + public subnet（必要なら 2AZ 分の public のみ）
- [ ] IGW + public route
- [ ] SG: `ecs-ec2`（80/443/必要ポート）、`rds`（3306 は ecs-ec2 からだけ）
- [ ] SSH は必要なら IP 制限。可能なら SSM Session Manager に寄せる

**完了条件:** `terraform plan` で network が通る / apply 後に VPC が立つ

### Phase 2 — データ層（RDS + S3 + Secrets）（1日）

- [ ] RDS MySQL 8.x（`db.t4g.micro` 目安、Single-AZ、20GB）
- [ ] Secrets Manager に DB 接続情報
- [ ] S3 バケット（暗号化・Block Public Access）
- [ ] （任意）既存本番 RDS を流用せず **staging 専用**を新規作成

**完了条件:** セキュリティグループ経由で MySQL に到達できる / S3 に読み書きできる

### Phase 3 — ECS on EC2（1〜2日）

- [ ] ECS Cluster
- [ ] ASG desired=1（`t4g.small` 目安）+ ECS-optimized AMI + instance profile
- [ ] Capacity Provider
- [ ] Task Definition（`infra/ecs/task-def*.json` を Terraform 化）
- [ ] Service: `frontend`, `backend`
- [ ] EIP または固定の公開方法
- [ ] CloudWatch Logs
- [ ] ホスト上または簡易 reverse proxy で 80→3000 / api→8080（ALB なし時）

**完了条件:** 公開 URL（EIP or 一時 DNS）で FE/BE が応答し、BE が RDS/S3 に接続できる

### Phase 4 — 設定・デプロイ連携（1日）

- [ ] staging 用環境変数 / Secrets 注入
- [ ] GitHub Actions: staging 向け ECR push → `ecs update-service`（手動 or staging ブランチ）
- [ ] `docs/wiki/` に apply・デプロイ手順を記載
- [ ] コスト確認（Cost Explorer タグ `Env=staging`）

**完了条件:** イメージ更新だけで staging に反映できる

### Phase 5 — 後続（今回スコープ外）

- [ ] ALB + ACM + Route53（ドメイン取得後）
- [ ] prod 環境（展示会時のみ start/stop）
- [ ] rag / chroma / company-graph
- [ ] Multi-AZ / バックアップ強化（#620）

---

## 5. 実装順序（推奨）

```text
bootstrap(state)
  → staging network（NATなし）
  → rds + s3 + secrets
  → ecs cluster(EC2/ASG) + services
  → CI/CD（staging）
  → （後）ALB/ACM/Route53
  → （後）prod の起動停止運用
```

---

## 6. 受け入れ条件（staging）

- [ ] `terraform plan/apply` で staging の ECS(EC2) + RDS + S3 が再現できる
- [ ] NAT Gateway が作られないこと
- [ ] ECS タスクが RDS / S3 / Secrets に接続できる
- [ ] frontend / backend に外部からアクセスできる
- [ ] 機密がコード・tfstate 平文に残らない
- [ ] 適用・デプロイ手順がドキュメント化されている
- [ ] 月額実測が概算レンジ（おおよそ ¥1万前後）から大きく外れないこと

---

## 7. リスクと対策

| リスク | 対策 |
|--------|------|
| public EC2 の露出 | SG 最小開放、SSM 推奨、後で ALB 化 |
| 単一 AZ | staging 許容。prod で見直し |
| 既存 prod EC2 との混線 | タグ・名前・SG・ECR タグを `staging` で分離 |
| RDS コスト | `db.t4g.micro`、不要なら停止（検証時） |
| ドメイン未取得 | IP/既存ドメインのサブドメインで仮公開 |

---

## 8. 次のアクション

1. ~~Terraform 実装~~ → **済み**（`infra/terraform/environments/staging`）
2. **bootstrap → staging を apply**し、**常時起動**で FE/BE を載せる（OCI は使わない）
3. RDS は **staging 専用新規**（既存本番 DB は触らない）
4. 本番は後続: 既定停止 + 反映時明示起動 + 指定日終日（決定ドキュメント参照）
5. ALB / 独自ドメインはドメイン準備後
