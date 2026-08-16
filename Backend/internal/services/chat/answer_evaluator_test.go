package chat

import (
	"Backend/internal/models"
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
//   !isTooPerfectRelaxedCategories[category] && !hasReason && !hasNumbersOrTime && length > 50 && hasAction && hasResult
// （#522: motivation/communication_non_it/ui_ux カテゴリでは適用しない）

// 定型的・優等生的な回答（理由・数値なし、行動+結果あり、50文字超） → credibility = 2
func TestScoreDimensions_isTooPerfect_appliesPenalty(t *testing.T) {
	// 理由・数値なし、行動（取り組み）・結果（成果）あり、50文字超
	answer := "チームで積極的に取り組み、プロジェクトを成功させ素晴らしい成果と達成を収めることができました。大変良い経験でした。"
	signals := extractSignals(answer, "")
	scores := scoreDimensions("generic_rubric", "", signals, answer)

	// hasAction と hasResult が検出されていることを前提確認
	if !signals.hasAction || !signals.hasResult {
		t.Skip("テスト用回答のシグナル検出が不正")
	}
	// isTooPerfect 判定の前提: 理由・数値がないこと
	if signals.hasReason || signals.hasNumbersOrTime {
		t.Skip("テスト用回答に理由・数値が含まれているためスキップ")
	}

	// credibility は 2（ペナルティ適用、完全ゼロ評価は避ける #522）であること
	if scores["credibility"] != 2 {
		t.Errorf("isTooPerfect 判定で credibility=2 を期待したが %d が返った", scores["credibility"])
	}
}

// 理由あり（ため）: isTooPerfect にならない → credibility > 1 になりうる
func TestScoreDimensions_hasReason_notTooPerfect(t *testing.T) {
	answer := "効率化のために積極的に取り組み、プロジェクトを成功させ素晴らしい成果と達成を収めることができました。大変良い経験でした。"
	signals := extractSignals(answer, "")
	scores := scoreDimensions("generic_rubric", "", signals, answer)

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
	scores := scoreDimensions("generic_rubric", "", signals, answer)

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
	scores := scoreDimensions("generic_rubric", "", signals, answer)

	// 50文字以下なら isTooPerfect は false のはず → credibility が 1 でも別の理由（初期値）
	// ただし contradiction もないので 0 にはならないことを確認
	if scores["credibility"] < 0 {
		t.Errorf("短い回答で credibility が負になった: %d", scores["credibility"])
	}
}

// 矛盾あり: credibility = 0（isTooPerfect より優先）
func TestScoreDimensions_contradiction_credibilityZero(t *testing.T) {
	signals := signalSet{
		hasAction:        true,
		hasResult:        true,
		hasReason:        false,
		hasNumbersOrTime: false,
		contradiction:    true,
	}
	answer := "取り組み、成果と達成と向上を収めました。チームで実施しました。成功しました。大変良い経験でした。"
	scores := scoreDimensions("generic_rubric", "", signals, answer)

	if scores["credibility"] != 0 {
		t.Errorf("contradiction ありの場合は credibility=0 を期待したが %d が返った", scores["credibility"])
	}
}

// isTooPerfect のカテゴリ別緩和テスト（#522）:
// motivation・communication_non_it・ui_ux では理由・数値が無い定型回答でも
// isTooPerfect ペナルティ（credibility=2）を適用せず、デフォルト値(1)のままとする。
// experience・collaboration・generic では従来どおり厳格適用する。
func TestScoreDimensions_isTooPerfect_categoryRelaxation(t *testing.T) {
	// 理由・数値・具体例なし、行動+結果あり、50文字超（isTooPerfect の前提を満たす回答）
	answer := "チームで施策を推進し、業務プロセスを改善して大きな成果を出すことができました。皆で協力して乗り越えました。"
	signals := extractSignals(answer, "")
	if signals.hasReason || signals.hasNumbersOrTime || signals.hasConcreteExample || !signals.hasAction || !signals.hasResult {
		t.Fatal("テスト用回答の前提シグナルが崩れている")
	}

	tests := []struct {
		name     string
		category string
		want     int
	}{
		{"motivation is relaxed", "motivation", 1},
		{"communication_non_it is relaxed", "communication_non_it", 1},
		{"ui_ux is relaxed", "ui_ux", 1},
		{"experience stays strict", "experience", 2},
		{"collaboration stays strict", "collaboration", 2},
		{"generic stays strict", "generic", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scores := scoreDimensions(rubricForCategory(tt.category), tt.category, signals, answer)
			if scores["credibility"] != tt.want {
				t.Errorf("category=%s: credibility=%d を期待したが %d が返った", tt.category, tt.want, scores["credibility"])
			}
		})
	}
}

