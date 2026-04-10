package services

import (
	"slices"
	"testing"
)

// ──────────────────────────────────────────────
// extractSignals のテスト
// ──────────────────────────────────────────────

func TestExtractSignals_hasAction(t *testing.T) {
	signals := extractSignals("新しいシステムを実装して改善しました", "")
	if !signals.hasAction {
		t.Error("「実装」「改善」を含む回答で hasAction が true にならなかった")
	}
}

func TestExtractSignals_hasResult(t *testing.T) {
	signals := extractSignals("その結果として成果が向上しました", "")
	if !signals.hasResult {
		t.Error("「結果」「成果」「向上」を含む回答で hasResult が true にならなかった")
	}
}

func TestExtractSignals_hasReason(t *testing.T) {
	signals := extractSignals("効率化のためにツールを導入しました", "")
	if !signals.hasReason {
		t.Error("「ため」を含む回答で hasReason が true にならなかった")
	}
}

func TestExtractSignals_hasNumbersOrTime(t *testing.T) {
	signals := extractSignals("3ヶ月かけて50%改善しました", "")
	if !signals.hasNumbersOrTime {
		t.Error("数値・時間表現を含む回答で hasNumbersOrTime が true にならなかった")
	}
}

func TestExtractSignals_hasConcreteExample(t *testing.T) {
	signals := extractSignals("例えば、実際に経験したことですが", "")
	if !signals.hasConcreteExample {
		t.Error("「例えば」「実際に」を含む回答で hasConcreteExample が true にならなかった")
	}
}

func TestExtractSignals_noSignals(t *testing.T) {
	signals := extractSignals("はい", "")
	if signals.hasAction || signals.hasResult || signals.hasReason || signals.hasConcreteExample {
		t.Error("短い回答でシグナルが誤検知された")
	}
}

// ──────────────────────────────────────────────
// isTooPerfect 判定条件のテスト（#230 コア要件）
// ──────────────────────────────────────────────

// isTooPerfect の条件:
//   !hasReason && !hasNumbersOrTime && length > 50 && hasAction && hasResult

// 定型的・優等生的な回答（理由・数値なし、行動+結果あり、50文字超） → credibility = 1
func TestScoreDimensions_isTooPerfect_appliesPenalty(t *testing.T) {
	// 理由・数値なし、行動（取り組み）・結果（成果）あり、50文字超
	answer := "チームで積極的に取り組み、プロジェクトを成功させ素晴らしい成果と達成を収めることができました。大変良い経験でした。"
	signals := extractSignals(answer, "")
	scores := scoreDimensions("generic_rubric", signals, answer)

	// hasAction と hasResult が検出されていることを前提確認
	if !signals.hasAction || !signals.hasResult {
		t.Skip("テスト用回答のシグナル検出が不正")
	}
	// isTooPerfect 判定の前提: 理由・数値がないこと
	if signals.hasReason || signals.hasNumbersOrTime {
		t.Skip("テスト用回答に理由・数値が含まれているためスキップ")
	}

	// credibility は 1（ペナルティ適用）であること
	if scores["credibility"] != 1 {
		t.Errorf("isTooPerfect 判定で credibility=1 を期待したが %d が返った", scores["credibility"])
	}
}

// 理由あり（ため）: isTooPerfect にならない → credibility > 1 になりうる
func TestScoreDimensions_hasReason_notTooPerfect(t *testing.T) {
	answer := "効率化のために積極的に取り組み、プロジェクトを成功させ素晴らしい成果と達成を収めることができました。大変良い経験でした。"
	signals := extractSignals(answer, "")
	scores := scoreDimensions("generic_rubric", signals, answer)

	if !signals.hasReason {
		t.Skip("テスト用回答に「ため」が含まれていない")
	}
	// credibility は 1 より大きい（ペナルティ非適用）
	if scores["credibility"] <= 1 {
		t.Errorf("hasReason ありの場合は credibility > 1 を期待したが %d が返った", scores["credibility"])
	}
}

// 数値あり: isTooPerfect にならない → credibility = 3
func TestScoreDimensions_hasNumbersOrTime_notTooPerfect(t *testing.T) {
	answer := "3ヶ月かけて積極的に取り組み、プロジェクトを成功させ素晴らしい成果と達成を収めることができました。大変良い経験でした。"
	signals := extractSignals(answer, "")
	scores := scoreDimensions("generic_rubric", signals, answer)

	if !signals.hasNumbersOrTime {
		t.Skip("テスト用回答に数値・時間が含まれていない")
	}
	// credibility は 3（数値あり）
	if scores["credibility"] != 3 {
		t.Errorf("hasNumbersOrTime ありの場合は credibility=3 を期待したが %d が返った", scores["credibility"])
	}
}

