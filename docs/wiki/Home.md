# SOC AI Agent — Wiki ホーム

求人・企業マッチングSaaSの開発・運用に関するドキュメントをまとめています。

---

## ドキュメント一覧

| ドキュメント | 対象 | 内容 |
|------------|------|------|
| [AIフライホイール設計](./flywheel.md) | 開発者 | 5機能のデータ連携設計・フロー |
| [API リファレンス](./api-reference.md) | 開発者 | 全エンドポイント詳細・リクエスト/レスポンス例 |
| [運用手順書](./operations.md) | 運用担当 | デプロイ・監視・バッチ・障害対応手順 |
| [データプライバシー設計](./data-privacy.md) | 開発者・法務 | 匿名化・同意管理の設計方針 |
| [スコアキャリブレーション](./score-calibration.md) | 運用担当 | スコア精度検証・改善の実施手順 |
| [選考ステータス状態遷移 要件定義](../requirements/application-status-transition.md) | 上流工程担当 | 選考管理の状態遷移・権限・画面/API要件 |

---

## システム概要

```
ユーザー
  │
  ├─ チャット分析（4フェーズ × 10カテゴリ）
  │     └─ UserWeightScore（スコアDB）
  │              │
  │    ┌─────────┼──────────┐
  │    │         │          │
  │  マッチング  面接AI    職務経歴書AI
  │    │         │          │
  │    └── 選考結果 → DB ──→ 企業プロファイル自動更新
  │              │
  │         集合知ログ（匿名）
  │              │
  └─────── 類似ユーザー通過企業レコメンド
```

---

## 用語集

| 用語 | 説明 |
|------|------|
| UserWeightScore | 10カテゴリ（技術志向・チームワーク等）のユーザースコア（0-100） |
| CompanyWeightProfile | 企業が重視する10カテゴリの重み設定 |
| UserCompanyMatch | ユーザーと企業のマッチングスコア（0-100） |
| UserApplicationStatus | 応募・選考ステータス（applied/document_passed/interview/offered/accepted/rejected） |
| CollectiveInsightLog | 匿名化されたユーザー行動ログ（集合知レコメンド用） |
| ScoreCalibrationWeight | 選考通過率に基づくカテゴリ別スコア重み係数 |
| QuestionVariant | A/Bテスト用の質問セットバリアント |
