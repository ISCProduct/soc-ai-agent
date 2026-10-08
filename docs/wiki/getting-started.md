# ローカル開発環境構築ガイド

このガイドに沿って進めると、新規メンバーが SOC AI Agent を初めてローカルで動作させるまでの手順を完結できます。

---

## 前提条件

以下のツールがインストールされていることを確認してください。

| ツール | 推奨バージョン | 確認コマンド |
|-------|-------------|------------|
| Go | 1.25 以上 | `go version` |
| Node.js | 18 以上 | `node --version` |
| Python | 3.10 以上 | `python3 --version` |
| Docker | 24 以上 | `docker --version` |
| Docker Compose | v2 (Plugin) | `docker compose version` |
| Git | — | `git --version` |
| GitHub CLI（任意） | 2.x | `gh --version` |

> **Docker Compose v2 について**: `docker-compose` コマンド（ハイフンあり）ではなく `docker compose`（ハイフンなし）を使用します。v1 がインストールされている場合は v2 に更新してください。

---

## 1. リポジトリのクローン

```sh
git clone https://github.com/ISCProduct/soc-ai-agent.git
cd soc-ai-agent
```

---

## 2. 環境変数の設定

### バックエンド（`Backend/.env`）

```sh
cp Backend/.env.example Backend/.env
```

`.env` を開いて以下の項目を設定します。

```env
# MySQL 接続情報（Docker Compose 環境ではデフォルト値のままで動作）
DB_USER=app_user
DB_PASSWORD=app_pass
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=app_db

# サーバーポート
SERVER_PORT=8080

# 管理者・ユーザー JWT シークレット（開発時は任意の文字列で可）
ADMIN_SECRET=change-me-admin-secret
USER_SECRET=change-me-user-secret

# OpenAI API キー（必須）
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OPENAI_MODEL=gpt-4o-mini

# CORS（開発時は localhost を許可）
ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000

# GitHub トークン暗号化キー（AES-256-GCM、64桁の hex）
# 未設定でも動作するが警告ログが出る
TOKEN_ENCRYPTION_KEY=your-64-char-hex-key-here

# RAG サービス URL（Docker Compose 環境では自動解決）
RAG_REVIEW_URL=http://rag-review:9000
```

> **最小構成**: `OPENAI_API_KEY` のみ設定すれば、AI 機能を除くほとんどの機能が動作します。

### フロントエンド（`frontend/.env.local`）

```sh
cp frontend/.env.local.example frontend/.env.local
```

```env
NEXT_PUBLIC_BACKEND_URL=http://localhost:8080
NEXT_PUBLIC_INTERVIEW_MAX_MINUTES=10
NEXT_PUBLIC_INTERVIEW_MAX_COST_USD=1.8
NEXT_PUBLIC_INTERVIEW_COST_PER_MIN_USD=0.18
```

---

## 3. Docker Compose での起動（推奨）

**最も簡単な方法**です。まずコア（バックエンド・フロントエンド・MySQL）を起動し、RAG が必要なときは **profile `rag`** で Chroma + rag-review を追加します。

```sh
# ルートディレクトリから実行
docker compose up -d --build

# RAG + 独立 Chroma（履歴書レビュー / 面接 hints で必須）
# どちらも既定サービスなので上の up -d で一緒に起動する。
# 作り直したいときだけ:
make rag-up
```

| サービス | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| Backend API | http://localhost:8080 |
| Chroma | http://localhost:8000 |
| RAG | http://localhost:9000 |

### ヘルスチェック

```sh
curl http://localhost:8080/healthz
# → {"status":"ok"}

curl http://localhost:8000/api/v2/heartbeat
# → {"nanosecond heartbeat":...}

curl http://localhost:9000/health
# → {"status":"ok","vector_store":{"ok":true,"detail":"chromadb http://chroma:8000"}}

curl http://localhost:9000/vector/status
# → collections / total_documents を含む JSON
```

