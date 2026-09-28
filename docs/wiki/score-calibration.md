# スコアキャリブレーション手順

## 概要

チャット分析の10カテゴリスコアが実際の選考通過率とどう相関するかを検証し、スコアの重み係数を実績データに基づいて調整する手順です。

---

## 全体フロー

```
1. 相関レポート確認
      ↓
2. A/Bテスト実施（必要に応じて）
      ↓
3. キャリブレーション実行
      ↓
4. 重みの確認・適用
      ↓
5. 効果測定（次回ループへ）
```

---

## Step 1: 相関レポート確認

スコアと選考通過率の相関を確認します。

```sh
GET /api/admin/score-validation/correlation
```

**レスポンス例:**
```json
{
  "rows": [
    {
      "category": "技術志向",
      "score_band": "61-80",
      "total_count": 45,
      "pass_count": 32,
      "pass_rate": 71.1,
      "avg_score": 72.3
    }
  ],
  "top_correlated": ["技術志向", "コミュニケーション力"],
  "low_correlated": ["安定志向", "ワークライフバランス"],
  "total_samples": 230
}
```

**判断基準:**
- `top_correlated`: 通過率との相関が高い → 現在の質問設計が有効
- `low_correlated`: 相関が低い → 質問内容の見直し・A/Bテストを検討

---

## Step 2: フェーズ精度メトリクス確認

```sh
GET /api/admin/score-validation/phase-metrics
```

**レスポンス例:**
```json
{
  "phases": [
    {
      "phase_name": "職種分析",
      "session_count": 180,
      "avg_completion": 85.2,
      "pass_count": 42,
      "pass_rate": 23.3
    }
  ],
  "overall_pass_rate": 21.5
}
```

完了率が低いフェーズは質問数・難易度の見直しが有効です。

---

## Step 3: A/Bテスト（任意）

相関が低いカテゴリの質問を改善する前に、A/Bテストで効果を検証します。

### バリアント作成

```sh
POST /api/admin/score-validation/variants
{
  "experiment_name": "tech_orientation_q4_2024",
  "variant_name": "control",
  "description": "現在の技術志向質問セット",
  "traffic_ratio": 0.5
}

POST /api/admin/score-validation/variants
{
  "experiment_name": "tech_orientation_q4_2024",
  "variant_name": "treatment_a",
  "description": "改善版技術志向質問セット",
  "traffic_ratio": 0.5
}
```

### バリアント割り当て（フロントエンド連携）

セッション開始時に:
```sh
POST /api/collective-insights/actions （または専用バリアントAPI）
```

### 結果確認（2〜4週間後）

```sh
GET /api/admin/score-validation/variants/results?experiment=tech_orientation_q4_2024
```

**結果例:**
```json
{
  "experiment": "tech_orientation_q4_2024",
  "results": [
    {"variant_name": "control",     "session_count": 90, "pass_count": 18, "pass_rate": 20.0},
    {"variant_name": "treatment_a", "session_count": 88, "pass_count": 24, "pass_rate": 27.3}
  ]
}
```

通過率が改善している場合は `treatment_a` を本番採用します。

---

## Step 4: キャリブレーション実行

**前提条件:** 各カテゴリに5件以上の選考結果データが必要

```sh
POST /api/admin/score-validation/calibration/run
```

**レスポンス例:**
```json
{
  "version": 3,
  "weights": [
    {"category": "技術志向",         "weight": 1.28, "pass_rate": 27.5, "correlation": 0.28, "sample_count": 48},
    {"category": "コミュニケーション力", "weight": 1.15, "pass_rate": 24.8, "correlation": 0.15, "sample_count": 52},
    {"category": "安定志向",          "weight": 0.72, "pass_rate": 15.5, "correlation": -0.28, "sample_count": 38}
  ],
  "message": "キャリブレーション完了: 10 カテゴリ、サンプル合計 480 件"
}
```

**重みの意味:**
- `weight > 1.0`: このカテゴリのスコアが高いと通過しやすい（重要度高）
- `weight < 1.0`: このカテゴリとの相関が弱い（重要度低）
- `weight ≈ 1.0`: 平均的な相関

---

## Step 5: キャリブレーション履歴確認

```sh
GET /api/admin/score-validation/calibration/history?limit=5
```

バージョン間の重み変化を追跡し、スコア設計の改善効果を定量評価します。

---

## 現在の重み確認

```sh
GET /api/admin/score-validation/calibration
```

`is_active: true` の重みが現在適用中です。

---

## 変換式を変えるときのドライラン（#1528）

キャリブレーション（重み係数の調整）とは別に、
**履歴書・面接から `user_weight_scores` への変換式そのもの**を変えることがあります。
式を変えると既存ユーザーのスコア分布が動くため、適用前に新旧を並べて確認します。

```sh
cd Backend && go test ./internal/services/flywheel/ -run DryRun -v
```

DB へは一切書き込みません。入力（履歴書の総合スコアと critical 件数、
面接のルーブリックと発話量）の全域を掃くので、
既存行を読んで分布を取るより変化を漏れなく見られます。

出力例（抜粋）:

```
=== 履歴書レビュー → user_weight_scores ===
score critical | 旧(細部/コミュ/技術) | 新(細部/コミュ)
    0        0 | 書き込みなし          | 0/20
   50        0 | 書き込みなし          | 50/50
  100        0 | 加点 100/100/100     | 100/80

=== 面接レポート → user_weight_scores ===
rubric evidence文字数 平均回答文字数 ターン | 旧 | 新
     3              0              0      0 |  60 |  60
     3            200            120     10 |  60 |  65
```

evidence も発話統計も無い行が補正なし（`60`）なのは仕様です。
**記録の不備で下げない**（#1554）ため、どちらの欠損も中立扱いにしています。

変換式と各カテゴリの対応の根拠は
[スコアリング設計 §6.3](./scoring.md) と
`Backend/internal/services/flywheel/score_mapping.go` にあります。

この出力は `testdata/score_mapping_dryrun.golden` として固定されています。
キャリブレーション定数（`criticalPenaltyPerItem` / `evidenceFullRunes` /
`evidenceSignalWeight` など）を**意図して**変えたときは、
差分を確認した上で次のコマンドでゴールデンを更新してください。

```sh
cd Backend && go test ./internal/services/flywheel/ -run DryRun -update
```

**既存行は一括再計算しません。** 次のレビュー・面接から移動平均
（`既存*0.7 + 新*0.3`）で徐々に置き換わります。
このとき新値は ×0.3 され、`score` が int なので差分の整数化でさらに潰れます。
**ドライランの分解能がそのまま保存されるのは初回書き込みのときだけ**です
（[スコアリング設計 §6.3](./scoring.md) 参照）。

---

## 注意事項

1. **サンプル不足時はキャリブレーション不要**: 各カテゴリ5件未満ではエラーになります。十分なデータが溜まってから実行してください。

2. **キャリブレーションの過剰適用に注意**: 頻繁に実行するとノイズに過適合する可能性があります。月1回程度が推奨です。

3. **重みはマッチング計算に自動反映されません**: 現時点では重みはレポート参照用です。マッチングサービスへの統合は今後の実装項目です。
