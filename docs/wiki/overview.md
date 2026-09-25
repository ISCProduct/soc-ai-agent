# システム概要

SOC AI Agent は、採用支援を目的としたフルスタック SaaS プロトタイプです。
チャット分析・音声面接・職務経歴書レビュー・選考管理が連携し、精度が自己改善する **AI フライホイール**構造を持ちます。

---

## アーキテクチャ全体図

```
                ┌────────────────────────────────────┐
                │          Next.js Frontend           │
                │  (App Router / MUI v7 / Three.js)  │
                └─────────────────┬──────────────────┘
                                  │ HTTP / WebSocket / WebRTC
                ┌─────────────────▼──────────────────┐
                │        Go Backend (Echo / 8080)      │
                │  DDD: entity → repo → service → ctl │
                └──┬─────────────┬──────────┬─────────┘
                   │             │          │
        ┌──────────▼──┐ ┌───▼───┐ ┌──▼──────┐ ┌──▼─────────────┐
        │  MySQL 8.0   │ │ Redis │ │ AWS S3  │ │  FastAPI RAG    │
        │  (GORM)      │ │ジョブ │ │(動画/PDF)│ │ (Python / 9000) │
        └─────────────┘ └───────┘ └─────────┘ └───┬────────────┘
                                                  │
                                            ┌─────▼──────┐
                                            │  ChromaDB  │
                                            │ ベクトルDB │
                                            └────────────┘
```

### 各サービスの責務

| サービス | 技術 | ポート | 責務 |
|---------|------|-------|------|
| Frontend | Next.js 16 / React 19 / TypeScript | 3000 | UI・UX 全般 |
| Backend | Go 1.25 / Echo / GORM | 8080 | ビジネスロジック・API |
| MySQL | MySQL 8.0 | 3306 | 永続データストア |
| RAG | Python / FastAPI / ChromaDB | 9000 | 職務経歴書レビュー・企業情報収集 |
| Redis | Redis 7 | 6379 | ジョブキュー・レート制限 |
| ChromaDB | Chroma | 8000 | ベクトルストア |
| AWS S3 | — | — | 面接動画・PDF 保管 |

---

## 主な機能一覧

| 機能 | 概要 | 関連 Issue |
|------|------|-----------|
| AI チャット分析 | 4フェーズ・10カテゴリのスコアリング | — |
| 音声面接練習 | OpenAI Realtime API + 3D アバター | — |
| 面接動画管理 | AWS S3 アップロード・Presigned URL | — |
| 職務経歴書レビュー | RAG (ChromaDB + OpenAI Embeddings) | — |
| 選考管理 | 応募〜内定の状態遷移管理 | #201 |
| 集合知レコメンド | 類似ユーザー通過企業の匿名集計 | #205 |
| スコア精度検証 | 通過率相関・A/B テスト・キャリブレーション | #203 |
| 企業プロファイル自動更新 | 採用実績から CompanyWeightProfile を動的調整 | #202 |
| 企業関係図 | gBizINFO + React Flow 可視化 | — |
| 選考スケジュール管理 | 面接日程・締切の一元管理 | — |
| GitHub 連携 | スキルスコア算出 | — |
| 管理者ダッシュボード | ユーザー・企業・コスト・監査ログ | — |
| OAuth 認証 | Google / GitHub / メール | — |

---

## データフロー（AI フライホイール）

5 つの機能が生成するデータが互いを強化し合う自己改善サイクルです。

```
チャット分析スコア（UserWeightScore）
       │
       ├──→ 企業マッチング（UserWeightScore × CompanyWeightProfile）
       │              │
       │              └──→ 応募・選考データ蓄積（UserApplicationStatus）
       │                              │
       │◄── 企業プロファイル動的更新 (#202) ◄──┘
       │
       ├──→ 面接 AI コンテキスト注入（#204）
       │              │
       │              └──→ 面接スコア → チャットスコア更新
       │
       ├──→ 職務経歴書レビュー コンテキスト注入（#204）
       │              │
       │              └──→ レビュースコア → チャットスコア更新
       │
       ├──→ スコア精度検証・キャリブレーション（#203）
       │
       └──→ 集合知レコメンド（類似ユーザーの選考通過パターン）（#205）
```

### フライホイール詳細

| Issue | 機能 | 処理内容 |
|-------|------|---------|
| #201 | 選考結果フィードバックループ | 選考ステータスをマッチングスコアに反映 |
| #202 | 企業プロファイル動的更新 | 通過実績ユーザーのスコアで企業重みを自動調整 |
| #203 | スコア精度検証基盤 | 通過率との相関分析・A/B テスト・キャリブレーション |
| #204 | 機能間データ連携 | 面接/職務経歴書スコアをチャット分析に双方向反映 |
| #205 | 集合知レコメンド | 類似スコアユーザーの通過企業を匿名集計してレコメンド |

