# Notion ドキュメント

リポジトリ内の設計・運用ドキュメントを Notion に同期するためのファイル置き場です。

## 構成図の方針

- **正本**: `docs/architecture/*.drawio.xml`（draw.io / AWS 公式アイコン）
- **PNG**: `docs/architecture/notion-diagrams/*.png`（Notion 埋め込み用）
- HTML 埋め込みは Notion サンドボックスで外部アイコンがブロックされるため **使わない**

diagrams.net で XML を開くと AWS アイコン付きで編集・閲覧できます。

## ドキュメント一覧

| ファイル | 内容 |
|----------|------|
| `617-redis-rate-limit-jobs.md` | #617 実装計画・運用まとめ |
| `infra-decision-aws-staging-prod.md` | AWS staging 常時 + 本番指定起動の方針 |
