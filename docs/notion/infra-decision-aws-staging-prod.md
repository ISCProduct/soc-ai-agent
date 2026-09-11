# インフラ方針: AWS staging（常時）+ AWS 本番（指定起動）

> **最終更新:** 2026-08-05  
> **リポジトリ:** `docs/architecture/infra-decision-oci-stg-aws-prod.md`

---

## 決定

| 環境 | 基盤 | 運用 |
|------|------|------|
| **staging** | AWS ECS on EC2 + ALB | **常時起動**。開発・検証の正環境 |
| **production** | AWS ECS on Fargate + ALB | **既定は停止**。本番反映時に明示起動。**指定日は終日（24h）起動** |

### 以前の方針からの変更

- staging = OCI → **staging = AWS（常時起動）**
- 本番 = ECS on EC2 → **本番 = ECS on Fargate**（断続運用向け）
- OCI（`environments/oci`）は現行 staging としては使わない

---

## 意図

- staging を常時使える開発環境にする
- 本番は使わない日は止めてコスト抑制
- 展示・説明会などは指定日リストで終日 ON

---

## 構成イメージ

```
[開発・検証] 常時
  GitHub → ECR → AWS staging ECS + RDS + S3
           infra/terraform/environments/staging

[本番] 既定停止
  GitHub → ECR → AWS prod ECS + RDS + S3
  起動:
    (A) 本番反映で明示指定 → 起動してデプロイ
    (B) 指定日リスト → その暦日(JST)は終日起動
```

---

## staging（AWS・常時）

- Terraform: `infra/terraform/environments/staging`
- ECS on EC2、NAT なし、RDS あり、ALB あり
- 常時 `desired>=1`
- ドメイン: `stg.shukatsu-ai.jp` / `api-stg.shukatsu-ai.jp`
- コスト目安: **¥8,000〜15,000 / 月**（NATなし）+ ALB

### 受け入れ条件

- [ ] `terraform apply` で VPC / RDS / S3 / ECS / ALB が立つ
- [ ] FE/BE が常時到達できる
- [ ] 既存本番を誤って変更しない

---

## 本番起動ポリシー

1. **既定状態は停止**（Fargate `desired=0`）
2. **毎日の昼間自動起動はしない**
3. 起動トリガー:
   - **(A)** 本番反映時の明示起動（`workflow_dispatch` 等）
   - **(B)** 指定日リスト（JST 00:00〜23:59:59 は終日起動）

### 受け入れ条件

- [ ] 既定は停止
- [ ] 反映時の明示起動→デプロイができる
- [ ] 指定日は終日起動が維持される
- [ ] 日付追加・削除手順がドキュメント化されている

---

## コードの位置づけ

| パス | 位置づけ |
|------|----------|
| `infra/terraform/environments/staging` | staging 正（ECS on EC2 + ALB） |
| `infra/terraform/environments/prod` | 本番（ECS on Fargate、既定停止） |
| `infra/terraform/environments/oci` | 非正（アーカイブ候補） |

---

## リスクと緩和

| リスク | 緩和 |
|--------|------|
| staging 常時の月額 + ALB | NAT なし維持。ALBはドメイン安定化のトレードオフ |
| 本番指定日の入れ忘れ | 展示前チェックリスト |
| 反映中の自動停止 | maintenance lock |
| Fargate 版 prod apply の破壊的移行 | plan レビュー・移行タイミング合意 |

---

## 次のアクション

1. AWS staging を常時で apply（bootstrap → staging）
2. staging にアプリを載せて常時検証
3. 本番 prod のレビュー→ apply。自動起動制御は別タスク
4. OCI を archive へ（任意）
