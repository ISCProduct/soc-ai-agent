# データプライバシー設計

## 概要

集合知レコメンド（#205）および退会・個人データ削除（#613）におけるプライバシー設計方針をまとめます。

---

## 退会・ユーザーデータ削除（#613）

### API

| 操作 | メソッド / パス | 認証 |
|------|-----------------|------|
| 本人退会 | `DELETE /api/users/me` | `X-User-Token`（JWT） |
| 本人退会（互換） | `DELETE /api/auth/account` | 同上 |
| 管理者削除 | `DELETE /api/admin/users/:id` | `X-Admin-Email` / `X-Admin-Token` |

いずれも **ハード削除**（物理削除）です。PII をソフトデリートで残しません。

### 削除対象

| レイヤー | 内容 |
|---------|------|
| DB | チャット・スコア・応募・面接セッション/発言/レポート/質問状態/動画メタ・履歴書およびレビュー・GitHub・スケジュール・Google トークン・`user_embeddings` 等、ユーザーに紐づく行 |
| S3 | 面接動画（`DriveFileID`）、履歴書の `stored_path` / `normalized_path` / `annotated_path`（`s3://` キー） |
| 集合知 | 当該ユーザーの `anonymous_user_id`（決定論的 SHA-256）に一致する `collective_insight_logs` |
| ベクトルストア（Chroma） | **ユーザー単位の埋め込みは存在しない**ため対象外（企業コンテキスト等のみ） |

S3 削除は DB コミット後の best-effort です。失敗時は監査ログ `metadata.s3_errors` に記録し、DB はロールバックしません（個人データは DB から既に除去済み）。

### 監査ログ

| Action | 実行者 |
|--------|--------|
| `user.self_delete` | 本人（メールを actor に記録） |
| `user.admin_delete` | 管理者（`X-Admin-Email`） |

`GET /api/admin/audit-logs` で確認できます。

---

## 匿名化方式

### userIDのハッシュ化

個人を特定できないよう、`CollectiveInsightLog` に保存する `anonymous_user_id` は以下の形式でSHA-256ハッシュ化します:

```
anonymous_user_id = SHA-256("user:{userID}:collective")
```

**特徴:**
- 元のuserIDは復元不可能
- 同一ユーザーのログは一貫したハッシュで紐付け可能（集計に必要）
- ハッシュ化には固定のプレフィックス・サフィックスを使用し、他のシステムのハッシュと衝突しにくい設計
- 退会時は同一ハッシュのログを削除する（#613）

---

## 同意管理

### ユーザー同意フラグ

`users.allow_collective_insight` フラグ（デフォルト: `true`）で管理します。

| フラグ | 動作 |
|-------|------|
| `true`（デフォルト） | 行動ログを匿名化して記録 |
| `false` | 行動ログは一切記録しない |

### 同意変更API

```
PUT /api/collective-insights/consent
{
  "user_id": 123,
  "allow": false
}
```

**注意:** 同意取り消し後も、退会しない限り過去の匿名ログは残ります。退会時は上記のとおりハッシュ一致分を削除します。

---

## 記録するデータの範囲

`CollectiveInsightLog` に保存されるデータ:

| フィールド | 内容 | 個人特定可能性 |
|----------|------|--------------|
| `anonymous_user_id` | SHA-256ハッシュ | 不可 |
| `company_id` | 企業ID（整数） | 不可 |
| `action_type` | viewed / applied / passed / rejected | 不可 |
| `score_snapshot` | カテゴリ別スコア（JSON） | 不可（スコアのみ） |
| `created_at` | タイムスタンプ | 単独では不可 |

**記録しないデータ:**
- 氏名・メールアドレス・電話番号
- セッションID・IPアドレス
- 具体的な応募内容・面接内容

---

## 集合知レコメンドで使用するデータ

レコメンド生成時には以下のみを使用します:

1. 匿名ユーザーハッシュの一覧（類似度計算用）
2. 各ハッシュに紐づく企業IDと行動タイプ（通過/応募）
3. スコアスナップショット（コサイン類似度計算用）

個人の氏名・連絡先・具体的な応募内容は使用しません。

---

## データ保持ポリシー

| データ | 保持期間 | 備考 |
|-------|----------|------|
| ユーザー PII（プロフィール・チャット・面接・履歴書等） | 退会まで | 退会時に即時ハード削除（#613） |
| 面接動画・履歴書ファイル（S3） | 退会まで | 退会時にオブジェクト削除 |
| `collective_insight_logs` | 退会時に当該ハッシュ分を削除 / 残分は最大2年推奨 | バッチで古い行を削除可 |
| `anonymized_behavior_summaries` | 無期限 | 集計済みのため個人特定不可 |
| `score_calibration_weights` | 無期限 | バージョン管理で履歴として保持 |
| `audit_logs`（削除記録） | 無期限（推奨） | 法令対応・インシデント調査用。削除済みユーザーのメールが metadata に残る点に注意 |

### 古い集合知ログの削除（例）

```sql
DELETE FROM collective_insight_logs
WHERE created_at < DATE_SUB(NOW(), INTERVAL 2 YEAR);
```

---

## 今後の対応事項（ToDo）

- [ ] プライバシーポリシーへの「集合知分析への参加」条項の追加
- [ ] ユーザー登録フロー・設定画面への同意UI追加
- [x] データ削除リクエスト（忘れられる権利）対応のAPIエンドポイント追加（#613）
- [ ] 個人データ取り扱い台帳への `CollectiveInsightLog` の追記
- [ ] 面接動画の自動 TTL（例: 90日）ジョブ
- [ ] JWT 失効リスト（退会直後のトークン無効化）