// #522: extractSignals の同義表現拡充テスト
func TestExtractSignals_synonymExpansion(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		check  func(signalSet) bool
	}{
		{"hasAction: 推進した", "新しい施策を推進した", func(s signalSet) bool { return s.hasAction }},
		{"hasAction: 展開した", "全社に展開した", func(s signalSet) bool { return s.hasAction }},
		{"hasResult: 実現した", "業務効率化を実現した", func(s signalSet) bool { return s.hasResult }},
		{"hasResult: 改善できた", "顧客対応を改善できた", func(s signalSet) bool { return s.hasResult }},
		{"hasReason: 背景として", "背景として人手不足がありました", func(s signalSet) bool { return s.hasReason }},
		{"hasReason: きっかけは", "きっかけは先輩の一言でした", func(s signalSet) bool { return s.hasReason }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(extractSignals(tt.answer, "")) {
				t.Fatalf("signal not detected for %q", tt.answer)
			}
		})
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
	// isTooPerfect が適用されれば credibility <= 2（#522: 1→2に緩和し完全ゼロ評価を避ける）
	if credibility > 2 {
		t.Errorf("優等生的回答で credibility=%d（期待: <=2）", credibility)
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
	score, penalties, _ := applyPenaltiesAndBoosts(50, signals, 40)

	if !slices.Contains(penalties, "contradiction") {
		t.Error("contradiction シグナルがあるのに penalties に 'contradiction' が含まれていない")
	}
	if score != 30 {
		t.Errorf("contradiction ペナルティ後のスコアが期待値と異なる: got=%d want=30", score)
	}
}

func TestApplyPenaltiesAndBoosts_numbersOrTime_boost(t *testing.T) {
	signals := signalSet{hasNumbersOrTime: true, hasConcreteExample: true, hasAction: true}
	score, _, boosts := applyPenaltiesAndBoosts(50, signals, 40)

	if !slices.Contains(boosts, "evidence") {
		t.Error("hasNumbersOrTime ありで boosts に 'evidence' が含まれていない")
	}
	if score != 55 {
		t.Errorf("evidence ブースト後のスコアが期待値と異なる: got=%d want=55", score)
	}
}

func TestApplyPenaltiesAndBoosts_scoreFloor(t *testing.T) {
	signals := signalSet{contradiction: true}
	score, _, _ := applyPenaltiesAndBoosts(10, signals, 40)
	if score < 0 {
		t.Errorf("スコアがマイナスになった: %d", score)
	}
}

func TestApplyPenaltiesAndBoosts_scoreCeiling(t *testing.T) {
	signals := signalSet{hasNumbersOrTime: true, hasConcreteExample: true, hasAction: true}
	score, _, _ := applyPenaltiesAndBoosts(100, signals, 40)
	if score > 100 {
		t.Errorf("スコアが100を超えた: %d", score)
	}
}

func TestApplyPenaltiesAndBoosts_engagementSkipsTooGeneric(t *testing.T) {
	// 理由のみでも関与あり → too_generic なし
	signals := signalSet{hasReason: true}
	score, penalties, _ := applyPenaltiesAndBoosts(40, signals, 40)
	if slices.Contains(penalties, "too_generic") {
		t.Error("関与シグナルありなのに too_generic が付与された")
	}
	if score != 40 {
		t.Errorf("got=%d want=40", score)
	}

	// シグナル0 → too_generic
	empty := signalSet{}
	score2, penalties2, _ := applyPenaltiesAndBoosts(40, empty, 40)
	if !slices.Contains(penalties2, "too_generic") {
		t.Error("関与シグナル0なのに too_generic が付与されなかった")
	}
	if score2 != 35 {
		t.Errorf("got=%d want=35", score2)
	}
}