`vector_store` が無い、または `/vector/status` が 404 の場合は旧イメージです。

```sh
make rag-rebuild
```

### ログ確認

```sh
docker compose logs -f app       # バックエンド
docker compose logs -f frontend  # フロントエンド
docker compose logs -f chroma rag-review
```

> **注意**: プロジェクトルートには `docker-compose.yml`（ハイフンあり）と `compose.yml` の2つが存在します。
> - `compose.yml` → **ローカル開発用**（デフォルトで `docker compose` が読み込む）
> - `docker-compose.yml` → **本番環境用**（AWS ECR / RDS に接続する）
>
> ローカル開発では必ず `compose.yml` を使用してください。誤って `docker-compose.yml` を指定すると本番 DB に接続しようとします。

---

## 4. 各サービスの個別起動（Docker を使わない場合）

### 4.1 MySQL のセットアップ

ローカル MySQL が必要です。Docker のみで MySQL を起動する場合:

```sh
docker compose up -d db
```

その後、マイグレーションを実行します。

```sh
cd Backend
go run ./cmd/migrate
```

### 4.2 バックエンド（Go）

```sh
cd Backend
go mod download      # 依存パッケージのダウンロード
go run ./cmd/server  # サーバー起動（http://localhost:8080）
```

### 4.3 フロントエンド（Next.js）

```sh
cd frontend
npm install          # 依存パッケージのインストール
npm run dev          # 開発サーバー起動（http://localhost:3000）
```

### 4.4 RAG サービス（Python）

RAG サービスは重いため、必要な場合のみ起動してください。

```sh
cd rag

# 依存インストール（requirements.txt の宣言を constraints.txt で縛る）
pip install -r requirements.txt -c constraints.txt

# サービス起動（http://localhost:9000）
python3 main.py
```

> **重要**: 必ず `-c constraints.txt` を付けてください。`pip install -r constraints.txt`（constraints を requirements として渡す形）だと、`requirements.txt` にしか宣言が無いパッケージ（uvicorn 等）が推移的依存として最新版で入り、宣言と実インストールが乖離します（#1159）。乖離は `rag/tests/test_dependency_constraints.py` が検知して落ちます。

---

## 5. DB マイグレーション

バックエンドは GORM の `AutoMigrate` を使用しており、サーバー起動時に自動的にテーブルが作成・更新されます。

手動でマイグレーションのみ実行する場合:

```sh
cd Backend
go run ./cmd/migrate
```

---

## 6. テストの実行

### バックエンド（Go）

```sh
cd Backend
go test ./internal/... ./migrations/...   # 本体のテスト（対象パッケージの隣に配置）
go test ./test/...                        # 複数パッケージ横断のテストのみ（4ファイル）
go test ./...                             # 全テスト
```

> テストは対象パッケージの隣に置きます（例: `internal/controllers/admin/*_test.go`）。
> `Backend/test/` に残しているのは、対象が1パッケージに定まらないものだけです。

### フロントエンド

```sh
cd frontend
npm run lint          # ESLint チェック
npm run build         # 本番ビルド（型チェック含む）
```

### E2E テスト（Playwright）

```sh
cd frontend
npx playwright test
```

---

## 7. よくある問題

