# AI面接: カスタム質問と深掘り

Issue: #559

## 概要

面接セッションごとに質問キューを DB 管理し、必須カスタム質問の出題と回答内容に基づく深掘りを Turn ベース面接で行う。

## データモデル

- `interview_question_states`: セッション内の出題計画（custom / topic / follow_up）
- `interview_sessions.company_id`, `position`, `company_name`: 面接コンテキスト

## フロー

1. **StartTurn / Turn（company_id あり）**
   - 初回: `BuildQuestionQueue` で必須カスタム → 標準トピック → 推奨カスタムを登録
   - ユーザー回答後: 最新 `asked` を `answered` に更新
   - `NeedsDeepening` が true なら `follow_up` を生成（最大 depth 2）
   - 深掘り不要なら次の `pending` を `asked` にしてプロンプトへ注入
2. **プロンプト**
   - `【今回の質問】` セクションで AI に1問だけ出題させる
3. **レポート**
   - `teacher_report_json` に `custom_questions_coverage`, `deepening_count`, `unasked_required_questions` をマージ

## Phase 2（未実装）

- Realtime 面接への同等機能
- Admin UI 深掘りヒント
- 面接履歴でのカバレッジ表示
