# CLAUDE.md
SOC AI Agent: 採用支援SaaS (Go + Next.js + Python RAG)

## コマンド
### Backend (Go)
- `cd Backend && go run ./cmd/server` (8080) / `go test ./internal/... ./migrations/...` / `go run ./cmd/migrate`（up/down/version/force）
### Frontend (Next.js)
- `cd frontend && npm run dev` (3000) / `npm run build` / `npm run lint`
### RAG (Python)
- `cd rag && pip install -r constraints.txt && python3 main.py` (9000)
### Docker
- `docker compose up -d`（db/app/frontend/rag-review/chroma/company-graphを含む全サービスを起動。`docker compose stop` / `down` で全停止・削除可）
- RAGのみ起動/リビルド: `make rag-up` または `docker compose up -d --build chroma rag-review`
- スモーク: `make rag-smoke`（`/health` の `vector_store.ok` と Chroma heartbeat）
- 旧イメージ疑い: `make rag-rebuild`

## アーキテクチャ & フライホイール
- **構造**: FE(Next.js) -> BE(Go/MySQL/S3) -> RAG(Python/ChromaDB/LangChain)
- **DDD**: Controller -> Service -> Repository -> GORM Model
- **Flywheel**: チャット分析/面接/職務経歴書スコアがDBへ反映され、マッチングやプロファイルを自動調整

## 環境変数 (主要)
- `OPENAI_API_KEY`, `OPENAI_REALTIME_MODEL`, `RAG_REVIEW_URL`, `DB_HOST/USER/PASS`
- FE: `NEXT_PUBLIC_BACKEND_URL=http://localhost:8080`

## CI/CD
- PR時: Go/FEのLint & Test。Main push時: ECR/EC2へ自動デプロイ。

## コード規約
- **共通**: 日本語(コメント/コミット), UTF-8/LF, camelCase(Go/TS), snake_case(Py)
- **Go 1.25+**: `any`, `slices`, `for i := range n`, `errors.Is/As`, 手動DI, テーブル駆動テスト,TDD
- **FE**: 型安全(any禁止), Functional Components/Hooks, MUI, App Router
- **RAG**: Python 3.10+ 型ヒント, Pydantic, `logging`, プロンプト管理
- **AI/LLM**: プロンプト版管理, ストリーミング, JSON出力検証, APIキー秘匿

## 注意点
- **スキーマ変更は `Backend/migrations/` のup/down SQL必須**（GORM AutoMigrate禁止、`docs/wiki/migrations.md` 参照）。
- RAG依存関係は `constraints.txt` 必須。
- スコアは `user_weight_scores` (10カテゴリ×4フェーズ)。
- マッチングは `UserWeightScore × CompanyWeightProfile`。
- 詳細設計は `docs/wiki/` 参照。