---

## ディレクトリ構成

```
/
├── Backend/
│   ├── cmd/server/          # サーバーエントリポイント（手動DI）
│   ├── cmd/migrate/         # DBマイグレーション
│   ├── migrations/          # up/down SQL（AutoMigrate は廃止済み）
│   ├── domain/
│   │   ├── entity/          # ドメインエンティティ（Go struct）
│   │   ├── mapper/          # model ↔ entity 変換
│   │   ├── repository/      # リポジトリインターフェース（ポート）
│   │   └── valueobject/     # 値オブジェクト
│   ├── internal/
│   │   ├── controllers/     # HTTP ハンドラー（ドメイン別13パッケージ）
│   │   │   ├── admin/ auth/ chat/ company/ es/ github/ insight/
│   │   │   ├── interview/ application/ release/ resume/ schedule/ user/
│   │   │   ├── httpapi/     # 共通HTTPヘルパー（エラー応答・パラメータ取得）
│   │   │   └── mocks/ testsupport/   # テスト用ダブル・共有ヘルパー
│   │   ├── services/        # ビジネスロジック（ドメイン別30パッケージ + shared/interfaces/prompts）
│   │   │   ├── interfaces/  # サービスインターフェース
│   │   │   └── shared/      # サービス横断の共通処理
│   │   ├── repositories/    # DB アクセス（domain/repository の実装）
│   │   ├── models/          # GORM モデル
│   │   ├── routes/          # ルーティング定義
│   │   ├── middleware/      # 認証・CORS・スコープ制御
│   │   ├── observability/   # Sentry・メトリクス
│   │   └── queue/           # Redis ジョブキュー
│   └── test/                # 複数パッケージを横断するテストのみ（4ファイル）
├── frontend/
│   ├── app/                 # Next.js App Router ページ・Route Handler
│   ├── components/          # コンポーネント（PascalCase / ui は shadcn 規約）
│   ├── lib/                 # admin/ auth/ company/ interview/ + 共通ユーティリティ
│   ├── middleware.ts        # 認証Cookie→ヘッダー注入・テナント解決
│   ├── tests/               # Jest ユニットテスト
│   └── e2e/                 # Playwright E2E・デプロイ後スモーク
├── rag/                     # RAG サービス（Python / FastAPI）
│   ├── main.py              # FastAPI アプリケーション
│   ├── routers/ services/   # エンドポイント・処理本体
│   ├── training_api.py      # ファインチューニングデータ出力 API
│   ├── training/            # LoRA 学習・学習データ出力スクリプト
│   ├── constraints.txt      # 固定バージョン依存関係
│   └── tests/               # RAG テスト
├── docs/
│   ├── wiki/                # 運用ドキュメント（正本）
│   ├── design/ requirements/ # 設計・要件
│   └── finetune/            # ファインチューニング関連
├── infra/terraform/         # AWS インフラ設定（staging / prod / modules）
├── automation/              # Discord通知・ワークフロー検査スクリプト
├── tools/company-graph/     # 企業スクレイピング（別Goモジュール）
├── compose.yml              # Docker Compose（ローカル開発用）
└── docker-compose.yml       # Docker Compose（staging EC2 用）
```

> **テストの置き場所**: Go のテストは対象パッケージの隣に置きます
> （`internal/controllers/admin/*_test.go` など）。`Backend/test/` に残しているのは
> 複数パッケージを横断する4ファイルのみです。

---

## 技術スタック

| 区分 | 技術 | バージョン |
|------|------|----------|
| Backend | Go | 1.25 |
| HTTP フレームワーク | Echo | v4 |
| ORM | GORM + MySQL ドライバー | — |
| Frontend | Next.js / React / TypeScript | 16 / 19 / — |
| UI ライブラリ | MUI | v7 |
| 3D アバター | Three.js + wawa-lipsync | — |
| RAG | Python / FastAPI / LangChain / ChromaDB | 3.10+ |
| ベクトル DB | ChromaDB | — |
| AI | OpenAI API（GPT-4o / Realtime） | — |
| 動画・PDF | AWS S3 | — |
| CI/CD | GitHub Actions | — |
| コンテナ | Docker / Docker Compose | — |

---

## 関連ドキュメント

| ドキュメント | 内容 |
|------------|------|
| [Getting Started](./getting-started.md) | ローカル開発環境構築手順 |
| [API リファレンス](./api-reference.md) | 全エンドポイント詳細 |
| [RAG サービス詳細](./rag-service.md) | ChromaDB・Web Search・Deep Research |
| [スコアリング・マッチング](./scoring.md) | スコア定義・マッチングアルゴリズム |
| [AI フライホイール設計](./flywheel.md) | データ連携設計・フロー |
| [運用手順書](./operations.md) | デプロイ・監視・障害対応 |
