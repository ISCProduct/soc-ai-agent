# deploy-smoke（反映後スモークテスト）

反映後に人手で「動作チェックお願いします」と依頼する代わりに、Playwrightで実環境向けの非破壊スモークを自動実行し、結果を Discord に投稿する。親Issue: #808 / プラン分割: #809

## プラン実装状況

| プラン | 内容 | 状態 |
|--------|------|------|
| 1 | staging 実URLスモーク + artifact | 実装済 |
| 2 | Discord 結果通知 + 担当者メンション | 実装済 |
| 3 | staging デプロイ後フック / production dispatch | 実装済（本番自動は未接続。ECS 未作成のため） |

## 実行方法

```
gh workflow run deploy-smoke.yml -f environment=staging
gh workflow run deploy-smoke.yml -f environment=production
```

staging は `deployment.yml` の `deploy-staging` 成功後にも自動起動する。production のデプロイ後フックは、本番 ECS を作ってから足す。

ラベル `stage:develop` / `stage:main` は従来の人手確認用。スモークが通れば「チェックして」依頼は不要。

## 対象URL

| 環境 | フロントエンド | バックエンドAPI |
|------|----------------|------------------|
| staging | `https://stg.shukatsu-ai.jp` | `https://api-stg.shukatsu-ai.jp` |
| production | `https://shukatsu-ai.jp` | `https://api.shukatsu-ai.jp` |

Variables: `STAGING_BASE_URL` / `STAGING_API_BASE_URL` / `PROD_BASE_URL` / `PROD_API_BASE_URL`

## チェック内容（非破壊のみ）

`frontend/e2e/smoke/staging.spec.ts`

- トップ到達、ログイン画面、Backend `/healthz`、Frontend `/api/healthz`
- データの作成・削除は行わない（本番フル E2E は Out of scope）

## Discord

Secret: `DISCORD_DEPLOY_WEBHOOK_URL`（コストアラート用とは分ける）

- 未設定 → 通知だけスキップ。デプロイもスモーク本体も落とさない
- 失敗時は `mention-map.json`（GitHub login → Discord snowflake）で `<@id>`。未登録なら `@here`
- 担当者 ID は `automation/deploy-smoke/mention-map.json` に追加する

```json
{ "oohashikazuyuki": "123456789012345678" }
```

## ローカル

```bash
cd frontend
PLAYWRIGHT_BASE_URL=https://stg.shukatsu-ai.jp \
PLAYWRIGHT_API_BASE_URL=https://api-stg.shukatsu-ai.jp \
npx playwright test --config=playwright.smoke.config.ts
```

## 既存E2E

`frontend/playwright.config.ts`（`test.yml` の localhost E2E）とは別設定。壊さない。