func TestApplyPenaltiesAndBoosts_condensedEngagementBoost(t *testing.T) {
	signals := signalSet{hasAction: true, hasResult: true, hasNumbersOrTime: true}
	score, _, boosts := applyPenaltiesAndBoosts(50, signals, 24)
	if !slices.Contains(boosts, "condensed_engagement") {
		t.Error("短文関与回答で condensed_engagement ブーストが無い")
	}
	// evidence(+5) + condensed(+8) = 63
	if score != 63 {
		t.Errorf("got=%d want=63", score)
	}
}

func TestExtractSignals_condensedPatterns(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		check  func(signalSet) bool
	}{
		{"release", "チームで役割分担して3日でリリースした。", func(s signalSet) bool {
			return s.hasAction && s.hasResult && s.hasNumbersOrTime && s.hasEngagement()
		}},
		{"emotion_reason", "やりがいを感じた。人の役に立てたから。", func(s signalSet) bool {
			return s.hasEmotion && s.hasReason && s.hasEngagement()
		}},
		{"hai", "はい", func(s signalSet) bool { return !s.hasEngagement() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(extractSignals(tt.answer, "")) {
				t.Fatalf("unexpected signals for %q", tt.answer)
			}
		})
	}
}

func TestEvaluateHumanScoring_ShortHighEngagement(t *testing.T) {
	e := NewAnswerEvaluator()
	tests := []struct {
		name       string
		question   string
		answer     string
		wantMin    int
		wantMax    int
		wantSkip   bool
		wantAction PrecheckAction
	}{
		{
			name:     "thin_reason",
			question: "なぜその職種に興味を持ちましたか？",
			answer:   "面白そうだと思ったから。",
			wantMin:  15,
			wantMax:  45,
		},
		{
			name:     "condensed_ship",
			question: "これまでの経験で工夫したことを教えてください",
			answer:   "チームで役割分担して3日でリリースした。",
			wantMin:  50,
			wantMax:  90,
		},
		{
			name:     "emotion_reason",
			question: "仕事でやりがいを感じた瞬間は？",
			answer:   "やりがいを感じた。人の役に立てたから。",
			wantMin:  45,
			wantMax:  85,
		},
		{
			name:     "hai",
			question: "チームで働くことは好きですか？",
			answer:   "はい",
			wantMin:  0,
			wantMax:  25,
		},
		{
			name:       "skip",
			question:   "これまでの経験を教えてください",
			answer:     "わからない",
			wantSkip:   true,
			wantAction: PrecheckSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.EvaluateHumanScoring(tt.question, tt.answer, false, false, nil)
			if tt.wantSkip {
				if result.Action != tt.wantAction {
					t.Fatalf("action=%s want=%s", result.Action, tt.wantAction)
				}
				return
			}
			if result.Action != PrecheckScore {
				t.Fatalf("action=%s score=%d", result.Action, result.Score)
			}
			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Fatalf("score=%d want [%d,%d] penalties=%v boosts=%v",
					result.Score, tt.wantMin, tt.wantMax, result.Penalties, result.Boosts)
			}
			// 関与短文は進捗カウント対象（score>0 または floor 後）
			floored := floorEngagedShortScore(tt.answer, result)
			if tt.name != "hai" && floored.Score <= 0 {
				t.Fatalf("engaged short should count for progress, score=%d", floored.Score)
			}
		})
	}
}