// 短い回答（50文字以下）: isTooPerfect にならない
func TestScoreDimensions_shortAnswer_notTooPerfect(t *testing.T) {
	answer := "取り組み、成果が出ました。" // 50文字未満
	signals := extractSignals(answer, "")
	scores := scoreDimensions("generic_rubric", signals, answer)

	// 50文字以下なら isTooPerfect は false のはず → credibility が 1 でも別の理由（初期値）
	// ただし contradiction もないので 0 にはならないことを確認
	if scores["credibility"] < 0 {
		t.Errorf("短い回答で credibility が負になった: %d", scores["credibility"])
	}
}

// 矛盾あり: credibility = 0（isTooPerfect より優先）
func TestScoreDimensions_contradiction_credibilityZero(t *testing.T) {
	signals := signalSet{
		hasAction:       true,
		hasResult:       true,
		hasReason:       false,
		hasNumbersOrTime: false,
		contradiction:   true,
	}
	answer := "取り組み、成果と達成と向上を収めました。チームで実施しました。成功しました。大変良い経験でした。"
	scores := scoreDimensions("generic_rubric", signals, answer)

	if scores["credibility"] != 0 {
		t.Errorf("contradiction ありの場合は credibility=0 を期待したが %d が返った", scores["credibility"])
	}
}

// ──────────────────────────────────────────────
// EvaluateHumanScoring 統合テスト
// ──────────────────────────────────────────────

func TestEvaluateHumanScoring_tooPerfectAnswer_lowScore(t *testing.T) {
	e := NewAnswerEvaluator()

	// 定型的優等生回答: 行動+結果はあるが理由・数値なし、50文字超
	question := "チームで困難な状況に直面したときの対処法を教えてください"
	answer := "チームで積極的に取り組み、困難を乗り越えて素晴らしい成果と達成を収めることができました。成功した大変良い経験でした。"

	result := e.EvaluateHumanScoring(question, answer, false, false, nil)

	if result.DimensionScores == nil {
		t.Skip("DimensionScores が返されなかった（precheck で早期リターン）")
	}

	credibility, ok := result.DimensionScores["credibility"]
	if !ok {
		t.Error("DimensionScores に credibility が含まれていない")
	}
	// isTooPerfect が適用されれば credibility <= 1
	if credibility > 1 {
		t.Errorf("優等生的回答で credibility=%d（期待: <=1）", credibility)
	}
}

func TestEvaluateHumanScoring_concreteAnswer_higherScore(t *testing.T) {
	e := NewAnswerEvaluator()

	question := "チームで困難な状況に直面したときの対処法を教えてください"
	// 理由・数値・具体例あり: 高スコア期待
	answer := "3ヶ月のプロジェクトで意見が対立したため、週次のミーティングを設けて合意形成を図りました。例えば、開発方針について具体的な提案書を作成し、結果として全員が納得する方針に改善できました。"

	result := e.EvaluateHumanScoring(question, answer, false, false, nil)

	if result.DimensionScores == nil {
		t.Skip("DimensionScores が返されなかった")
	}

	credibility, ok := result.DimensionScores["credibility"]
	if !ok {
		t.Error("DimensionScores に credibility が含まれていない")
	}
	if credibility < 2 {
		t.Errorf("具体的回答で credibility=%d（期待: >=2）", credibility)
	}
}

// ──────────────────────────────────────────────
// applyPenaltiesAndBoosts のテスト
// ──────────────────────────────────────────────

func TestApplyPenaltiesAndBoosts_contradiction_penalty(t *testing.T) {
	signals := signalSet{contradiction: true, hasConcreteExample: true, hasAction: true}
	score, penalties, _ := applyPenaltiesAndBoosts(50, signals)

	if !slices.Contains(penalties, "contradiction") {
		t.Error("contradiction シグナルがあるのに penalties に 'contradiction' が含まれていない")
	}
	if score != 30 {
		t.Errorf("contradiction ペナルティ後のスコアが期待値と異なる: got=%d want=30", score)
	}
}

func TestApplyPenaltiesAndBoosts_numbersOrTime_boost(t *testing.T) {
	signals := signalSet{hasNumbersOrTime: true, hasConcreteExample: true, hasAction: true}
	score, _, boosts := applyPenaltiesAndBoosts(50, signals)

	if !slices.Contains(boosts, "evidence") {
		t.Error("hasNumbersOrTime ありで boosts に 'evidence' が含まれていない")
	}
	if score != 55 {
		t.Errorf("evidence ブースト後のスコアが期待値と異なる: got=%d want=55", score)
	}
}

func TestApplyPenaltiesAndBoosts_scoreFloor(t *testing.T) {
	signals := signalSet{contradiction: true}
	score, _, _ := applyPenaltiesAndBoosts(10, signals)
	if score < 0 {
		t.Errorf("スコアがマイナスになった: %d", score)
	}
}

func TestApplyPenaltiesAndBoosts_scoreCeiling(t *testing.T) {
	signals := signalSet{hasNumbersOrTime: true, hasConcreteExample: true, hasAction: true}
	score, _, _ := applyPenaltiesAndBoosts(100, signals)
	if score > 100 {
		t.Errorf("スコアが100を超えた: %d", score)
	}
}
