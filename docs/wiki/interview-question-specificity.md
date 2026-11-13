# 面接の質問の企業固有性の評価（#2274 段階0）

AI面接の質問が「その企業ならではの質問」か「どの企業にも通用する一般的な質問」かを数字で測る評価基盤。
質問の生成を変えたとき、効果を数字で言えるようにするためのもの。**質問の生成そのものは変えない**（`services/interview/` は未変更）。

実装: `Backend/internal/aibench/question_specificity.go`
ゴールデンセット: `docs/research/ai-eval/interview-question.jsonl`
既存の評価ハーネス全般: `docs/research/ai-eval/README.md`

## 指標

| 指標 | 定義 | 方向 | LLM |
|---|---|---|---|
| 企業固有語率 `SpecificTermRate` | 質問のうち、その企業の固有語（社名の「株式会社」抜き + 各事実カードの `terms`）を1語以上含む質問の割合 | 高いほど良い | 不要 |
| 事実の実在検証 `UnknownFactIDs` | 質問が引用した事実カードID（`fact_ids`）のうち、与えた事実カードに無いもの。1件でもあれば違反 | 0件が正 | 不要 |
| 差別的表現 `DiscriminatoryHits` | 家族構成・親の職業・出身地・本籍・国籍・宗教・支持政党・思想を聞く表現のカテゴリ。1件でもあれば違反 | 0件が正 | 不要 |
| 企業入替テスト `GenericRate` | 質問を別の企業 `swapN` 社の文脈に置き、判定器が「通用する」と答えた割合 | **低いほど良い**（高いと一般的） | 要（有料） |

固有語は「他社の社名・事実本文に現れない語」でなければならない。これは `Testゴールデンセットの検証` が機械的に検査している（被ると固有語率が意味を失うため）。

差別的表現は聞き方の形で絞ったキーワード一致。「ご出身の大学」「家族のようなチーム」は検出しない。言い換え（遠回しな聞き方）は拾えない。

## ゴールデンセット

- **架空の企業のみ**（実在企業の情報は使わない。企業データは正しさ優先のため）。現在 9社10行（ミナトロジスティクスのみ2職種）。
- 1行 = 企業 × 職種。`facts`（事実カード: business / culture / work_style / tech / requirement。各 `id`・`text`・`terms`）、`top_axes`（重視する人物像の上位軸）、`history`（面接の直前までの会話）、`note`。
- 更新ルール
  1. 社名・事業名・技術語は架空にする。実在の社名・製品名と一致しないか確認する
  2. 事実カードを足すときは、固有語（`terms`）を必ず付ける。固有語は**事実本文に含まれ、他社の事実・社名には含まれない**こと
  3. 一般語（「チーム」「成長」など）は固有語にしない
  4. 事実の追加・削除は `id` を使い回さない（引用の実在検証の前提が崩れる）
  5. 変更したら `cd Backend && go test ./internal/aibench/...`

## 実行方法

指標のうち LLM を使わない3つはテスト（`go test ./internal/aibench/...`）で動作を確認している。**CI で動くのは指標の関数とゴールデンセットの検証だけ**で、実際の質問は測っていない。

生成済みの質問を測るには、`GeneratedQuestion{Text, FactIDs}` の一覧を作り、次の関数に渡す（呼び出し側のコマンドは未整備。下の「測れていないこと」参照）。

```go
cases, _ := aibench.LoadQuestionCases("docs/research/ai-eval/interview-question.jsonl")
rate := aibench.SpecificTermRate(cases[0], questions)
unknown := aibench.UnknownFactIDs(cases[0], q)      // 空なら正常
hits := aibench.DiscriminatoryHits(q.Text)          // 空なら正常
```

## 有料実行の手順（企業入替テスト）

既定では実行されない。テストはフェイクの判定器だけを使い、OpenAI を呼ばない。

有料の判定器 `NewPaidSwapJudge` は次の4つが**すべて**そろわないとエラーを返す。

1. 環境変数 `OPENAI_API_KEY`
2. `PaidJudgeOptions.Enabled = true`（明示フラグ）
3. `MaxCalls >= 1`（呼び出し回数の上限）
4. `MaxUSD > 0`（概算金額の上限）

呼び出しごとに「回数が上限に達していないか」「累計 + この呼び出しの見積もりが `MaxUSD` を超えないか」を先に見て、超えるなら API を呼ばず `ErrJudgeBudget` を返す。

```go
judge, err := aibench.NewPaidSwapJudge(aibench.PaidJudgeOptions{
    Enabled: true, MaxCalls: 80, MaxUSD: 0.05, Model: "gpt-4o-mini",
})
generic, err := aibench.GenericRate(ctx, cases, cases[0], questions, judge, 2)
```

### 想定コスト（概算）

gpt-4o-mini、判定1回あたり入力 約550トークン・出力 約30トークン ≒ $0.0001。
10行 × 1行3問 × 入替先2社 = 60回で **約 $0.006**。上限は余裕を見て 80回・$0.05 程度を推奨。
単価は `internal/services/costs` の表を使う。実測は未実施（API を呼んでいないため）。

## 現時点で測れていないこと

- 実際の質問に対する各指標の値。質問を生成して集める経路（コマンド）が未整備で、ベースラインの数字はまだ無い
- 企業入替テスト（有料）は未実行。判定器の精度（人の判断と合うか）も未検証
- 固有語率は「語を含むか」だけで、「その語を使って質問が成立しているか」は見ない（固有語を貼っただけの質問も高く出る）。事実の実在検証・入替テストと並べて読む
- 事実の実在検証は引用IDの存在だけで、質問の中身が事実と矛盾していないかは見ない
- 差別的表現は言い換えを拾えない
- 本番の企業データの充填率（次節）

## 企業情報の充填率を測るSQL（読み取り専用・未実行）

質問が企業固有になるかは、そもそも企業情報が入っているかに左右される。staging で次を実行すれば充填率が分かる。
**このPRでは実行していない**（DB接続情報が要るため、オーナーが判断して実行する）。SELECT のみ。

```sql
-- companies の充填率（有効な企業のみ）
SELECT
  COUNT(*) AS total,
  SUM(main_business IS NOT NULL AND main_business <> '') AS has_main_business,
  SUM(culture       IS NOT NULL AND culture       <> '') AS has_culture,
  SUM(work_style    IS NOT NULL AND work_style    <> '') AS has_work_style,
  SUM(description   IS NOT NULL AND description   <> '') AS has_description
FROM companies
WHERE deleted_at IS NULL AND is_active = 1;

-- 求人の required_skills の充填率
SELECT
  COUNT(*) AS total,
  SUM(required_skills IS NOT NULL AND required_skills <> '') AS has_required_skills
FROM company_job_positions
WHERE deleted_at IS NULL AND is_active = 1;

-- company_weight_profiles を持つ企業の割合
SELECT
  COUNT(*) AS total_companies,
  SUM(EXISTS (SELECT 1 FROM company_weight_profiles p WHERE p.company_id = c.id)) AS has_weight_profile
FROM companies c
WHERE c.deleted_at IS NULL AND c.is_active = 1;
```
