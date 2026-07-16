# API リファレンス

## 認証

### 管理者API

`Authorization: Basic {base64(email:password)}` ヘッダーが必要です。

### ユーザーAPI

現在はクエリパラメータ `user_id` で識別します（将来的にJWT化予定）。

---

## 認証エンドポイント

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/auth/register` | メール登録 |
| POST | `/api/auth/login` | ログイン（`user_token` + `refresh_token` を返す） |
| POST | `/api/auth/refresh` | トークンリフレッシュ（ローテーション #616） |
| POST | `/api/auth/logout` | リフレッシュトークン失効 |
| POST | `/api/auth/verify-email` | メール認証 |
| POST | `/api/auth/forgot-password` | パスワードリセット要求 |
| GET | `/api/auth/google` | Google OAuth開始 |
| GET | `/api/auth/github` | GitHub OAuth開始 |

### トークン仕様（#616）

- **アクセストークン**（`user_token`）: JWT・有効期間 **1時間**。`X-User-Token` ヘッダーで送信
- **リフレッシュトークン**（`refresh_token`）: 不透明トークン・有効期間 **30日**・DB管理（SHA-256ハッシュ保存）
  - `/api/auth/refresh` のたびにローテーションされる（並行リクエスト対策で旧トークンは60秒の猶予あり）
  - ログアウト・パスワードリセット・アカウント削除で失効する
- フロントエンドは httpOnly Cookie（`user_token` / `refresh_token`）で保持し、`middleware.ts` が期限2分前に自動リフレッシュする

---

## チャット分析

| メソッド | パス | パラメータ | 概要 |
|---------|------|-----------|------|
| POST | `/api/chat/messages` | body: message, user_id, session_id | メッセージ送信・スコア更新 |
| GET | `/api/chat/scores` | ?user_id&session_id | 10カテゴリスコア取得 |
| GET | `/api/chat/companies` | ?user_id&session_id | マッチング企業一覧 |
| POST | `/api/chat/send-report` | body: user_id, session_id | 分析レポートメール送信 |

### スコアレスポンス例
```json
{
  "scores": [
    {"category": "技術志向", "score": 85},
    {"category": "チームワーク", "score": 72},
    {"category": "リーダーシップ", "score": 60}
  ]
}
```

---

## 面接

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/interviews` | セッション作成 |
| GET | `/api/interviews?user_id=xxx` | セッション一覧 |
| POST | `/api/interviews/{id}/start` | 開始（チャットスコアをAIプロンプトに注入） |
| POST | `/api/interviews/{id}/turn` | 1ターン実行（音声→テキスト→AI→音声） |
| POST | `/api/interviews/{id}/finish` | 終了・レポート生成キュー |
| GET | `/api/interviews/{id}/report` | レポート取得 |
| POST | `/api/interviews/{id}/upload-video` | 動画S3アップロード |
| POST | `/api/interviews/{id}/send-report` | レポートメール送信 |
| POST | `/api/realtime/token` | WebRTCセッショントークン取得 |

---

## 職務経歴書

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/resume/upload` | PDF アップロード（S3） |
| POST | `/api/resume/{id}/review` | レビュー実行（スコア更新も実施） |
| GET | `/api/resume/{id}/review/stream` | レビューSSEストリーミング |
| GET | `/api/resume/{id}/annotated` | 注釈済みPDF取得 |

---

## 選考管理（#201）

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/applications` | 応募登録 |
| GET | `/api/applications?user_id=xxx` | 選考一覧 |
| PUT | `/api/applications/{id}` | ステータス更新 |

### ステータス一覧
```
applied          → 応募済み
document_passed  → 書類通過
interview        → 面接中
offered          → 内定
accepted         → 内定承諾
declined         → 辞退
rejected         → 不合格
```

---

## 統合プロファイル（#204）

| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/user/profile?user_id=xxx&session_id=xxx` | 統合プロファイル取得 |

### レスポンス例
```json
{
  "user_id": 1,
  "chat_session_id": "session-abc123",
  "weight_scores": [...],
  "top_categories": [
    {"weight_category": "技術志向", "score": 85}
  ],
  "source_summary": {
    "has_chat_scores": true,
    "interview_count": 3,
    "resume_review_done": true
  }
}
```

---

## 集合知レコメンド（#205）

| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/collective-insights/recommendations?user_id=xxx&session_id=xxx` | 集合知レコメンド |
| GET | `/api/collective-insights/top-companies?limit=10` | 通過率上位企業 |
| PUT | `/api/collective-insights/consent` | 同意設定更新 |
| POST | `/api/collective-insights/actions` | 行動ログ記録 |

### レコメンドレスポンス例
```json
{
  "recommendations": [
    {
      "company_id": 42,
      "company_name": "株式会社Example",
      "pass_count": 8,
      "similar_users": 15,
      "collective_score": 53.3,
      "reason": "あなたと似たスコアプロファイルの8人が通過・応募した企業です"
    }
  ],
  "count": 5
}
```

### 行動ログ記録リクエスト例
```json
{
  "user_id": 1,
  "session_id": "session-abc123",
  "company_id": 42,
  "action_type": "applied"
}
```

---

## 管理者API

### 企業プロファイル再計算（#202）

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/admin/profile-recalculation/run` | 全企業を再計算 |
| POST | `/api/admin/profile-recalculation/{id}/run` | 指定企業を再計算 |
| POST | `/api/admin/profile-recalculation/{id}/rollback` | 1バージョン前に戻す |
| GET | `/api/admin/profile-recalculation/history/{id}` | 更新履歴 |

### スコア精度検証（#203）

| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/admin/score-validation/correlation` | スコアvs通過率相関レポート |
| GET | `/api/admin/score-validation/phase-metrics` | フェーズ別精度メトリクス |
| GET | `/api/admin/score-validation/calibration` | 現在の重み |
| POST | `/api/admin/score-validation/calibration/run` | キャリブレーション実行 |
| GET | `/api/admin/score-validation/calibration/history` | 重み履歴 |
| GET | `/api/admin/score-validation/variants` | 実験一覧 |
| POST | `/api/admin/score-validation/variants` | バリアント作成 |
| GET | `/api/admin/score-validation/variants/results?experiment=xxx` | バリアント結果 |

### 集合知バッチ（#205）

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/admin/collective-insights/rebuild-summaries` | 企業別サマリー再集計 |

### その他管理者API

| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/admin/companies` | 企業一覧 |
| POST | `/api/admin/companies` | 企業作成 |
| GET/PUT | `/api/admin/companies/{id}` | 企業詳細・更新 |
| GET | `/api/admin/interviews` | 面接セッション一覧 |
| GET | `/api/admin/dashboard/users` | ユーザーダッシュボード |
| GET | `/api/admin/dashboard/export/csv` | データCSVエクスポート |
| GET | `/api/admin/costs/summary` | APIコストサマリー |
| GET | `/api/admin/costs/daily` | 日別コスト |
| GET | `/api/admin/costs/monthly` | 月別コスト |
| GET | `/api/admin/audit-logs` | 監査ログ |

---

## ヘルスチェック

| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/health` | ヘルスチェック（後方互換） |
| GET | `/healthz` | ヘルスチェック（ECS/K8s標準） |

レスポンス: `{"status":"ok"}`
