# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

SOC AI Agent は、AIチャット分析・音声面接・職務経歴書レビュー・企業マッチングを統合した採用支援SaaSです。Go バックエンド + Next.js フロントエンド + Python RAG サービスの3層構成。

## コマンド

### バックエンド (Go)

```bash
cd Backend
go mod download
go run ./cmd/server          # 開発サーバー起動 (port 8080)
go test ./internal/... -v -race -timeout 60s  # テスト実行
go test ./internal/services/... -run TestXxx  # 単一テスト実行
go build ./...               # ビルド確認
```

### フロントエンド (Next.js)

```bash
cd frontend
npm install
npm run dev                  # 開発サーバー起動 (port 3000)
npm run build                # 本番ビルド
npm run lint                 # ESLint
npx playwright test          # E2E テスト
```

### RAG サービス (Python)

```bash
cd rag
pip install -r constraints.txt   # constraints.txt で固定バージョンを使用 (requirements.txt ではない)
python3 main.py              # FastAPI サーバー起動 (port 9000)
```

### Docker Compose (推奨)

```bash
docker compose up -d --build                    # 全サービス起動 (app/db/frontend)
docker compose --profile rag up -d rag-review   # RAGも起動
docker compose logs -f app                      # ログ確認
```

### DB マイグレーション

```bash
cd Backend
go run ./cmd/migrate         # AutoMigrate 実行
go run ./cmd/seed            # テストデータ投入
```

## アーキテクチャ

### 全体構成

```
frontend (Next.js :3000)
    ↓ HTTP
Backend (Go :8080)
    ├── MySQL 8.0 (GORM)
    ├── AWS S3 (面接動画・PDF)
    └── RAG service (Python FastAPI :9000)
            └── ChromaDB + CrewAI + OpenAI Embeddings
```

### バックエンド層構造 (DDD)

```
Backend/
├── domain/           # DDDエンティティ・リポジトリI/F (DB非依存)
│   ├── entity/       # ビジネスエンティティ
│   ├── repository/   # リポジトリインターフェース
│   ├── valueobject/  # スコア・マッチング等の値オブジェクト
│   └── mapper/       # Entity ↔ Model マッパー
└── internal/
    ├── models/       # GORMモデル (55+テーブル)
    ├── repositories/ # リポジトリ実装
    ├── services/     # ビジネスロジック (40+サービス)
    ├── controllers/  # HTTPハンドラ
    ├── routes/       # ルーティング定義
    ├── openai/       # OpenAI APIクライアント
    └── middleware/   # JWT認証ミドルウェア
```

依存関係の向き: Controller → Service → Repository → GORM Model。`main.go` で全コンポーネントを手動DI。

### AIフライホイール設計

チャット分析スコア (10カテゴリ × 4フェーズ) が各機能に連携し、精度が自己改善するサイクル:

- **チャット分析** → `AnswerEvaluator` でスコア評価 → `ChatScoreUpdaterService` でDB更新 → マッチング再計算
- **面接** (WebRTC + OpenAI Realtime API) → 面接スコア → チャットスコアに反映 (#204)
- **職務経歴書** (OCR + RAG) → レビュースコア → チャットスコアに反映 (#204)
- **応募・選考結果** → `ProfileRecalculationService` で企業プロファイル自動調整 (#202)
- **集合知** → `CollectiveInsightService` で類似ユーザーの通過企業を匿名集計・レコメンド (#205)

### フロントエンド

`frontend/app/` は Next.js App Router。主要ページ: `chat` (AIチャット)、`interview` (音声面接)、`resume` (職務経歴書)、`results` (マッチング結果)、`applications` (選考管理)、`admin` (管理者ダッシュボード)。

API通信は `frontend/lib/` 配下のモジュール経由。

## 環境変数

`.env.example` を参照。主要項目:

| 変数 | 用途 |
|------|------|
| `OPENAI_API_KEY` | チャット・面接・レポート生成 (必須) |
| `OPENAI_REALTIME_MODEL` | 音声面接用モデル (gpt-4-realtime-preview-20241217) |
| `RAG_REVIEW_URL` | RAGサービスエンドポイント (http://rag-review:9000) |
| `AWS_S3_BUCKET` | 面接動画・PDFの保存先 |
| `DB_HOST` / `DB_USER` / `DB_PASSWORD` | MySQL接続情報 |
| `BASE_URL` | OAuth コールバックURL |
| `GOOGLE_CLIENT_ID/SECRET` / `GITHUB_CLIENT_ID/SECRET` | OAuth2認証 |

フロントエンド: `NEXT_PUBLIC_BACKEND_URL=http://localhost:8080`

## CI/CD

- **PR時**: `go test ./internal/... -v -race` + `go build ./...` (`.github/workflows/test.yml`)
- **main push時**: ECRへDockerイメージをビルド&プッシュ → EC2にSSHデプロイ (`deploy-ec2.yml`)
- インフラは `/infra/terraform/` でAWS (EC2 + VPC + Route53) を管理

## 重要な注意点

- RAGサービスの依存関係は `constraints.txt` (固定バージョン) を使うこと。`requirements.txt` だとバージョン競合が起きる。
- Backendの `Dockerfile` はOCR・PDF処理のため libreoffice / chromium / fonts-noto-cjk を含む重量級イメージ。
- スコアは `user_weight_scores` テーブルに保存。10カテゴリ × 4フェーズで管理。
- 企業マッチングは `UserWeightScore × CompanyWeightProfile` の積で計算。
- 詳細な設計ドキュメントは `docs/wiki/` を参照 (API仕様・フライホイール設計・スコアキャリブレーション等)。
