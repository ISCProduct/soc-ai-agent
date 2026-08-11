# deploy-smoke（反映後スモークテスト）

反映後に人手で「動作チェックお願いします」と依頼する代わりに、Playwrightで実環境向けの非破壊スモークを自動実行する。親Issue: #808 / プラン分割: #809

現在実装済みなのは **プラン1（staging スモーク基盤）** のみ。Discord通知（プラン2）、デプロイ連携・production対応（プラン3）は未実装。

## 実行方法

GitHub Actions の `Deploy Smoke Test`（`.github/workflows/deploy-smoke.yml`）を `workflow_dispatch` で手動起動する。`environment` に `staging` を選択する（現状はこれのみ）。

```
gh workflow run deploy-smoke.yml -f environment=staging
```

## 対象URL

| 環境 | フロントエンド | バックエンドAPI |
|------|----------------|------------------|
| staging | `https://stg.shukatsu-ai.jp` | `https://api-stg.shukatsu-ai.jp` |

`vars.STAGING_BASE_URL` / `vars.STAGING_API_BASE_URL`（リポジトリ Variables）で上書き可能。未設定時は上表のデフォルト値を使う。

## チェック内容（非破壊のみ）

`frontend/e2e/smoke/staging.spec.ts` 参照。

- トップページへの到達
- ログイン画面の表示
- バックエンド `/healthz` の疎通
- フロントエンド `/api/healthz` の疎通

データの作成・削除を伴う操作は行わない（Out of scope: #808）。

## ローカルでの実行

```bash
cd frontend
PLAYWRIGHT_BASE_URL=https://stg.shukatsu-ai.jp \
PLAYWRIGHT_API_BASE_URL=https://api-stg.shukatsu-ai.jp \
npx playwright test --config=playwright.smoke.config.ts
```

## 既存E2Eとの違い

`frontend/playwright.config.ts`（`.github/workflows/test.yml` の `e2e-frontend`）はlocalhostで `webServer` を起動し、APIをモックする単体〜結合レベルのE2E。今回の `playwright.smoke.config.ts` は実際に反映された環境（実バックエンド）に対して疎通確認するためのもので、完全に別設定・別ディレクトリ（`e2e/smoke/`）に分離している。既存E2Eには影響しない。

## 失敗時

`playwright-report` がArtifactとして保存される（保持7日）。Discordへの結果通知は未実装（プラン2で対応予定）。
