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

**最も簡単な方法**です。バックエンド・フロントエンド・MySQL・RAG サービスをまとめて起動します。

```sh
# ルートディレクトリから実行
docker compose up -d --build
```

| サービス | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| Backend API | http://localhost:8080 |
| RAG | http://localhost:9000 |

### ヘルスチェック

```sh
curl http://localhost:8080/healthz
# → {"status":"ok"}

curl http://localhost:9000/healthz
# → {"status":"ok"}
```

### ログ確認

```sh
docker compose logs -f app       # バックエンド
docker compose logs -f frontend  # フロントエンド
docker compose logs -f rag-review
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

# 依存インストール（constraints.txt で固定バージョンを使用）
pip install -r constraints.txt

# サービス起動（http://localhost:9000）
python3 main.py
```

> **重要**: RAG の依存パッケージは `constraints.txt` で固定されています。`requirements.txt` ではなく `constraints.txt` を使用してください。バージョンの違いで動作しなくなる場合があります。

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
go test ./internal/...      # 内部パッケージテスト
go test ./test/...          # 統合テスト・コントローラーテスト
go test ./...               # 全テスト
```

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
| `rag-review 起動失敗` | 依存パッケージのバージョン不一致 | `pip install -r constraints.txt` で固定バージョンを使用 |
| `CORS エラー（開発時）` | `ALLOWED_ORIGINS` 未設定 | `.env` に `ALLOWED_ORIGINS=http://localhost:3000` を追加 |
| `フロントビルド失敗` | Node.js バージョンが古い | Node.js 18 以上を使用（`nvm use 18` 等） |
| `TOKEN_ENCRYPTION_KEY 警告` | GitHub 連携に必要 | 64 桁の hex キーを生成して設定（`python3 -c "import secrets; print(secrets.token_hex(32))"`) |
| `S3アップロード失敗` | IAM 権限不足 | `s3:PutObject` / `s3:GetObject` 権限を確認 |

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