| 症状 | 原因 | 対処 |
|------|------|------|
| `DB接続エラー` | MySQL が未起動 / `.env` の設定ミス | `docker compose up -d db` を実行、または `.env` を確認 |
| `OPENAI_API_KEY が未設定` | AI 機能を使う場合に必要 | `.env` に `OPENAI_API_KEY` を設定 |
| `rag-review 起動失敗` | 依存パッケージのバージョン不一致 | `pip install -r requirements.txt -c constraints.txt` で作り直す |
| `CORS エラー（開発時）` | `ALLOWED_ORIGINS` 未設定 | `.env` に `ALLOWED_ORIGINS=http://localhost:3000` を追加 |
| `フロントビルド失敗` | Node.js バージョンが古い | Node.js 18 以上を使用（`nvm use 18` 等） |
| `TOKEN_ENCRYPTION_KEY 警告` | GitHub 連携に必要 | 64 桁の hex キーを生成して設定（`python3 -c "import secrets; print(secrets.token_hex(32))"`) |
| `S3アップロード失敗` | IAM 権限不足 | `s3:PutObject` / `s3:GetObject` 権限を確認 |
| `frontend が EACCES で起動しない`（Linux） | ホストの UID が 1000 以外で、bind mount した `./frontend` に `next dev` が `next-env.d.ts` を書けない | `.env` に `FRONTEND_USER=$(id -u):$(id -g)` を設定し、下記のボリューム作り直しも行う |
| `rag-review が /data で PermissionError` | root 実行時代のデータが `rag_data` に残っている（`RAG_CHROMA_DATA_DIR=/data/...` を使う場合のみ） | `docker compose run --rm --user root rag-review chown -R 10001:10001 /data`（ファイルは消えない） |
| `チャットで「送信に失敗しました」が出る` | `schema_migrations` のバージョンは最新なのに、実際の列が無い。`POST /api/chat` が 500 を返している | backend のログに `Error 1054 (42S22): Unknown column '...'` が出ていれば該当。下記「スキーマがマイグレーションと食い違うとき」の手順で埋める |
| `ブラウザに「接続がリセットされました」が出る`（開発時） | `next dev` が cgroup の OOM killer に殺され、`restart: unless-stopped` で再起動している。落ちた瞬間に接続が切れるため、利用者にはリロードが要るように見える | `docker inspect soc-ai-agent-frontend --format '{{.RestartCount}}'` が増えていれば該当。**`docker events --since 10m --filter event=oom` に記録が残る**（子プロセスだけ殺されるので `docker inspect` の `OOMKilled` は `false` のまま。ここだけ見ると見逃す。`--since` が無いと以後の新しいイベントを待つだけで、落ちたあとに叩いても出てこない）。`.env` の `FRONTEND_MEM_LIMIT` を上げる |
| `frontend が EACCES: mkdir '/app/.next/dev' で起動ループ` | イメージが古く `/app/.next` を含まないため、名前付きボリュームが root 所有の空で作られた。`node`(uid 1000) で動くコンテナが書けない | イメージを作り直してからボリュームを消す（下記の手順）。`docker compose up` だけでは直らない |

### スキーマがマイグレーションと食い違うとき

`schema_migrations.version` は最新を指しているのに、その版で入るはずの列が無いことがある。
`migrate` は版だけ見るので再実行しても埋まらず、`dirty` も立たないため気づけない。

症状はアプリ側の汎用エラー（チャットなら「送信に失敗しました」）で、
原因は backend のログにだけ出る。

```sh
# 1. 何が足りないかを特定する（列名はログの Unknown column に出る）
docker compose logs --tail=200 app | grep "Unknown column"

# 2. 実際に当たっているか確認する
docker compose exec -T db sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" app_db' <<'SQL'
SELECT version, dirty FROM schema_migrations;
SELECT COLUMN_NAME FROM information_schema.COLUMNS
 WHERE TABLE_SCHEMA='app_db' AND TABLE_NAME='chat_messages';
SQL

# 3. 該当マイグレーションの DDL だけを流して実体を合わせる
#    （version は既に先を指しているので、版は動かさない）
sed -n '/^ALTER TABLE/,$p' Backend/migrations/000030_chat_message_weight_category.up.sql   | docker compose exec -T db sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" app_db'
```

staging・本番はデプロイのたびに `migrate up` が走る（`deployment.yml`）ので、
この食い違いはローカルで起きる。DB を長く使い回している環境で出やすい。

### frontend の dev サーバが繰り返し落ちるとき

