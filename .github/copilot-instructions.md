# SOC AI Agent - Repository-wide Copilot instructions

このリポジトリでは、以下を優先して提案・実装してください。

## 基本方針
- 既存の構成と規約を尊重し、変更は最小限かつ根本原因に対処する。
- コメント・コミットメッセージは日本語を基本とする。
- 文字コードは UTF-8 / 改行は LF。

## アーキテクチャ理解
- FE(Next.js) -> BE(Go) -> RAG(Python) の構成。
- Backend は DDD 構成（Controller -> Service -> Repository -> GORM Model）を維持する。

## 実装時の規約
- Go/TypeScript は camelCase、Python は snake_case。
- Frontend は型安全を優先し、`any` を避ける。
- RAG 実装は型ヒント、Pydantic、`logging` を使う。
- APIキー等の秘密情報はコードやログに出力しない。

## 主要コマンド
- Backend: `cd Backend && go run ./cmd/server`
- Frontend: `cd frontend && npm run dev`
- RAG: `cd rag && pip install -r constraints.txt && python3 main.py`
- Docker: `docker compose up -d`（必要に応じて `--profile rag`）

## レビュー観点
- 仕様との整合性、型安全性、エラーハンドリング、既存テストへの影響を重視する。
- 関連ドキュメント（README / docs/wiki）を必要に応じて更新する。
