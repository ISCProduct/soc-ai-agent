# SOC AI Agent

求人・企業マッチングと会話型AIを組み合わせたフルスタックSaaSプロトタイプ。
チャット分析・音声面接・職務経歴書レビュー・選考管理のデータが互いに連携し、精度が自己改善する**AIフライホイール**構造を持ちます。

**開発メンバー**

| メンバー | 担当 |
|---------|------|
| 大橋 和幸 | 全体統括 |
| 原 拓哉 | フロントエンド |
| 田中 優希 | AI・LLM |
| 亀川 英地 | インフラ / バックエンド |
| 江坂 広樹 | インフラ / バックエンド |

---

## 目次

- [アーキテクチャ概要](#アーキテクチャ概要)
- [AIフライホイール](#aiフライホイール)
- [主な機能](#主な機能)
- [技術スタック](#技術スタック)
- [ディレクトリ構成](#ディレクトリ構成)
- [環境変数](#環境変数)
- [ローカル開発](#ローカル開発)
- [Docker Compose](#docker-compose)
- [GitHub CLI セットアップ](#github-cli-セットアップ)
- [Copilot カスタムコマンド共有](#copilot-カスタムコマンド共有)
- [主要APIエンドポイント](#主要apiエンドポイント)
- [品質管理](#品質管理)
- [よくあるトラブル](#よくあるトラブル)
- [Wiki・運用ドキュメント](#wikiwikiの場所)

---

## アーキテクチャ概要

```
                   ┌──────────────────────────────────┐
                   │         Next.js Frontend         │
                   │   App Router / MUI v7 / Three.js │
                   │   middleware.ts:                 │
                   │     認証Cookie→ヘッダー注入      │
                   │     テナント解決                 │
                   └────────────────┬─────────────────┘
                                    │ HTTP / WebRTC(OpenAI Realtime)
                   ┌────────────────▼─────────────────┐
                   │        Go Backend (Echo v4)      │
                   │   controllers → services →       │
                   │   repositories （domain = ポート）│
                   └──┬────────┬────────┬─────────┬───┘
                      │        │        │         │
            ┌─────────▼──┐ ┌───▼────┐ ┌─▼──────┐ ┌▼──────────────┐
            │ MySQL 8.0  │ │ Redis  │ │ AWS S3 │ │ FastAPI RAG   │
            │ GORM +     │ │ ジョブ │ │ 動画・ │ │ LangChain     │
            │ migrations │ │ キュー │ │ PDF    │ └───────┬───────┘
            └────────────┘ └────────┘ └────────┘         │
                                                 ┌───────▼───────┐
                                                 │   ChromaDB    │
                                                 │  ベクトルDB   │
                                                 └───────────────┘
```

### レイヤー構成（Backend）

`domain/` がリポジトリのインターフェース（ポート）と値オブジェクトを持ち、
`internal/repositories/` がその実装（アダプタ）にあたります。

```
routes/                      ルーティング・ミドルウェア適用
  └─ controllers/<domain>/   HTTPハンドラ（ドメイン別13パッケージ）
       └─ services/<domain>/ ビジネスロジック（ドメイン別30パッケージ）
            └─ repositories/ DBアクセス（domain/repository のI/Fを実装）
                 └─ models/  GORMモデル
```

スキーマ変更は `Backend/migrations/` の up/down SQL で管理します
（**GORM AutoMigrate は使用禁止**）。

---

## AIフライホイール

5つの機能が生成するデータが互いを強化し合う自己改善サイクルです。

```
  チャット分析スコア
       │
       ├──→ マッチング精度向上 ──→ 応募・選考データ蓄積 (#201)
       │                                    │
       │◄── 企業プロファイル動的更新 (#202) ◄──┘
       │
       ├──→ 面接AIコンテキスト注入 (#204)
       │         │
       │         └──→ 面接スコア → チャットスコア更新 (#204)
       │
       ├──→ 職務経歴書レビューコンテキスト注入 (#204)
       │         │
       │         └──→ レビュースコア → チャットスコア更新 (#204)
       │
       ├──→ スコア精度検証・キャリブレーション (#203)
       │
       └──→ 集合知レコメンド (類似ユーザーの選考通過パターン) (#205)
```

| Issue | 機能 | 概要 |
|-------|------|------|
| #201 | 選考結果フィードバックループ | 応募・選考ステータスをマッチングスコアに反映 |
| #202 | 企業プロファイル動的更新 | 通過実績ユーザーのスコアで企業重みを自動調整 |
| #203 | スコア精度検証基盤 | 通過率との相関分析・A/Bテスト・キャリブレーション |
| #204 | 機能間データ連携 | 面接/職務経歴書のスコアをチャット分析に双方向反映 |
| #205 | 集合知レコメンド | 類似スコアユーザーの通過企業を匿名集計してレコメンド |

---

## 主な機能

| 機能 | 概要 |
|------|------|
| AI チャット分析 | 4フェーズ・10カテゴリのスコアリングによる企業マッチング |
| 音声面接練習 | OpenAI Realtime API + 3D アバター（Three.js / wawa-lipsync） |
| 面接動画管理 | AWS S3 アップロード・管理者 Presigned URL 閲覧 |
| 職務経歴書レビュー | RAG（ChromaDB + OpenAI Embeddings）によるフィードバック生成 |
| 選考管理 | 応募→書類通過→面接→内定の選考ステータス管理 |
| 集合知レコメンド | 類似スコアユーザーの通過企業を匿名集計してレコメンド |
| スコア精度検証 | 通過率相関・A/Bテスト・自動キャリブレーション |
| 企業プロファイル自動更新 | 採用通過実績からCompanyWeightProfileを動的調整 |
| 企業関係図 | gBizINFO + React Flow 可視化 |
| 選考スケジュール管理 | 面接日程・締切の一元管理 |
| GitHub連携 | GitHubプロフィール・言語統計からスキルスコア算出 |
| 管理者ダッシュボード | ユーザー・企業・コスト・監査ログのCRUD |
| OAuth 認証 | Google / GitHub OAuth2 + メール・パスワード |
| メールレポート | 分析・面接レポートのメール配信 |
| 企業ポータル | 企業ユーザーによる学生検索・応募管理・自社プロフィール編集 |
| ES レビュー / リライト | エントリーシートの添削と書き換え提案 |
| 教員向け分析 | 担当校スコープでの生徒傾向分析（学校単位のアクセス制御） |
| マルチテナント | 学園サブドメインによるテナント解決 |

---

## 技術スタック

| 層 | 主な技術 |
|----|---------|
| Backend | Go 1.25 / Echo v4 / GORM (MySQL 8.0) / Redis / AWS SDK v2 / go-openai / Sentry |
| Frontend | Next.js 16 / React 19 / TypeScript / MUI v7 / Radix UI / React Flow / Three.js / Sentry |
| RAG | Python 3.10 / FastAPI / LangChain / ChromaDB / OpenAI Embeddings / Sentry |
| インフラ | AWS ECS（staging: on EC2 / 本番: on Fargate）+ ALB / RDS / S3 / Terraform |
| CI・テスト | GitHub Actions / Docker Compose / Jest / Playwright E2E / pytest |

> **補足**: CrewAI は依存衝突のため requirements.txt から除外済みです（Issue #273）。
> RAG の検索は LangChain + OpenAI web search 経由で行います。

## ディレクトリ構成

```
/
├── Backend/
│   ├── cmd/server/              # エントリポイント（手動DI）
│   ├── domain/                  # エンティティ・リポジトリI/F・VO・マッパー
│   ├── migrations/              # up/down SQL（AutoMigrate禁止）
│   └── internal/
│       ├── controllers/         # HTTPハンドラ（ドメイン別13パッケージ）
│       │   ├── admin/ auth/ chat/ company/ es/ github/ insight/
│       │   ├── interview/ application/ release/ resume/ schedule/ user/
│       │   ├── httpapi/         # 共通HTTPヘルパー（エラー応答・パラメータ取得）
│       │   ├── mocks/ testsupport/  # テスト用ダブル・共有ヘルパー
│       ├── services/            # ビジネスロジック（ドメイン別30パッケージ + shared/interfaces/prompts）
│       ├── repositories/        # DBアクセス（domain/repository の実装）
│       ├── models/              # GORMモデル
│       ├── routes/              # ルーティング・ミドルウェア適用
│       ├── middleware/          # 認証・スコープ制御
│       ├── observability/       # Sentry・メトリクス
│       └── queue/               # Redis ジョブキュー
├── frontend/
│   ├── app/                     # App Router ページ・Route Handler
│   ├── components/              # コンポーネント（PascalCase / ui は shadcn 規約）
│   ├── lib/                     # admin/ auth/ company/ interview/ + 共通ユーティリティ
│   ├── middleware.ts            # 認証Cookie→ヘッダー注入・テナント解決・旧URL転送
│   ├── tests/                   # Jest ユニットテスト
│   └── e2e/                     # Playwright E2E・デプロイ後スモーク
├── rag/
│   ├── main.py                  # FastAPI エントリポイント
│   ├── routers/ services/       # エンドポイント・処理本体
│   ├── training/                # LoRA学習・学習データ出力
│   └── tests/                   # pytest
├── docs/
│   ├── wiki/                    # 運用ドキュメント（正本）
│   ├── design/ requirements/    # 設計・要件
│   └── finetune/                # ファインチューニング関連
├── infra/terraform/             # staging / prod / modules
├── automation/                  # Discord通知・ワークフロー検査スクリプト
├── scripts/                     # 開発補助スクリプト
├── tools/company-graph/         # 企業スクレイピング（別Goモジュール）
├── compose.yml                  # ローカル開発用
└── docker-compose.yml           # staging EC2 用（ローカルでは使わない）
```

> **テストの置き場所**: Go のテストは対象パッケージの隣に置きます
> （`internal/controllers/admin/*_test.go` など）。`Backend/test/` に残っているのは
> 複数パッケージを横断する4ファイルのみです。

## 環境変数

### バックエンド（`.env`）

```env
# MySQL
DB_USER=app_user
DB_PASSWORD=app_pass
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=app_db

# サーバー
SERVER_PORT=8080
ADMIN_SECRET=change-me-admin-secret
USER_SECRET=change-me-user-secret

# OpenAI
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OPENAI_MODEL=gpt-4o-mini

# Realtime（音声面接）
OPENAI_REALTIME_MODEL=gpt-realtime
OPENAI_REALTIME_VOICE=alloy
OPENAI_REALTIME_TRANSCRIBE_MODEL=gpt-4o-mini-transcribe
OPENAI_REALTIME_MAX_OUTPUT_TOKENS=120
REALTIME_MAX_CONCURRENT_CONNECTIONS=30
REALTIME_MONTHLY_ALERT_THRESHOLD_USD=200
MATCHING_ALERT_EMAILS=ops@example.com,admin@example.com

# 面接レポート
INTERVIEW_REPORT_MODEL=gpt-4o-mini
INTERVIEW_TEMPLATE_VERSION=v1
INTERVIEW_MAX_MINUTES=10
INTERVIEW_MAX_COST_USD=1.8
INTERVIEW_COST_PER_MIN_USD=0.18

# CORS 許可オリジン（カンマ区切り）（#327: 未設定時は全オリジン拒否）
ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000

# GitHubアクセストークン暗号化キー（AES-256-GCM）（#326）
# 生成コマンド: python3 -c "import secrets; print(secrets.token_hex(32))"
# ⚠️ 本番環境では必ず環境ごとに異なる値を設定してください
TOKEN_ENCRYPTION_KEY=your-64-char-hex-key-here

# gBizINFO
GBIZINFO_BASE_URL=https://api.biz-info.go.jp
GBIZINFO_API_KEY=xxxxxxxxxxxxxxxx

# OAuth2
BASE_URL=http://localhost:8080
GOOGLE_CLIENT_ID=xxxxxxxxxxxxxxxx
GOOGLE_CLIENT_SECRET=xxxxxxxxxxxxxxxx
GITHUB_CLIENT_ID=xxxxxxxxxxxxxxxx
GITHUB_CLIENT_SECRET=xxxxxxxxxxxxxxxx

# AWS S3（面接動画・職務経歴書）
AWS_REGION=ap-northeast-1
AWS_S3_BUCKET=your-bucket
AWS_S3_PREFIX=interview-videos

# PDF アノテーション
# ANNOTATION_FONT_PATH=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc

# RAG レビューサービス
RAG_REVIEW_URL=http://rag-review:9000
```

### フロントエンド（`.env.local`）

```env
NEXT_PUBLIC_BACKEND_URL=http://localhost:8080
NEXT_PUBLIC_INTERVIEW_MAX_MINUTES=10
NEXT_PUBLIC_INTERVIEW_MAX_COST_USD=1.8
NEXT_PUBLIC_INTERVIEW_COST_PER_MIN_USD=0.18
```

---

## ローカル開発

### バックエンド

```sh
cd Backend
go mod download
go run ./cmd/server   # サーバー起動
```

### フロントエンド

```sh
cd frontend
npm install
npm run dev   # http://localhost:3000
```

### Docker Compose（推奨）

ローカル開発には `compose.yml` を使用します。

```sh
# コア（db / app / frontend）
docker compose up -d --build

# または
make core-up
```

**注意:** プロジェクト直下には `docker-compose.yml` も存在しますが、こちらは **本番環境（AWS ECR/RDS接続）用** です。ローカル開発で誤って使用すると本番DBに接続しようとするため、必ずデフォルトの `compose.yml`（ファイル名指定なしの `docker compose` コマンドで読み込まれます）を使用してください。

### RAG + Chroma

履歴書レビュー / 面接 hints / 企業コンテキストには **Chroma** と **rag-review** が必要です。
どちらも `compose.yml` の既定サービスなので、`docker compose up -d` で一緒に起動します。
RAG だけ作り直したいときは次を使います。

```sh
# ビルド込み + スモークまで一気に
make rag-up
# または
./scripts/dev-rag-up.sh
```

| サービス | 役割 | ポート |
|---------|------|--------|
| `chroma` | ベクトル DB（永続 volume `chroma_data`） | 8000 |
| `rag-review` | RAG API（`CHROMA_HOST=chroma`） | 9000 |

**スモーク（必ず確認）**

```sh
make rag-smoke
# または
curl -s http://localhost:8000/api/v2/heartbeat
curl -s http://localhost:9000/health
# → {"status":"ok","vector_store":{"ok":true,"detail":"chromadb http://chroma:8000"}}
curl -s http://localhost:9000/vector/status
```

`/health` に `vector_store` が無い・`/vector/status` が 404 のときは **旧 RAG イメージ**です。

```sh
make rag-rebuild
```

Backend からは compose 内で `RAG_REVIEW_URL=http://rag-review:9000`（`Backend/.env.example` 参照）。ホストから叩く場合は `http://localhost:9000`。

---

## Docker Compose

| ファイル名 | 用途 | 参照イメージ / 接続先 |
|------------|------|-----------------------|
| `compose.yml` | **ローカル開発用** | ローカルビルド / ローカルMySQL |
| `docker-compose.yml` | **本番環境用** | AWS ECR / AWS RDS |

### サービス一覧

```sh
docker compose up -d --build     # 既定サービスをまとめて起動
docker compose stop              # 停止
docker compose down              # 停止 + 削除（named volume は残る）
```

| サービス | 役割 | ポート | 備考 |
|---------|------|--------|------|
| `app` | Go バックエンド API | 8080 | |
| `db` | MySQL 8.0 | 3306 | `DB_HOST_PORT` で変更可（既定は 127.0.0.1 のみ公開） |
| `redis` | ジョブキュー・レート制限 | 6379 | |
| `frontend` | Next.js | 3000 | |
| `chroma` | ベクトル DB | 8000 | 永続 volume `chroma_data` |
| `rag-review` | 職務経歴書 / hints RAG | 9000 | Chroma 依存 |
| `company-graph` | 企業スクレイピング | 9100 | |
| `migrate` | DBマイグレーション適用 | - | **profile `tools`**。`docker compose --profile tools run --rm migrate` |

---

## GitHub CLI セットアップ

GitHub CLI（`gh`）を使うことで、Issue作成・PR作成・ブランチ操作などをターミナルから直接行えます。

### インストール

**macOS**

```sh
brew install gh
```

**Linux (apt)**

```sh
sudo apt install gh
```

**Windows (winget)**

```sh
winget install GitHub.cli
```

その他のインストール方法は [GitHub CLI 公式ドキュメント](https://cli.github.com/) を参照してください。

### 認証

```sh
gh auth login
```

対話形式で以下を選択します:

1. **GitHub.com** を選択
2. 認証プロトコルは **HTTPS** を推奨
3. 認証方法は **Login with a web browser** を選択（ブラウザが開き、コードを入力して認証）

### 認証確認

```sh
gh auth status
```

`Logged in to github.com as <username>` と表示されれば完了です。

### よく使うコマンド

```sh
# Issue 一覧
gh issue list

# PR 一覧
gh pr list

# 現在のブランチからPR作成
gh pr create

# PR のチェック状況確認
gh pr checks
```

---

## Copilot カスタムコマンド共有

チームで同じCopilotカスタムコマンドを使う場合は、リポジトリ内の以下を共通利用します。

- Prompt定義: `.github/prompts/*.prompt.md`
- 呼び出し関数: `scripts/copilot-shortcuts.sh`

### 初回セットアップ

方法1（推奨）: シェルに読み込んで関数として使う

```sh
source scripts/copilot-shortcuts.sh
```

毎回読み込む場合は、`~/.zshrc` などに追記します。

```sh
echo 'source /absolute/path/to/soc-ai-agent-mock/scripts/copilot-shortcuts.sh' >> ~/.zshrc
```

方法2: 直接実行する（`source` 不要）

```sh
./scripts/copilot-shortcuts.sh issue "管理者画面に監査ログ検索を追加したい"
./scripts/copilot-shortcuts.sh implement "123"
./scripts/copilot-shortcuts.sh pr "123"
```

### 利用例

```sh
cissue "管理者画面に監査ログ検索を追加したい"
cimpl "123"
cpr "123"
```

### うまく動かない場合

- `cissue: command not found` が出る場合: `source scripts/copilot-shortcuts.sh` が未実行です（またはシェル再起動後に未読込）。
- 直接実行する場合は `./scripts/copilot-shortcuts.sh ...` の形式で実行してください。

---

## 主要APIエンドポイント

### 認証
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/auth/register` | メール登録 |
| POST | `/api/auth/login` | ログイン |
| GET | `/api/auth/google` | Google OAuth |
| GET | `/api/auth/github` | GitHub OAuth |

### チャット分析
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/chat/messages` | メッセージ送信・スコア更新 |
| GET | `/api/chat/scores` | 分析スコア取得（10カテゴリ） |
| POST | `/api/chat/send-report` | メールレポート送信 |

### 面接
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/interviews` | セッション作成 |
| POST | `/api/interviews/{id}/start` | 開始（AIコンテキスト注入済み） |
| POST | `/api/interviews/{id}/upload-video` | 動画S3アップロード |
| POST | `/api/interviews/{id}/send-report` | レポートメール送信 |
| POST | `/api/realtime/token` | WebRTCトークン取得 |

### 職務経歴書
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/resume/upload` | アップロード |
| POST | `/api/resume/review` | RAGレビュー実行 |
| POST | `/api/resume/review/stream` | レビューのストリーミング取得 |
| GET | `/api/resume/status` | レビュー状態取得 |
| GET | `/api/resume/annotated` | 注釈付きPDF取得 |

### 選考管理（#201）
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/applications` | 応募登録 |
| GET | `/api/applications` | 選考一覧取得 |
| PUT | `/api/applications/{id}` | 応募内容更新 |
| POST | `/api/applications/{id}/withdraw` | 辞退 |
| POST | `/api/applications/{id}/accept` | 内定承諾 |

### 統合プロファイル（#204）
| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/user/profile` | チャット/面接/職務経歴書の統合プロファイル |

### 集合知レコメンド（#205）
| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/collective-insights/recommendations` | 類似ユーザー通過企業レコメンド |
| GET | `/api/collective-insights/top-companies` | 通過率上位企業 |
| PUT | `/api/collective-insights/consent` | 集合知参加同意設定 |
| POST | `/api/collective-insights/actions` | 行動ログ記録 |

### 管理者
| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/admin/dashboard/users` | ユーザー一覧 |
| GET | `/api/admin/interviews` | 面接セッション一覧 |
| POST | `/api/admin/profile-recalculation/run` | 企業プロファイル再計算（#202） |
| GET | `/api/admin/score-validation/correlation` | スコア通過率相関（#203） |
| POST | `/api/admin/score-validation/calibration/run` | キャリブレーション実行（#203） |
| POST | `/api/admin/score-validation/variants` | A/Bテストバリアント作成（#203） |
| POST | `/api/admin/collective-insights/rebuild-summaries` | 集合知サマリー再集計（#205） |
| GET | `/api/admin/costs/summary` | APIコストサマリー |
| GET | `/api/admin/audit-logs` | 監査ログ |

### 企業ポータル（企業ユーザー向け）
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/company-auth/login` | 企業ユーザーログイン |
| POST | `/api/company-auth/accept-invite` | 招待受諾 |
| GET | `/api/company-auth/students` | 学生検索 |
| POST | `/api/company-auth/students/semantic-search` | 学生のセマンティック検索 |
| GET | `/api/company-portal/applications` | 自社への応募一覧 |
| PATCH | `/api/company-portal/applications/{id}/status` | 選考ステータス更新 |

### ES（エントリーシート）
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/es/review` | ESレビュー |
| POST | `/api/es/rewrite` | ESリライト |

### スケジュール・カレンダー
| メソッド | パス | 概要 |
|---------|------|------|
| GET | `/api/schedule` | 選考スケジュール取得 |
| GET | `/api/google-calendar/...` | Googleカレンダー連携 |

> エンドポイントは全部で約200本あります。網羅した一覧は
> [`docs/wiki/api-reference.md`](./docs/wiki/api-reference.md) を参照してください。

---

## 品質管理

| 対象 | コマンド | CI |
|------|---------|----|
| Go | `cd Backend && go vet ./... && go test ./internal/... ./migrations/...` | Go Unit Tests |
| Frontend | `cd frontend && npm run lint && npx jest` | Frontend Unit Tests |
| E2E | `cd frontend && npx playwright test` | Frontend E2E Tests |
| RAG | `cd rag && python -m pytest tests` | test（`rag/**` の変更時のみ） |
| ワークフロー | `./automation/test/*.sh` | Workflow Scripts |

- **ブランチフロー**: `feature/* → develop → release → main`（各段でPRレビュー必須）
- **デプロイ**: develop push → staging へ自動デプロイ、main push → 本番へ自動デプロイ。`release` は中間ゲート（自動デプロイなし）
- **反映後スモーク**: staging デプロイ後に Playwright スモークが走ります（`frontend/e2e/smoke/`）

## よくあるトラブル

| 症状 | 対処 |
|------|------|
| DB接続エラー | `.env` の `DB_HOST` / 資格情報を確認。MySQL起動確認 |
| OpenAIキーエラー | `OPENAI_API_KEY` を設定 |
| OAuth動作不良 | `BASE_URL` / クライアントID / コールバックURLを確認 |
| GitHub同期エラー | `TOKEN_ENCRYPTION_KEY` が設定されているか確認。未設定だとトークン暗号化がスキップされ警告ログが出力される |
| CORS エラー（開発時） | `ALLOWED_ORIGINS=http://localhost:3000` を `.env` に設定（未設定時は全オリジン拒否） |
| S3アップロード失敗 | `AWS_S3_BUCKET` と IAM権限（`s3:PutObject` / `s3:GetObject`）を確認 |
| フロントビルド失敗 | Node.js 22 を使用（`frontend/.nvmrc`） |
| rag-review起動失敗 | `cd rag && pip install -r requirements.txt -c constraints.txt` で作り直す |

---

## Wiki・運用ドキュメント

詳細な運用ドキュメントは [`docs/wiki/`](./docs/wiki/) を参照してください。

| ドキュメント | 対象 | 内容 |
|------------|------|------|
| [Home](./docs/wiki/Home.md) | 全員 | 概要・ナビゲーション |
| [システム概要](./docs/wiki/overview.md) | 開発者・新規メンバー | アーキテクチャ・技術スタック・ディレクトリ構成 |
| [Getting Started](./docs/wiki/getting-started.md) | 新規メンバー | ローカル開発環境構築手順 |
| [AIフライホイール設計](./docs/wiki/flywheel.md) | 開発者 | データ連携の設計思想・フロー |
| [API リファレンス](./docs/wiki/api-reference.md) | 開発者 | 全エンドポイント詳細 |
| [RAG サービス詳細](./docs/wiki/rag-service.md) | 開発者 | ChromaDB・Web Search・Deep Research |
| [スコアリング・マッチング](./docs/wiki/scoring.md) | 開発者 | 10カテゴリスコア・マッチングアルゴリズム |
| [運用手順書](./docs/wiki/operations.md) | 運用担当 | デプロイ・監視・バッチ・障害対応 |
| [データプライバシー設計](./docs/wiki/data-privacy.md) | 開発者・法務 | 匿名化・同意管理の設計 |
| [スコアキャリブレーション](./docs/wiki/score-calibration.md) | 運用担当 | スコア精度検証・改善手順 |