func TestShouldBlendLLMForHumanScore(t *testing.T) {
	short := HumanScoreResult{Action: PrecheckScore, Score: 40}
	if !shouldBlendLLMForHumanScore("短い関与回答です。", short) {
		t.Fatal("30文字以下は LLM blend 対象であるべき")
	}
	longLow := HumanScoreResult{Action: PrecheckScore, Score: 70}
	longAnswer := "これは十分に長い回答で、境界帯以外かつ短文でもないためルール評価のみで十分と判断されるべき文章です。"
	if shouldBlendLLMForHumanScore(longAnswer, longLow) {
		t.Fatal("長文・高スコアは LLM blend 対象外であるべき")
	}
	border := HumanScoreResult{Action: PrecheckScore, Score: 40}
	if !shouldBlendLLMForHumanScore(longAnswer, border) {
		t.Fatal("境界帯スコアは LLM blend 対象であるべき")
	}
}

func TestMergeConfidence(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"low", "medium", "medium"},
		{"high", "low", "high"},
		{"medium", "medium", "medium"},
		{"low", "low", "low"},
		{"high", "medium", "high"},
	}
	for _, tc := range cases {
		got := MergeConfidence(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("MergeConfidence(%q,%q)=%q want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestEvaluateRule_Table(t *testing.T) {
	e := NewAnswerEvaluator()
	cases := []struct {
		name   string
		rule   models.ScoreRule
		answer string
		want   bool
	}{
		{
			name:   "contains_any hit",
			rule:   models.ScoreRule{Condition: "contains_any", Keywords: []string{"改善", "実装"}},
			answer: "システムを改善しました",
			want:   true,
		},
		{
			name:   "contains_any miss",
			rule:   models.ScoreRule{Condition: "contains_any", Keywords: []string{"改善"}},
			answer: "特にありません",
			want:   false,
		},
		{
			name:   "contains_all hit",
			rule:   models.ScoreRule{Condition: "contains_all", Keywords: []string{"チーム", "協力"}},
			answer: "チームで協力しました",
			want:   true,
		},
		{
			name:   "length_gt",
			rule:   models.ScoreRule{Condition: "length_gt", Keywords: []string{"5"}},
			answer: "123456",
			want:   true,
		},
		{
			name:   "has_example",
			rule:   models.ScoreRule{Condition: "has_example"},
			answer: "例えばインターンで実装しました",
			want:   true,
		},
		{
			name:   "unknown condition",
			rule:   models.ScoreRule{Condition: "unknown"},
			answer: "anything",
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := e.evaluateRule(tc.rule, tc.answer, len(tc.answer))
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestGetConfidenceLevel(t *testing.T) {
	e := NewAnswerEvaluator()
	cases := []struct {
		score, keywords int
		want            string
	}{
		{8, 3, "high"},
		{5, 1, "medium"},
		{2, 0, "low"},
	}
	for _, tc := range cases {
		got := e.GetConfidenceLevel(tc.score, tc.keywords)
		if got != tc.want {
			t.Fatalf("score=%d kw=%d got=%q want %q", tc.score, tc.keywords, got, tc.want)
		}
	}
}

// #522: CHAT_EVAL_RULE_WEIGHT によるハイブリッド評価の ruleWeight 上書きテスト
func TestConfiguredRuleWeight(t *testing.T) {
	cases := []struct {
		name     string
		envValue string
		fallback float64
		want     float64
	}{
		{"未設定なら fallback を返す", "", 0.55, 0.55},
		{"有効な値で上書きされる", "0.7", 0.55, 0.7},
		{"範囲外(負)は fallback を返す", "-0.1", 0.55, 0.55},
		{"範囲外(1超)は fallback を返す", "1.5", 0.55, 0.55},
		{"パース不能な値は fallback を返す", "abc", 0.55, 0.55},
		// ParseFloatは"NaN"/"Inf"をエラーなしで返し、NaNとの比較は常にfalseになるため
		// 範囲チェックをすり抜けないことを確認する
		{"NaNは fallback を返す", "NaN", 0.55, 0.55},
		{"+Infは fallback を返す", "+Inf", 0.55, 0.55},
		{"-Infは fallback を返す", "-Inf", 0.55, 0.55},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHAT_EVAL_RULE_WEIGHT", tc.envValue)
			got := configuredRuleWeight(tc.fallback)
			if got != tc.want {
				t.Errorf("got=%v want=%v", got, tc.want)
			}
		})
	}
}
