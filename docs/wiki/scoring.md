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
- チャット進捗の「評価済み」判定は **score ≠ 0** の件数
- マッチングでは未計測カテゴリを中立50で埋めず、平均から除外する（#1124）

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

## 2-1. カテゴリの正典と別名（#929）

カテゴリ名の正典は `Backend/domain/valueobject/match.go` の10種類。
マッチングは `scoreMap` をこのキーで引き、**見つからないカテゴリは平均から除外する**
（`matching_service.go` の `scoredMatch`、#1124）。名前が揺れるとユーザーの実スコアが
捨てられて軸数が減るため、保存経路では正典化が必須。

保存経路（`UserWeightScoreRepository.SetScore/AddScore`、`POST /api/questions/*`）は
`valueobject.ParseWeightCategory` を必ず通し、正典外を弾く。

### 別名表の2種類

`weightCategoryAliases` には性質の違う2種類が同居している。追加するときは
どちらなのかを意識すること。

**(a) 表記揺れの吸収** — 同じ概念の別表記。意味は変わらない。

| 別名 | 正典 |
|---|---|
| チームワーク | チームワーク志向 |
| リーダーシップ | リーダーシップ志向 |
| 創造性 | 創造性志向 |
| コミュニケーション能力 | コミュニケーション力 |

**(b) 意味的な統合** — 正典に対応軸が無いものを最も近い軸へ寄せている。
**スコアの意味が元の質問の意図とは変わる。**

| 旧カテゴリ | 統合先 | 妥当性 |
|---|---|---|
| 問題解決力 / 分析思考 | 技術志向 | 妥当（`analysis_scoring_calculate.go` の論理性軸と整合） |
| 計画性・実行力 | 細部志向 | 妥当 |
| ストレス耐性・粘り強さ | チャレンジ志向 | 妥当 |
| 学習意欲・成長志向 | 成長志向 | 妥当 |
| **ビジネス思考・目標志向** | **チャレンジ志向** | 成果・目標達成に寄せる（成長＝学習意欲と分離） |

### ビジネス思考・目標志向の寄せ先

正典10軸に事業志向の専用軸は無い。成長志向へ寄せると学習意欲質問と同軸になり
ユーザーの Growth だけが膨らむため、**チャレンジ志向**へ統合する（migration 028）。

seed 質問の振り分け:

| 質問の意図 | 正典カテゴリ |
|---|---|
| 成果を重視するか | チャレンジ志向 |
| 顧客・利用者視点 | コミュニケーション力 |
| 社会・組織への価値提供 | リーダーシップ志向 |

同じ理由で `技術志向` も 問題解決力 + 分析思考 の受け皿になっている。

### 正典を分離すべき条件

次のいずれかに当たったら、統合をやめて正典に軸を追加することを検討する。

1. 事業志向・目標達成を企業側が明示的に重視したいという要求が出たとき
2. `score_validation` でチャレンジ志向・技術志向が企業側と系統的にずれていると確認できたとき
3. 統合先の軸のスコア分布が、統合前と比べて明らかに歪んでいるとき

**分離する場合の影響範囲**（1マイグレーションでは済まない）:

- `company_weight_profiles` / `user_company_matches` のカラム追加
- 企業入力UI（`frontend/app/company-entry/page-content.tsx`）の項目追加
- 既存企業データの初期値埋め
- フロントの型・APIレスポンス
- `matching_service.calculateMatchScore` の軸追加

---

## 2-2. 業界プロファイル（IndustryWeightProfile / #1027）

教員向けの「向いている業界」は、生徒の10カテゴリスコアと業界ごとの重視度を
`matching.CalculateCategoryMatch`（企業マッチングと同じ式）で突き合わせて出す。

- テーブル: `industry_weight_profiles`（`Backend/migrations/000021_*`）
- 投入: `Backend/internal/models/seed.go` の `seedIndustryWeightProfiles`（冪等）
- 行が無い業界は**中立50**として扱い、エラーにしない

### 現在の値は暫定である

初期値の出典は業務知見ではなく、**業界イメージからの仮置き**である。
PRD でも「初期データ投入作業自体はスコープ外」としている。

画面の「向いている業界」を直接決める値なので、運用に乗せる前に
採用実績や求人票の分析に基づく値へ差し替えること。

### 更新方法

`seedIndustryWeightProfiles` は**既に行がある業界は上書きしない**。
運用中に管理者が調整した値をシードが壊さないためである。

したがって値を変えるには次のどちらかを行う。

1. DB を直接更新する（運用中の調整）
2. マイグレーションで `UPDATE` する（全環境へ配りたい変更）

シードの定数を書き換えるだけでは、**既にデータがある環境には反映されない**。

### 見直しの条件

- 生徒の進路実績と「向いている業界」の乖離が現場から報告されたとき
- 業界マスタ（`industries`）に業界を追加したとき（未設定は中立50になり、
  他業界と横並びで区別できなくなる）
- `matching.CalculateCategoryMatch` の式を変えたとき（企業側と同じ式を使うため）

