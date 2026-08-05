# インフラ方針決定: AWS staging（常時）+ AWS 本番（指定起動）

最終更新: 2026-08-05

## 決定

| 環境 | 基盤 | 運用 |
|------|------|------|
| **staging** | **AWS ECS on EC2**（`infra/terraform/environments/staging`） | **常時起動**。開発・検証の正環境 |
| **production** | **AWS ECS on EC2** | **既定は停止**。本番反映時に明示起動。**指定した日付はその日終日（24時間）起動** |

### 以前の方針からの変更

- ~~staging = OCI~~ → **staging = AWS（常時起動）**
- ~~本番の平日昼間定期 cron~~ → **本番反映時の明示起動 + 指定日は終日稼働**（変更なし）
- OCI（`environments/oci`）は **現行 staging としては使わない**（アーカイブ / 将来の実験用）

staging と本番が同じクラウド（AWS ECS）になるため、環境差分が小さくなり予行しやすい。

## 意図

- staging を常時使える開発環境にする（OCI ではなく AWS）
- 本番は使わない日は止めてコスト抑制
- 展示・説明会などは指定日リストで終日 ON

## 構成イメージ

```text
[開発・検証] 常時
  GitHub → ECR → AWS staging ECS + RDS + S3
           infra/terraform/environments/staging
           NAT なし / ALB なし可 / EIP 公開

[本番] 既定停止
  GitHub → ECR → AWS prod ECS + RDS + S3
  起動:
    (A) 本番反映で明示指定 → 起動してデプロイ
    (B) 指定日リスト → その暦日(JST)は終日起動
  停止: 指定日以外（反映ロック中を除く）
```

---

## staging（AWS・常時）

### 方針

- Terraform: `infra/terraform/environments/staging`（既存実装を正とする）
- コンピュート: ECS on EC2、NAT なし、RDS あり
- **常時 `desired>=1`**（コスト優先構成のまま止めない）
- 手順: `infra/terraform/environments/staging/README.md`

### 受け入れ条件

- [ ] `terraform apply` で staging の VPC / RDS / S3 / ECS が立つ
- [ ] FE/BE が常時到達できる
- [ ] NAT Gateway が無い
- [ ] 既存本番（現行 EC2 等）を誤って変更しない

### コスト目安

- 常時 staging: おおよそ **¥8,000〜15,000 / 月**（ALB なし・NAT なし）
- 詳細: `aws-ecs-ec2-cost-estimate.md`

---

## 本番起動ポリシー（指定起動）

### 原則

1. **既定状態は停止**（ASG `desired=0`。必要なら RDS も stop）
2. **毎日の昼間自動起動はしない**
3. 起動は次のどちらか（または両方）

### (A) 本番反映時の明示起動

- 本番デプロイ操作で **「今回は本番を起動する」** を明示（例: `workflow_dispatch` の `start_production=true`）
- 流れ: 起動 → ヘルス待ち → デプロイ →（指定日でなければ作業後停止）

### (B) 指定日は終日起動

- 運用者が **日付リスト**を指定（展示会・説明会など）
- **その日付（JST 00:00〜23:59:59）はずっと起動**
- 日付が明けたら停止
- 持ち方の候補: `infra/prod-uptime-dates.txt` / GitHub Variables / SSM Parameter
- 照合用スケジューラ（毎時等）は「今日が指定日か」だけ見る。昼間固定起動 cron ではない

### 反映作業中ロック

- 反映直後に指定日チェッカーが止めないよう lock（SSM / タグ等）を入れる

### 受け入れ条件（本番）

- [ ] 既定は停止
- [ ] 反映時の明示起動→デプロイができる
- [ ] 指定日は終日起動が維持される
- [ ] 指定日以外は停止に戻る（ロック中除く）
- [ ] 日付追加・削除手順がドキュメント化されている

---

## AWS コードの位置づけ

| パス | 位置づけ |
|------|----------|
| `infra/terraform/environments/staging` | **★ staging 正（常時）** |
| `infra/terraform/bootstrap` | AWS remote state |
| `infra/terraform/modules/{network,rds,s3,secrets,ecs_*}` | staging / 将来 prod 共用 |
| `infra/terraform/environments/prod` | 将来の本番 root（現状は旧 EC2 雛形。ECS 化時に刷新） |
| `infra/terraform/environments/oci` | **非正**（使わない。整理時に archive） |

## リスクと緩和

| リスク | 緩和 |
|--------|------|
| staging 常時の月額 | NAT/ALB なし構成を維持。不要リソースを作らない |
| 本番指定日の入れ忘れ | 展示前チェックリストに uptime dates を入れる |
| 反映中の自動停止 | maintenance lock |
| `prod` ディレクトリが旧 EC2 | 本番着手時に ECS 化 or staging をベースに作り直し |

## 次のアクション

1. **AWS staging を常時で apply**（bootstrap → staging）。OCI は触らない
2. staging にアプリ（ECR イメージ）を載せて常時検証できる状態にする
3. **本番**: `environments/prod` を ECS 化し、明示起動 + 指定日終日の制御を追加
4. infra 整理: OCI を `archive/` へ（任意・後続）
