# feat: #605 管理者向け AI/RAG 運用機能 - 初期作業ノート

目的:
- 管理者ダッシュボードに AI/RAG 運用セクションを追加し、RAG（ベクトルDB）を活用して LLM/WebSearch 呼び出しを削減する。

初期タスク:
1. ブランチ作成: feature/issue-605
2. フロントエンド: frontend/app/admin/ai-rag/page.tsx の雛形作成（ダッシュボードリンク、基本メトリクスのプレースホルダ）
3. バックエンド: Backend/internal/controllers/admin_ai_controller.go のルート・ハンドラ雛形を追加（/admin/ai-rag API）
4. RAG メトリクス取得用の軽量 API (Backend/internal/services/admin_ai_service.go) を追加
5. テスト: 最小のユニットテストを追加してエンドポイントの存在を検証
6. CI: 既存ワークフローで通ることを確認（push→PR→CI）

実装方針 (段階的導入):
- まずは画面と API の雛形を作成し、既存機能に影響を与えないことを確認する。
- 次フェーズで RAG ヒット率・cache hit メトリクスの収集と LLMスキップ判定ロジックを実装する。

備考:
- 変更は最小限かつ破壊的でないこと（既存 /admin/* を壊さない）
- 実装中に大きな設計変更が必要なら都度確認する

作成日: 2026-07-27
作成者: Copilot (実装開始のためのブランチ・ファイル作成)