### 既知の制約

- `industries.parent_id` はシードが設定しておらず全件 NULL。
  親子関係は `level` でしか判別できない。一覧は大分類（level 0）のみを対象にしている
- スコアはシグモイド由来で 95〜99 に張り付きやすく、業界間の差が小さい。
  UI では順位として扱い、数値を適合率のように見せないこと

---

## 2-3. LLM が使えないときはスコアを書かない（#831）

面接には LLM 非依存のローカル評価器（`evaluator_local.go`）があったが、
**本番コードから一度も呼ばれておらず削除した**（#831）。

同種のフォールバックを再び足す前に、次を確認すること。

### なぜ「粗いスコアを書く」より「書かない」方が安全か

`user_weight_scores` はマッチング（`UserWeightScore × CompanyWeightProfile`）と
教員向けの傾向分析（#1027）の両方が読む。**誤ったスコアを書くと、
どちらにも静かに混ざり、後から区別できない。**

一方スコアが無い場合は、マッチングがその軸を平均から除外し（`matching_service.go` の
`scoredMatch`）、傾向分析は「分析データ不足」と表示する（#1027）。
**欠損は画面に出るが、誤りは出ない。**

### 削除したヒューリスティックの実際の壊れ方

キーワード一致で +10 する実装だったが、実測で次のようになっていた。

| 入力 | 出力 | 問題 |
|---|---|---|
| 今回のプロジェクトについてお話しします。 | 細部志向 +10 | 「回」がキーワードで「今回」に誤爆 |
| チームワークは苦手で、一人で作業する方が得意です。 | チームワーク志向 +10 | 否定形を見ない |
| 具体的には、例えば詳細な設計を丁寧に正確に…実績として月10回… | 細部志向 +70 | 加点が青天井。長く話すほど上がる |

キーワードを直しても、**否定・文脈・話者の意図を見ない限り同じ問題が残る**。
ルーブリックに寄せるなら LLM が要る。それが使えない状況のための
フォールバックなので、この方向は成立しない。

## 3. 企業プロファイル（CompanyWeightProfile）

