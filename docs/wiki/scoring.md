# スコアリング・マッチングロジック

SOC AI Agent のスコアリングとマッチングの設計・アルゴリズムを説明します。

---

## 概要

```
チャット分析（4フェーズ × 10カテゴリ）
       │
       ▼
UserWeightScore（ユーザースコア DB）
       │
       ▼
マッチング計算（UserWeightScore × CompanyWeightProfile）
       │
       ▼
UserCompanyMatch（総合マッチ度 0-100）
```

---

## 1. チャット分析フェーズ

チャット分析は 4 つのフェーズで構成されます。各フェーズの質問に回答することで、ユーザーの適性スコアが段階的に蓄積されます。

| フェーズ | フェーズ名（コード） | 内容 |
|---------|-------------------|------|
| 1 | `job_analysis` | 職務・技術志向の分析 |
| 2 | `interest_analysis` | 興味・関心の分析 |
| 3 | `aptitude_analysis` | 適性・特性の分析 |
| 4 | `future_analysis` | キャリアビジョンの分析 |

各フェーズに設定された質問に回答するたびに、対応するカテゴリのスコアが更新されます。

---

## 2. 10カテゴリスコア（UserWeightScore）

### カテゴリ定義

| # | カテゴリ（日本語） | コード | 説明 |
|---|----------------|------|------|
| 1 | 技術志向 | `技術志向` | 技術的な深さ・専門性への志向 |
| 2 | チームワーク志向 | `チームワーク志向` | チームでの協働・協調性への志向 |
| 3 | リーダーシップ志向 | `リーダーシップ志向` | 牽引・意思決定への志向 |
| 4 | 創造性志向 | `創造性志向` | 新しいアイデア・独自性への志向 |
| 5 | 安定志向 | `安定志向` | 継続性・安定した環境への志向 |
| 6 | 成長志向 | `成長志向` | 自己成長・学習機会への志向 |
| 7 | ワークライフバランス | `ワークライフバランス` | プライベートとの両立への志向 |
| 8 | チャレンジ志向 | `チャレンジ志向` | 新しい挑戦・リスクテイクへの志向 |
| 9 | 細部志向 | `細部志向` | 品質・正確さへの志向 |
| 10 | コミュニケーション力 | `コミュニケーション力` | 対人スキル・表現力への志向 |

### スコアの範囲

- 各カテゴリ: **0〜100**
- 未評価のカテゴリは中立値 **50** として扱われます

### データ構造（`UserWeightScore`）

```go
type UserWeightScore struct {
    ID             uint
    UserID         uint
    SessionID      string
    WeightCategory string  // 例: "技術志向"
    Score          int     // 0-100
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

---

## 3. 企業プロファイル（CompanyWeightProfile）

各企業は 10 カテゴリそれぞれに「重視度（0〜100）」を持ちます。

```go
type CompanyWeightProfile struct {
    TechnicalOrientation  int // 技術志向 (0-100)
    TeamworkOrientation   int // チームワーク志向 (0-100)
    LeadershipOrientation int // リーダーシップ志向 (0-100)
    CreativityOrientation int // 創造性志向 (0-100)
    StabilityOrientation  int // 安定志向 (0-100)
    GrowthOrientation     int // 成長志向 (0-100)
    WorkLifeBalance       int // ワークライフバランス (0-100)
    ChallengeSeeking      int // チャレンジ志向 (0-100)
    DetailOrientation     int // 細部志向 (0-100)
    CommunicationSkill    int // コミュニケーション力 (0-100)
    // ...
}
```

---

## 4. マッチングアルゴリズム

### カテゴリ別マッチ度の計算

```
マッチ度 = 100 - |ユーザースコア - 企業重視度|
```

**例:**
- ユーザー「技術志向」スコア: 80
- 企業「技術志向」重視度: 70
- マッチ度: `100 - |80 - 70|` = **90**

### 総合マッチスコアの計算

```
総合マッチスコア = 全カテゴリのマッチ度の平均
```

未評価カテゴリは中立値（50）として計算に含めます。

### 実装（`matching_service.go`）

```go
// scoredMatch 1カテゴリのマッチ度を計算
func scoredMatch(userScores map[string]float64, category string, companyWeight float64, ...) (...) {
    // 未評価カテゴリは中立値(50)として扱い、評価対象に含める
    userScore := userScores[category]  // 未評価の場合は 0 → 中立値補完
    diff := math.Abs(userScore - companyWeight)
    matchDegree := 100.0 - diff
    // ...
}
```

### 高マッチ判定

```
マッチ度 >= 80 → 高マッチ（IsHighMatch = true）
```

---

## 5. マッチング結果（UserCompanyMatch）

```go
type UserCompanyMatch struct {
    ID                 uint
    UserID             uint
    SessionID          string
    CompanyID          uint
    MatchScore         float64  // 総合マッチ度（0-100）
    MatchReason        string   // AI 生成のマッチ理由
    TechnicalMatch     float64  // カテゴリ別マッチ度
    TeamworkMatch      float64
    LeadershipMatch    float64
    CreativityMatch    float64
    StabilityMatch     float64
    GrowthMatch        float64
    WorkLifeMatch      float64
    ChallengeMatch     float64
    DetailMatch        float64
    CommunicationMatch float64
    IsApplied          bool     // 応募済みフラグ
    // ...
}
```

マッチング結果は `match_score` 降順でソートされ、ユーザーに提示されます。

---

## 6. フライホイール（スコア自動改善サイクル）

### 6.1 チャット分析スコア → マッチング精度向上

チャット分析が完了するたびに `CalculateMatching()` が実行され、すべての公開企業との `UserCompanyMatch` が更新されます。

### 6.2 選考結果 → 企業プロファイル動的更新（#202）

```
選考通過ユーザーのスコア蓄積
       │
       ▼
