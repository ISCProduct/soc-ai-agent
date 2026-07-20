# DBマイグレーション運用 (#614)

## 概要

DBスキーマは [golang-migrate](https://github.com/golang-migrate/migrate) によるバージョン管理型マイグレーションで管理します。
GORM AutoMigrate は廃止済みで、**スキーマ変更は必ず `Backend/migrations/` のSQLファイルで行います**。

- マイグレーションファイルはバイナリに埋め込まれる（`embed.FS`）ため、デプロイに追加ファイルは不要
- サーバー起動時に未適用のマイグレーションが自動適用される（MySQLの `GET_LOCK` により複数インスタンスの同時起動でも安全）
- 適用状態は `schema_migrations` テーブルで管理される

## Docker Compose での自動適用

ローカル開発（`compose.yml`）では `app` コンテナ起動時に

`Backend/scripts/docker-entrypoint.dev.sh` → `go run ./cmd/migrate up` → `air`

の順でマイグレーションが自動実行されます。

```bash
docker compose up -d --build db app frontend
# または
make core-up
```

dirty 状態で止まっている場合も、当該バージョンのスキーマが揃っていれば自動修復して続行します。
手動確認は次のとおりです。

```bash
docker compose --profile tools run --rm migrate
# またはコンテナ内
docker compose exec app go run ./cmd/migrate version
```

## コマンド（ホストから直接）

```bash
cd Backend
DB_HOST=127.0.0.1 go run ./cmd/migrate            # up: 未適用のマイグレーションをすべて適用（デフォルト）
DB_HOST=127.0.0.1 go run ./cmd/migrate down       # 直近のマイグレーションを1つロールバック
DB_HOST=127.0.0.1 go run ./cmd/migrate version    # 現在のバージョンと dirty フラグを表示
DB_HOST=127.0.0.1 go run ./cmd/migrate force <N>  # バージョンを強制設定（dirty 復旧用・通常は使わない）
```

`SEED_DATA=true go run ./cmd/migrate` で up 後に初期データを投入します。

## スキーマ変更の手順

1. `Backend/migrations/` に連番でファイルを追加する

   ```
   000002_add_xxx_to_users.up.sql    # 変更を適用するSQL
   000002_add_xxx_to_users.down.sql  # 変更を取り消すSQL
   ```

2. **up/down は必ずペアで作成する**（downがないとロールバックできない）
3. GORMモデル（`internal/models/`）も同じPRで変更し、スキーマと一致させる
4. ローカルで `go run ./cmd/migrate` → 動作確認 → `go run ./cmd/migrate down` → 再度 up でロールバック可能性を確認する

### 書き方の注意

- 1ファイル1目的（1テーブルの変更、1インデックスの追加など）に分割する
- データ移行（UPDATE等）とスキーマ変更は別ファイルに分ける
- MySQLのDDLはトランザクションで巻き戻せないため、失敗時は `version`（dirtyフラグ）を確認して手動復旧する（下記）

## ロールバック手順

```bash
# 1. 現在のバージョンを確認
go run ./cmd/migrate version
# => Current version: 2 (dirty: false)

# 2. 直近のマイグレーションを1つ戻す
go run ./cmd/migrate down
# => Current version: 1 (dirty: false)
```

アプリのバージョンも合わせて戻す場合は、旧イメージへの切り戻しとセットで実施します
（旧バイナリには新しいマイグレーションが埋め込まれていないため、DBを先に戻します）。

## dirty 状態からの復旧

マイグレーション実行が途中で失敗すると `dirty: true` になり、以後の適用が止まります。

```bash
go run ./cmd/migrate version
# => Current version: 2 (dirty: true)
```

1. 失敗したマイグレーション（version 2）のSQLがどこまで適用されたかDBを直接確認する
2. 手動でSQLを補修する（残りを適用する or 適用済み分を取り消す）
3. 実際の状態に合わせてバージョンを設定する

```bash
go run ./cmd/migrate force 2   # version 2 が適用済みの状態に補修した場合
go run ./cmd/migrate force 1   # version 2 を取り消した状態に補修した場合
```

## 既存DB（AutoMigrate時代）の移行

`schema_migrations` テーブルが存在せず、かつ `users` テーブルが存在するDBは、
初回の up 実行時に自動的に **version 1（初期スナップショット）適用済み** として記録されます（ベースライン処理）。
既存の本番・開発DBに対して特別な作業は不要です。

## 初期スナップショットについて

`000001_init_schema.up.sql` は、AutoMigrate廃止時点（2026-07-15）のモデル定義を
空のMySQL 8.0に適用し `mysqldump --no-data` から整形して生成したものです（63テーブル）。
以後のスキーマ変更はすべて `000002` 以降の差分ファイルで行います。

## マイグレーション一覧

| Version | 内容 |
|---------|------|
| 1 | 初期スキーマスナップショット |
| 2 | `user_refresh_tokens` |
| 3 | 退会（`withdrawn_at` / `withdrawn_users`） |
| 4 | マルチテナント（`organizations` / memberships / 主要テーブルの `organization_id`）→ [multitenancy.md](./multitenancy.md) |
| 5 | 主要テーブル `organization_id` への FK 制約 |