各企業は 10 カテゴリそれぞれに「重視度（0〜100）」を持ちます。
プロファイルが無い企業はマッチングの対象外（[#1380](#プロファイルを持たない企業1380)）。

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
総合マッチスコア = 計測できたカテゴリのマッチ度の平均
```

未計測カテゴリは平均に**含めない**（#1124）。件数は `matched_axis_count` に保存する。
以前は中立値50で埋めていたが、企業重視度が50前後に寄ると未診断でも97%前後に飽和するため廃止した。

### プロファイルを持たない企業（#1380）

**`CompanyWeightProfile` が無い公開企業はマッチング対象から外す**（`CalculateMatching` が `continue` する）。

以前は全軸50のデフォルトで埋めていたが、全軸50は取りうる中で最も平坦なプロファイルで、
マッチ度が `100 - |差|` である以上、スコアが50付近の平均的な学生に対して各軸100点になる。
**情報が無い企業ほど上位に出る**という逆のインセンティブになり、
学生にも「なぜこの企業が1位なのか」を説明できなかった。

- 学生から見える影響: プロファイルが無い企業は推薦に出ない（`user_company_matches` に行を作らない）
- **既存行も読み出し時に除外する**: `CalculateMatching` は行を作らないだけで、過去に作られた
  `user_company_matches` は残る（`CreateOrUpdateBatch` は upsert で削除しない）。
  そのため読み出し側でも外す — `FindTopMatchesByUserAndSession`（推薦一覧・メールレポート）と
  `FindLowMatchApplicationsByUsers`（教員の低マッチ集計）に
  `CompanyHasWeightProfileSQL` の EXISTS 条件を入れている。
  **行は消さない**。`is_viewed` / `is_favorited` / `is_applied` はユーザー操作の結果で、
  プロファイルを生成し直せば元のスコアごと復帰する。削除すると復帰できない
- 運用者から見える影響:
  - 一部欠損の常時監視は `GET /api/admin/companies/l1-coverage` の
    `companies_without_profile`（= `published_total - has_profile`。追加クエリ無し）。
    `profile_rate` / `profile_target=0.95` のアラートも同じ数字から出る
  - 推薦が0件になったときは `GET /api/chat/recommendations` の
    `diagnostics.companies_without_profile`（`CountPublishedWithoutWeightProfile`）
  - 公開企業がすべてプロファイル未設定なら `reason = insufficient_company_profiles` を返す。
    公開企業が0社の `insufficient_company_data` とは分ける
    （前者は「プロファイル生成が必要」、後者は「企業公開が必要」で復旧手順が違う）
- プロファイルは `FetchAndSavePersona`（AI）で生成する。失敗した企業は推薦に出ないまま残るため、
  上の件数が増え続けていないかを見る
- 識別力が低いだけ（全軸ほぼ同値）のプロファイルは**除外しない**。保存はして記録に残す（#1331）

### 実装（`matching_service.go`）

```go
// scoredMatch 1カテゴリのマッチ度を計算
func scoredMatch(userScores map[string]float64, category string, companyWeight float64, ...) (...) {
    userScore, ok := userScores[category]
    if !ok {
        return 0, evaluatedCount, totalScore // 未計測は平均から除外
    }
    matchDegree := 100.0 - math.Abs(userScore - companyWeight) // 線形
    // ...
}
```

### 高マッチ判定

```
マッチ度 >= 80 → 高マッチ（IsHighMatch = true）
```

### 選択肢回答の根拠品質

選択肢記号の**生の位置**は `A=100 … E=20`。ただしそのまま書くと理由なしの極端値が量産されるため、書き込み時に調整する（`choice_evidence.go`）。

| 状況 | 扱い |
|------|------|
| 理由なし（または極短） | 中立50へ減衰（距離×0.55）。フラグ `choice_only_evidence` |
| 支持する理由あり | 生の位置を採用 |
| 理由が選択と矛盾 | 強く減衰（距離×0.25）。フラグ `choice_reason_contradiction` |

文章の「品質スコア」と軸位置はスケールが違うため**混ぜない**（混ぜると理由を書いた人ほど中央に寄る）。

### 診断妥当性と暫定表示

診断完了後の品質ジョブはスコアを**自動補正せず**、`diagnosis_quality_reports` に confidence / flags を保存する。
`GET /api/chat/recommendations` は次のいずれかで `is_provisional=true` とする。

- 評価カテゴリ数が 4 未満
- 上位マッチの最小 `matched_axis_count` が 4 未満
- 診断信頼度が 55 未満、または薄い根拠フラグ（`thin_chat_evidence` / `mostly_choice_only` 等）

レスポンスに `diagnosis_confidence` / `diagnosis_flags` / `diagnosis_summary` / `min_matched_axis_count` を載せる。

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
面接完了
  → 最新のチャット診断 session_id を解決（無ければ interview-{userId}）
  → 面接スコアを移動平均で UserWeightScore に反映
  → チャット session なら CalculateMatching で再マッチング
職務経歴書レビュー → レビュースコア → UserWeightScore 更新
（職務経歴書側の自動再マッチは未接続）
```

チャット診断と面接スナップショットを混ぜない。`FindLatestByUser` もチャット session を優先する。

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

## 2-4. 面接ルーブリックのスキーマ違反はレポートごと捨てる（#795）

面接レポートの評価項目（`logic` / `specificity` / `ownership` /
`communication` / `enthusiasm`、各0〜5）は
`Backend/internal/services/interview/interview_rubric.go` に**1箇所だけ**定義する。
プロンプトの評価基準セクションも、検証も、同じ定義から読む。

### なぜ1箇所にまとめるか

以前はプロンプト文字列に評価基準がハードコードされ、検証は存在しなかった。
項目を増減してもどこも壊れないため、
**「LLM は出しているのに誰も読まない項目」が静かに生まれる。**

### 何を弾くか

`ValidateRubricScores` が次を弾く。

- スコアが空
- 必須項目の欠落
- 未知の項目
- 0〜5 の範囲外（LLM が100点満点で返す事故が実際に起こりうる）

### 弾いた後どうするか

**1度だけ作り直す。それも違反なら、失敗の種類で分ける。**

| 失敗 | 保存されるもの |
| --- | --- |
| JSON として読めない | 何も保存しない（残せるものが無い） |
| 読めるがスコアだけ不正 | **講評は保存し、スコアだけ捨てる** |

レポートごと捨てると、面接を終えた学生に何も表示されない。
summary / strengths / improvements はスコアと独立して有用なので届ける。

スコアを捨てるときは `scores_json` / `evidence_json` を**空文字**にする。
`"null"` や `"{}"` を入れると `UpdateScoresFromInterviewReport` の
`ScoresJSON == ""` 早期リターンに乗らずスコア反映へ進み、
画面側も空オブジェクトを truthy と見て平均が `NaN` になる。

### 「発話が無い」ときもスコアは書かない

発話0件の空レポートは以前 `{"logic":0,...}` と全項目0点を書いていたが、
**「発話が無い」ことと「全項目が最低評価」は違う。**
0点は画面に最低評価として表示され、学生を誤解させる。空文字にする。

面接スコアは `UpdateScoresFromInterviewReport` が `×20` して
`user_weight_scores` へ流す。リポジトリ層が0〜100に丸めるためDBは壊れないが、
**丸めた結果は「最高評価」として保存され、マッチングと教員向け傾向分析の
両方に静かに混ざる。** §2-3 と同じ理由で、粗い値を書くより書かない方が安全。

ただし**捨てるのはスコアだけ**である。講評まで捨てると、
誤りを防ぐ代わりに学生への価値をゼロにしてしまう。


---

## 関連ドキュメント

- [AIフライホイール設計](./flywheel.md) — フライホイール全体設計
- [スコアキャリブレーション](./score-calibration.md) — キャリブレーション実施手順
- [システム概要](./overview.md) — アーキテクチャ全体
