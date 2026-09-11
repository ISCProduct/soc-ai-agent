# インフラ方針決定: AWS staging（常時）+ AWS 本番（指定起動）

最終更新: 2026-08-05

## 決定

| 環境 | 基盤 | 運用 |
|------|------|------|
| **staging** | **AWS ECS on EC2 + ALB**（`infra/terraform/environments/staging`） | **常時起動**。開発・検証の正環境 |
| **production** | **AWS ECS on Fargate + ALB** | **既定は停止**（Fargateタスクdesired=0。ALB自体は常時課金）。本番反映時に明示起動。**指定した日付はその日終日（24時間）起動** |

### 以前の方針からの変更

- ~~staging = OCI~~ → **staging = AWS（常時起動）**
- ~~本番の平日昼間定期 cron~~ → **本番反映時の明示起動 + 指定日は終日稼働**（変更なし）
- ~~本番 = ECS on EC2~~ → **本番 = ECS on Fargate**（展示会・説明会時のみ起動する断続運用のため、稼働時間分のみ課金されるFargateを採用。EC2/ASGだと停止管理やインスタンス保守が残る）
- OCI（`environments/oci`）は **現行 staging としては使わない**（アーカイブ / 将来の実験用）

staging と本番は同じクラウド（AWS）だが、コンピュートは用途で分ける: staging=常時稼働向けのECS on EC2、production=断続稼働向けのECS on Fargate。両者ともALBでドメイン/HTTPSを提供する点は共通。

## 意図

- staging を常時使える開発環境にする（OCI ではなく AWS）
- 本番は使わない日は止めてコスト抑制
- 展示・説明会などは指定日リストで終日 ON

## 構成図

正本は draw.io（AWS 公式アイコン）:

| 図 | ファイル |
|----|----------|
| 全体像 | [`aws-infra-overview.drawio.xml`](./aws-infra-overview.drawio.xml) |
| Staging 詳細 | [`aws-staging-architecture.drawio.xml`](./aws-staging-architecture.drawio.xml) |
| Production 詳細 | [`aws-production-architecture.drawio.xml`](./aws-production-architecture.drawio.xml) |
| PNG 書き出し | [`notion-diagrams/`](./notion-diagrams/) |

```text
[開発・検証] 常時
  GitHub → ECR → AWS staging ECS on EC2 + ALB + RDS + S3
           infra/terraform/environments/staging
           NAT なし / ALB あり

[本番] 既定停止
  GitHub → ECR → AWS prod ECS on Fargate + ALB + RDS + S3
  起動:
    (A) 本番反映で明示指定 → 起動してデプロイ
    (B) 指定日リスト → その暦日(JST)は終日起動
  停止: 指定日以外（反映ロック中を除く）
```

---

## staging（AWS・常時）

### 方針

- Terraform: `infra/terraform/environments/staging`（既存実装を正とする）
- コンピュート: ECS on EC2、NAT なし、RDS あり、ALB あり（ドメイン/HTTPS用）
- **常時 `desired>=1`**（コスト優先構成のまま止めない）
- 手順: `infra/terraform/environments/staging/README.md`

### 受け入れ条件

- [ ] `terraform apply` で staging の VPC / RDS / S3 / ECS / ALB が立つ
- [ ] FE/BE が常時到達できる（`https://stg.shukatsu-ai.jp` / `https://api-stg.shukatsu-ai.jp`）
- [ ] NAT Gateway が無い
- [ ] 既存本番（現行 EC2 等）を誤って変更しない

### コスト目安

- 常時 staging: おおよそ **¥8,000〜15,000 / 月**（NATなし）+ ALB追加分（目安 $16〜20/月）
- 詳細: `aws-ecs-ec2-cost-estimate.md`（**ALBなし試算のため要再計算**）

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
| `infra/terraform/environments/staging` | **★ staging 正（常時・ECS on EC2 + ALB）** |
| `infra/terraform/bootstrap` | AWS remote state |
| `infra/terraform/modules/{network,rds,s3,secrets,alb}` | staging / prod 共用 |
| `infra/terraform/modules/ecs_cluster` / `ecs_service` | staging専用（EC2/ASG） |
| `infra/terraform/modules/ecs_service_fargate` | prod専用（Fargate） |
| `infra/terraform/environments/prod` | **本番 root（ECS on Fargate 化済み）**。既定 `*_desired_count=0`（停止） |
| `infra/terraform/environments/oci` | **非正**（使わない。整理時に archive） |

## ドメイン紐付け

- staging: `stg.shukatsu-ai.jp`（frontend） / `api-stg.shukatsu-ai.jp`（backend）
- production: `shukatsu-ai.jp`（frontend） / `api.shukatsu-ai.jp`（backend）
- いずれも既存 Route53 ホストゾーン（`shukatsu-ai.jp`）内に ALIAS レコードで ALB を指す。ALBがACM証明書(DNS検証)でHTTPS終端し、ホストヘッダーでfrontend/backendにルーティングする
- Fargateはタスク再起動のたびにIPが変わるため、固定DNSを得るにはALBが実質必須という判断（staging/prod共通化）

## リスクと緩和

| リスク | 緩和 |
|--------|------|
| staging 常時の月額 + ALB固定課金 | NAT なし構成は維持。ALBは常時課金だがドメイン安定化のためのトレードオフとして受容 |
| 本番指定日の入れ忘れ | 展示前チェックリストに uptime dates を入れる |
| 反映中の自動停止 | maintenance lock |
| 既存の旧 EC2 本番に対して Fargate 版 `prod` を apply すると破壊的移行になる | 事前に `terraform plan` をレビューし、チームで移行タイミングを合意してから apply |
| 本番停止中もALBは課金され続ける（「動いた分だけ課金」の効果が部分的に相殺される） | 許容範囲として明示。完全ゼロ化したい場合はALBごとdestroy/re-applyする運用も選択肢（次回起動が遅くなるトレードオフ） |

## 次のアクション

1. **AWS staging を常時で apply**（bootstrap → staging）。OCI は触らない
2. staging にアプリ（ECR イメージ）を載せて常時検証できる状態にする
3. **本番**: `environments/prod`（ECS 化・ドメイン紐付け済み）を実際にレビュー→ apply。明示起動 + 指定日終日の自動制御は別タスクとして残っている（現状は `ecs_desired_capacity` の手動 apply で起動/停止）
4. infra 整理: OCI を `archive/` へ（任意・後続）
