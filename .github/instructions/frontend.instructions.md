---
applyTo: "frontend/**/*.{ts,tsx}"
---

Frontend の変更では型安全を最優先し、`any` は避けてください。
Next.js App Router と既存の MUI ベースUIのパターンに合わせて実装してください。
副作用は Hooks に閉じ込め、表示ロジックとデータ処理を分離してください。

## API エラーハンドリング（CEATEC 品質基準 #819）

クライアントから API を呼ぶ際は、失敗を空データ・未検出 UI に落とさないこと。

- `fetch` の `.catch(() => {})` や `!ok` 時に `null` を握りつぶす実装は避ける
- **404** と **ネットワーク/5xx** を区別する（404 だけ「見つかりません」、他はエラー＋再試行）
- 空状態（「データがありません」）は **取得成功かつ件数 0** のときだけ表示
- バックエンド直叩きは `/api/*` プロキシ経由に統一（同一オリジン・CORS 回避）
- 日付の `toISOString().slice(0, 10)` は UTC 日付になるため、暦日はローカル変換（`lib/datetime-local`）を使う

## CSP / 同一オリジン棚卸し（#819）

本番 CSP の `connect-src` は `'self'` + `NEXT_PUBLIC_BACKEND_URL`（ビルド時）。ブラウザから **オリジン外 fetch** すると CSP で落ちる。

| 種別 | 方針 | 現状 |
|------|------|------|
| 画面からの API | `/api/*` Next プロキシ | ES（#772）、応募、企業、スケジュール等 |
| 画面からの直接 `BACKEND_URL` fetch | 新規禁止。既存はプロキシ化するか CSP にオリジンを含める | `lib/interview.ts`、`lib/auth.ts`、`profile` の Calendar、`github-skills` |
| OAuth の `window.location` 遷移 | バックエンドへフル遷移してよい | ログイン、GitHub 連携 |

新規画面は `/api/*` プロキシを足す。`NEXT_PUBLIC_BACKEND_URL` 未設定の本番ビルドは `connect-src` に API が乗らず落ちる。

