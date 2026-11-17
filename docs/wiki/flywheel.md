# AIフライホイール設計

## 概要

5つの機能が生成するデータが互いを強化し合い、ユーザー数の増加が直接的にレコメンド精度向上につながるサイクルです。

---

## フライホイール全体図

```
┌──────────────────────────────────────────────────────────┐
│                    AIフライホイール                        │
│                                                          │
│  ① チャット分析スコア（UserWeightScore）                   │
│          │                                               │
│          ├──────────────────────────────┐                │
│          ▼                              ▼                │
│  ② マッチング                   ⑤ 集合知レコメンド        │
│  （企業スコアとの照合）           （類似ユーザー通過企業）   │
│          │                              ▲                │
│          ▼                              │                │
│  ③ 選考結果フィードバック ────────────────┘                │
│  （応募→通過→内定データ蓄積）                              │
│          │                                               │
│          ▼                                               │
│  ④ 企業プロファイル動的更新                                │
│  （通過実績でWeightProfileを調整）                         │
│          │                                               │
│          └──→ ① のマッチング精度向上へ                     │
│                                                          │
│  ⑥ 機能間連携（#204）                                     │
│  面接スコア ──→ UserWeightScore 更新                       │
│  職務経歴書スコア ──→ UserWeightScore 更新                 │
│  UserWeightScore ──→ 面接AIシステムプロンプトへ注入        │
│  UserWeightScore ──→ 職務経歴書RAGコンテキストへ注入       │
└──────────────────────────────────────────────────────────┘
```

---

## 各機能の実装詳細

### #201 選考結果フィードバックループ

**データフロー:**
```
ユーザーが応募 → UserApplicationStatus 作成
選考通過 → UserApplicationStatus.Status 更新
         → UserCompanyMatch.IsApplied = true
         → 次回マッチング計算時に反映
```

**実装ファイル:**
- `models/company.go` — `UserApplicationStatus` モデル
- `services/application_service.go` — Apply / UpdateStatus / GetCorrelation
- `controllers/application_controller.go` — REST API
- `routes/application_routes.go` — `/api/applications`

---

### #202 企業プロファイル動的更新

**データフロー:**
```
選考通過ユーザーのスコア集計
→ PassedApplicantScores（カテゴリ別平均スコア）
→ CompanyWeightProfile を移動平均で更新（既存70% + 新データ30%）
→ 次回マッチングに反映
```

**更新条件:**
- 最低サンプル数: デフォルト3件
- 更新頻度: 管理画面から手動実行 or バッチ

**実装ファイル:**
- `services/profile_recalculation_service.go` — RecalculateAll / Rollback
- `controllers/admin_profile_recalculation_controller.go`
- API: `POST /api/admin/profile-recalculation/run`

---

### #203 スコア精度検証・改善基盤

**現状（自動ループ接続済み）:**
- 応募時に `user_application_statuses.score_snapshot` へ診断セッションのカテゴリ別スコアを JSON 保存する。
- 日次ジョブ `AutoCalibrateIfDue` が確定結果（通過/不採用）とスナップショットからカテゴリ軸重みを更新する。
- 重みは「通過側平均スコア − 不採用側平均スコア」を縮小・ステップ制限したもの（0.1〜3.0）。
- 有効な `ScoreCalibrationWeight` は次の `CalculateMatching` で軸重みとして適用される。
- 詳細・制約は[スコアキャリブレーション調査・手順](./score-calibration.md)。

**データフロー:**
```
応募 → score_snapshot 保存
    → 選考が通過/不採用で確定
    → 日次 AutoCalibrateIfDue（サンプル充足時）
    → ScoreCalibrationWeight 保存（is_active）
    → CalculateMatching が軸重みとして適用 → 精度がデータに追従
```

**実装ファイル:**
- `migrations/000075_application_score_snapshot.*`
- `models/score_validation.go` — QuestionVariant / VariantAssignment / ScoreCalibrationWeight
- `services/matching` — `SetCalibrationWeightReader` / `loadCalibrationAxisWeights`
- `services/application` — `SetScoreSnapshotReader` / `buildScoreSnapshot`
- `services/score_validation_service.go`
- `controllers/admin_score_validation_controller.go`

---

### #204 機能間データ連携（CrossFeatureIntegration）

**データフロー（双方向）:**

```
面接レポート保存後:
  communication(0-5) × 20 → コミュニケーション力スコア（移動平均）
  logic             × 20 → 技術志向スコア
  specificity       × 20 → 細部志向スコア
  ownership         × 20 → リーダーシップ・チャレンジ志向スコア
  enthusiasm        × 20 → 成長志向・チームワークスコア

職務経歴書レビュー保存後:
  score >= 70 → 細部志向・コミュニケーション力・技術志向 加点
  criticalCount >= 3 → 細部志向・コミュニケーション力 ペナルティ

面接開始時（StartTurn / Turn）:
  UserWeightScore（最新セッション）→ システムプロンプト先頭に注入
  「強み傾向: 技術志向(85点)、チームワーク(72点)」

職務経歴書レビュー開始時:
  UserWeightScore（最新セッション）→ RAG の candidate_context に注入
  （企業 brief の company_context とは分離。キャッシュ汚染を避ける）

職務経歴書レビュー保存後の再マッチ:
  診断セッションへスコア反映後、面接と同じく CalculateMatching を非同期実行
  （interview-{userId} スナップショットには再マッチしない）
```

**移動平均の計算式:**
```
blended = existing * 0.7 + newValue * 0.3
delta   = blended - existing
UpdateScore(delta)  // 加算式で更新
```

**実装ファイル:**
- `services/cross_feature_integration_service.go` — メインサービス
- `services/interview_service.go` — SetCrossFeatureService / StartTurn / Turn の修正
- `services/resume_service.go` — SetCrossFeatureService / ReviewDocument の修正

---

### #205 集合知レコメンド

**データフロー:**
```
ユーザー行動発生（同意済みのみ）:
  ApplicationService.Apply / UpdateStatus から自動 RecordAction
  （applied / document_passed|offered|accepted→passed / rejected）
  手動 API POST /api/collective-insights/actions も併用可（viewed 等）
  userID → SHA-256ハッシュ化（匿名化）
  スコアスナップショット（JSON）と共に CollectiveInsightLog に保存

レコメンド要求時:
  自分のスコアマップ → 過去ログのスコアとコサイン類似度計算
  類似度 >= 0.85 のユーザーハッシュを最大20件抽出
  → そのユーザーたちが通過/応募した企業を集計
  → collectiveScore = passCount / similarUserCount × 100

バッチ再集計（管理画面）:
  POST /api/admin/collective-insights/rebuild-summaries
  → 企業別 viewCount / applyCount / passCount / passRate を更新
```

**プライバシー:**
- userIDは `SHA-256("user:{id}:collective")` でハッシュ化
- `AllowCollectiveInsight = false` のユーザーの行動は記録しない
- 集計データから個人は特定不可能

---

## データベース関連図（主要テーブル）

```
users ──────────────────────────────────────────────────┐
  │                                                      │
  ├── user_weight_scores (sessionID × category × score)  │
  │         │                                            │
  │    (マッチング時に参照)                               │
  │         ▼                                            │
  ├── user_company_matches                               │
  │         │                                            │
  │    (応募時に参照)                                     │
  │         ▼                                            │
  └── user_application_statuses ── companies ────────────┘
                │                      │
         (選考通過集計)         company_weight_profiles
                │              (#202で自動更新)
                └─────────────────────────────────┐
                                                  │
  collective_insight_logs (匿名) ─────────────────┘
  anonymized_behavior_summaries
```
