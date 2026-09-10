# Notion ドキュメント

リポジトリ内の設計・運用ドキュメントを Notion に同期するためのファイル置き場です。

## 構成図の方針

- **ローカル正本**: `docs/architecture/*.drawio.xml`（draw.io / AWS 公式アイコン）
- **PNG**: `docs/architecture/notion-diagrams/*.png`（Notion 埋め込み用）
- `docs/architecture/` は **git 追跡しない**（`.gitignore`）。クローン後はローカルで保持するか Notion から取得する
- HTML 埋め込みは Notion サンドボックスで外部アイコンがブロックされるため **使わない**

diagrams.net で XML を開くと AWS アイコン付きで編集・閲覧できます。

## ドキュメント一覧

| ファイル | 内容 |
|----------|------|
| `prd-designdoc-format.md` | `/issue` 用 PRD / DesignDoc / 仕様書フォーマット正本（Notion DB 連携） |
| `617-redis-rate-limit-jobs.md` | #617 実装計画・運用まとめ |
| `infra-decision-aws-staging-prod.md` | AWS staging 常時 + 本番指定起動の方針 |