通過ユーザーの平均スコアで CompanyWeightProfile を自動調整
（POST /api/admin/profile-recalculation/run）
```

### 6.3 面接・職務経歴書スコア → UserWeightScore 更新（#204）

```
面接完了 → 面接スコア → UserWeightScore 更新 → 再マッチング
職務経歴書レビュー → レビュースコア → UserWeightScore 更新 → 再マッチング
```

---

## 7. スコア精度検証・キャリブレーション（#203）

実際の選考通過率とスコアの相関を分析し、スコアの重み係数を調整します。

### 相関分析

```
GET /api/admin/score-validation/correlation
```

10 カテゴリそれぞれについて、スコアバンド（0-20, 21-40, 41-60, 61-80, 81-100）ごとの選考通過率を集計します。

### A/Bテスト

```
POST /api/admin/score-validation/variants
```

異なるスコア計算ロジックを A/B テストして有効性を検証します。

### キャリブレーション実行

```
POST /api/admin/score-validation/calibration/run
```

通過率との相関が低いカテゴリの重み係数（`ScoreCalibrationWeight`）を自動調整します。

詳細は [スコアキャリブレーション手順書](./score-calibration.md) を参照してください。

---

## 8. 集合知レコメンド（#205）

```
ユーザーの UserWeightScore
       │
       ▼
類似スコアを持つ匿名ユーザーを検索
       │
       ▼
その匿名ユーザーが選考を通過した企業を集計
       │
       ▼
「あなたに近いプロフィールのユーザーが通過した企業」としてレコメンド
```

利用する際はユーザーの集合知参加同意が必要です。

```
PUT /api/collective-insights/consent
```

---

## 9. スコア計算フロー まとめ

```
1. ユーザーがチャット分析を完了
       │
2. UserWeightScore が 10 カテゴリ分 DB に保存
       │
3. CalculateMatching() 実行
   │── 全公開企業 × 10 カテゴリのマッチ度を計算
   │── 総合マッチスコア（平均）を算出
   └── AI がマッチ理由テキストを生成
       │
4. UserCompanyMatch（match_score 降順）を提示
       │
5. ユーザーが応募 → 選考結果がフィードバック
   │── 通過企業 → CompanyWeightProfile 自動更新（#202）
   │── 選考データ → 相関分析・キャリブレーション（#203）
   └── 類似ユーザーデータ → 集合知レコメンド更新（#205）
```

---

## 関連ドキュメント

- [AIフライホイール設計](./flywheel.md) — フライホイール全体設計
- [スコアキャリブレーション](./score-calibration.md) — キャリブレーション実施手順
- [システム概要](./overview.md) — アーキテクチャ全体
