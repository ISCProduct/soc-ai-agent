# マルチテナント基盤（Organization）(#611)

## 概要

SaaS 向けに契約組織（テナント）単位でデータを分離するための基盤です。

組織（`organizations`）と学校（`schools`）は別の境界です。

- 画面のホスト名からフロントの middleware が `X-Tenant-Slug` を付け、バックエンドの `EchoTenantResolver` が slug で組織を解決する。クライアントが付けた `X-Tenant-Slug` は middleware が捨てて、ホストから付け直す。
- 学生の学校は `users.school_id`。学校名の自己申告から解決し、教員向けの一覧は `school_id` と `organization_id` の両方で絞る。
- 職員は `users.role = staff`。担当校は `admin_school_memberships`。担当校が無い職員は何も見えない。全校を見るのは `is_admin` のときだけ（`school.ResolveAccess`）。

組織側の分離は次のとおりです。

- `organizations` … テナント本体
- `organization_memberships` … ユーザー所属と組織内ロール（`owner` / `admin` / `member`）
- 主要ユーザーデータに `organization_id` を付与し、リポジトリ層でクロステナント参照を遮断

## データモデル

| テーブル | 役割 |
|---------|------|
| `organizations` | `name`, `slug`(unique), `status`(`active`/`disabled`) |
| `organization_memberships` | `(organization_id, user_id)` unique。現状は **ユーザーあたり1組織**（`user_id` UNIQUE） |
| `users.organization_id` | 所属組織（NOT NULL）。新規登録時はデフォルト組織へ自動所属 |
| `chat_messages` / `user_weight_scores` / `interview_sessions` / `interview_videos` / `resume_documents` | 組織スコープ列 |

### デフォルト組織

マイグレーション適用時に以下を seed します。

- ID: `1`
- slug: `default`
- name: `Default Organization`

既存ユーザーはすべてこの組織へバックフィルされ、メンバーシップが作成されます（`is_admin` ユーザーは `owner`）。

## API（プラットフォーム管理者）

認証: `X-Admin-Email` / `X-Admin-Token`

| 操作 | メソッド / パス |
|------|-----------------|
| 一覧 | `GET /api/admin/organizations` |
| 作成 | `POST /api/admin/organizations` `{ "name", "slug" }` |
| 取得 | `GET /api/admin/organizations/:id` |
| 更新 | `PUT /api/admin/organizations/:id` `{ "name?", "status?" }` |
| メンバー一覧 | `GET /api/admin/organizations/:id/members` |
| メンバー追加 | `POST /api/admin/organizations/:id/members` `{ "user_id", "role?" }` |
| ロール変更 | `PUT /api/admin/organizations/:id/members/:user_id` `{ "role" }` |
| メンバー削除 | `DELETE /api/admin/organizations/:id/members/:user_id` |

## スコープ強制

1. `EchoUserAuth` が JWT 検証後に `organization_id` をリクエストコンテキストへ載せる
2. チャット / 面接 / 履歴書の取得は `OrganizationService.Get*ForOrganization` で **自組織以外は `ErrCrossOrganization`（403）**
3. Create 系リポジトリは `user_id` から `organization_id` を自動解決して保存

## マイグレーション

```
Backend/migrations/000004_add_organizations.up.sql
Backend/migrations/000004_add_organizations.down.sql
```

```bash
cd Backend && go run ./cmd/migrate
```

## 移行計画（既存環境）

1. デプロイ前に DB バックアップ
2. アプリ起動時（または `cmd/migrate`）で 000004 を適用
3. 全ユーザーが `organization_id=1` かつ membership を持つことを確認
4. 新規契約組織は管理 API で作成し、メンバーを移動（現状はユーザー1組織のため、移動は membership 削除→別組織へ追加）

## Phase 2（未実装）

- 組織切り替え UI / 組織管理者コンソール
- カタログ（企業マスタ等）のテナント化
- 複数組織所属
- JWT への org claim 埋め込み