`next dev --webpack` は、定常では 1.8GiB 程度だが、大きいページ
（`app/admin/companies/page-content.tsx` は 1,583 行）を組み直すときに
**瞬間的に 2.5GiB を超える**。実測で 2.493GiB を確認している。

`mem_limit` が足りないとその瞬間に cgroup の OOM killer が `next dev` を殺し、
親の `npm` は 0 で終わるため **`docker inspect` の `OOMKilled` は `false` のまま**になる。
見分けるには `docker events --since 10m --filter event=oom` を見る。
`--since` を付けないと以後の新しいイベントを待つだけなので、
落ちたあとに叩いても既に起きた OOM は出てこない。

`NODE_OPTIONS=--max-old-space-size` は V8 のヒープだけの上限で、
webpack のネイティブ確保は含まない。ヒープを絞っても跳ねは止まらないので、
**`mem_limit` との差を広く取る**こと（既定はヒープ 1536m / 上限 3584m）。

### コンテナ非root化（#1477）にともなう既存環境の移行

全コンテナを非rootで動かすようにしたため、**root で動いていた頃に作られたボリュームが残っていると書き込みに失敗する**。

- **frontend の `.next` / `node_modules`**: 匿名ボリュームをやめて名前付き（`frontend_next` / `frontend_node_modules`）にしたので、**次の `docker compose up` で自動的に新しいボリュームが作られ移行が済む**。旧ボリュームは残るだけなので、回収したければ `docker volume prune`。
- **`FRONTEND_USER` を変えた場合**: 名前付きボリュームは `node`（uid 1000）所有で作られるため、作り直しが必要。どちらもビルドキャッシュなので消してよい。

  ```sh
  docker compose down
  docker volume rm "$(basename "$PWD")_frontend_node_modules" "$(basename "$PWD")_frontend_next"
  docker compose up -d --build
  ```

- **イメージが古い場合**: `/app/.next` を作る `RUN mkdir -p /app/.next && chown -R node:node /app` は後から入った。
  それ以前にビルドしたイメージを使っていると、名前付きボリュームが**root 所有の空**で初期化され、
  `EACCES: mkdir '/app/.next/dev'` で起動ループする。ボリュームを消すだけでは同じ状態が再発するので、
  **先にイメージを作り直してからボリュームを消す**。`node_modules` 側も同時に作り直さないと、
  古い依存が残って `Module not found` になる。

  ```sh
  docker compose stop frontend && docker compose rm -f frontend
  docker compose build frontend
  docker volume rm "$(basename "$PWD")_frontend_next" "$(basename "$PWD")_frontend_node_modules"
  docker compose up -d frontend
  ```

- **`rag_data`**: ChromaDB の実データが入りうるので**消さずに所有権だけ移す**。既定構成（`CHROMA_HOST=chroma`）では `/data` を使わないため、この作業が要るのは `RAG_CHROMA_DATA_DIR=/data/...` へ切り替えている環境だけ。

  ```sh
  docker compose run --rm --user root rag-review chown -R 10001:10001 /data
  ```

- **`mysql_data` / `chroma_data`**: 対象外。MySQL・Chroma の公式イメージは非root化していないので、これまでどおり動く。

---

## 8. 開発フロー

```sh
# Issue からブランチを作成
git checkout -b feat/issue-xxx-description

# 開発・コミット
git add .
git commit -m "feat: #xxx 変更内容"

# テスト実行
cd Backend && go test ./...
cd frontend && npm run lint

# PR を作成
gh pr create --title "Resolve #xxx: 機能説明" --base main
```

---

## 関連ドキュメント

- [システム概要](./overview.md) — アーキテクチャ・技術スタック
- [API リファレンス](./api-reference.md) — 全エンドポイント詳細
- [RAG サービス詳細](./rag-service.md) — RAG の仕組みと使い方
- [運用手順書](./operations.md) — デプロイ・障害対応
