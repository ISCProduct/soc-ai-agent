# SOC AI Agent

求人・企業マッチングと会話型AIを組み合わせたフルスタックSaaSプロトタイプ。
チャット分析・音声面接・職務経歴書レビュー・選考管理のデータが互いに連携し、精度が自己改善する**AIフライホイール**構造を持ちます。

**開発メンバー**

- **バックエンド**: 大橋 和幸
- **フロントエンド**: 原 拓哉
- **AI・LLM**: 田中優希
- **インフラ**: 亀川英地
- **インフラ**: 江坂広樹
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
- [Copilot カスタムコマンド共有](#copilot-カスタムコマンド共有)
- [主要APIエンドポイント](#主要apiエンドポイント)
- [品質管理](#品質管理)
- [よくあるトラブル](#よくあるトラブル)
- [Wiki・運用ドキュメント](#wikiwikiの場所)

---

## アーキテクチャ概要

```
                      ┌─────────────────────────────┐
                      │        Next.js Frontend        │
                      │  (App Router / MUI v7 / 3D)   │
                      └───────────────┬───────────────┘
                                      │ HTTP / WebRTC
                      ┌───────────────▼───────────────┐
                      │       Go Backend (net/http)     │
                      │  DDD: entity / repo / svc / ctl │
                      └──┬──────────┬──────────┬───────┘
                         │          │          │
              ┌──────────▼──┐  ┌────▼────┐  ┌─▼──────────────┐
              │  MySQL 8.0   │  │ AWS S3  │  │ FastAPI RAG     │
              │  (GORM)      │  │ (動画・  │  │ (Python/CrewAI) │
              └─────────────┘  │  PDF)   │  └────────────────┘
                               └─────────┘
```

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
| 職務経歴書レビュー | RAG（DuckDuckGo + OpenAI Embeddings）によるフィードバック生成 |
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

---

## 技術スタック

- **Backend**: Go 1.25 / GORM (MySQL) / AWS SDK v2 / `go-openai`
- **Frontend**: Next.js 16 / React 19 / TypeScript / MUI v7 / Radix UI / React Flow / Three.js
- **RAG**: Python / FastAPI / CrewAI / OpenAI Embeddings
- **CI / 実行環境**: Docker / Docker Compose / GitHub Actions / Playwright E2E

---

## ディレクトリ構成

```
/
├── Backend/
│   ├── cmd/server/          # サーバーエントリポイント
│   ├── domain/              # エンティティ・リポジトリI/F・マッパー
│   ├── internal/
│   │   ├── controllers/     # HTTPハンドラ
│   │   ├── services/        # ビジネスロジック
│   │   ├── repositories/    # DBアクセス
│   │   ├── models/          # GORMモデル・AutoMigrate
│   │   ├── routes/          # ルーティング
│   │   └── middleware/      # 認証ミドルウェア
│   └── test/                # 統合テスト
├── frontend/
│   ├── app/                 # Next.js App Router ページ
│   ├── components/          # 共通コンポーネント
│   └── e2e/                 # Playwright E2E テスト
├── rag/                     # 職務経歴書RAGサービス (Python/FastAPI)
├── docs/wiki/               # 運用ドキュメント・Wiki
├── infra/                   # ECS インフラ設定
├── mysql/                   # MySQL ローカル設定
└── compose.yml              # Docker Compose 定義
```

---

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

# CORS 許可オリジン（カンマ区切り）
ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000

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
NEXT_PUBLIC_BACKEND_URL=http://localhost:80
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
docker compose up -d --build
```

**注意:** プロジェクト直下には `docker-compose.yml` も存在しますが、こちらは **本番環境（AWS ECR/RDS接続）用** です。ローカル開発で誤って使用すると本番DBに接続しようとするため、必ずデフォルトの `compose.yml`（ファイル名指定なしの `docker compose` コマンドで読み込まれます）を使用してください。

重いサービス（RAG）を後から起動する場合:

```sh
docker compose --profile rag up -d rag-review
```

---

## Docker Compose

| ファイル名 | 用途 | 参照イメージ / 接続先 |
|------------|------|-----------------------|
| `compose.yml` | **ローカル開発用** | ローカルビルド / ローカルMySQL |
| `docker-compose.yml` | **本番環境用** | AWS ECR / AWS RDS |

### サービス一覧

| サービス | 役割 | ポート |
|---------|------|--------|
| `app` | Go バックエンド API | 8080 |
| `db` | MySQL 8.0 | 3306 |
| `frontend` | Next.js | 3000 |
| `rag-review` | 職務経歴書 RAG | 9000 |
| `company-graph` | 企業スクレイピング | 9100 |

---

## Copilot カスタムコマンド共有

チームで同じCopilotカスタムコマンドを使う場合は、リポジトリ内の以下を共通利用します。

- Prompt定義: `.github/prompts/*.prompt.md`
- 呼び出しスクリプト: `scripts/copilot-shortcuts.sh`

### 初回セットアップ

方法1（推奨）: 関数として読み込む

```sh
source scripts/copilot-shortcuts.sh
```

方法2: 直接実行する（`source` 不要）

```sh
./scripts/copilot-shortcuts.sh issue "管理者画面に監査ログ検索を追加したい"
./scripts/copilot-shortcuts.sh implement "123"
./scripts/copilot-shortcuts.sh pr "123"
```

方法3: pull 後にすぐ使えるラッパーを実行する（`source` 不要）

```sh
./scripts/cissue "管理者画面に監査ログ検索を追加したい"
./scripts/cimpl "123"
./scripts/cpr "123"
```

### うまく動かない場合

- `cissue: command not found`: `source scripts/copilot-shortcuts.sh` が未実行です。
- `Permission denied`: `chmod +x scripts/copilot-shortcuts.sh scripts/cissue scripts/cimpl scripts/cpr` を実行してください。

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
| GET | `/api/resume/review` | RAGレビュー取得 |

### 選考管理（#201）
| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/api/applications` | 応募登録 |
| GET | `/api/applications` | 選考一覧取得 |
| PUT | `/api/applications/{id}` | ステータス更新 |

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

---

## /品質管理

- **Go**: `go vet` / `go test ./...` を CI で実行
- **Frontend**: `npm run lint`（ESLint）
- **E2E**: Playwright（`frontend/e2e/`）
- **PR ルール**: Lint + Go Unit Tests を通過させてからマージ

---

## よくあるトラブル

| 症状 | 対処 |
|------|------|
| DB接続エラー | `.env` の `DB_HOST` / 資格情報を確認。MySQL起動確認 |
| OpenAIキーエラー | `OPENAI_API_KEY` を設定 |
| OAuth動作不良 | `BASE_URL` / クライアントID / コールバックURLを確認 |
| S3アップロード失敗 | `AWS_S3_BUCKET` と IAM権限（`s3:PutObject` / `s3:GetObject`）を確認 |
| フロントビルド失敗 | Node.js 18以上を使用 |
| rag-review起動失敗 | `rag/constraints.txt` の固定バージョンで `pip install` |

---

## Wiki・運用ドキュメント

詳細な運用ドキュメントは [`docs/wiki/`](./docs/wiki/) を参照してください。

| ドキュメント | 内容 |
|------------|------|
| [Home](./docs/wiki/Home.md) | 概要・ナビゲーション |
| [AIフライホイール設計](./docs/wiki/flywheel.md) | データ連携の設計思想・フロー |
| [API リファレンス](./docs/wiki/api-reference.md) | 全エンドポイント詳細 |
| [運用手順書](./docs/wiki/operations.md) | デプロイ・監視・バッチ・障害対応 |
| [データプライバシー設計](./docs/wiki/data-privacy.md) | 匿名化・同意管理の設計 |
| [スコアキャリブレーション](./docs/wiki/score-calibration.md) | スコア精度検証・改善手順 |
