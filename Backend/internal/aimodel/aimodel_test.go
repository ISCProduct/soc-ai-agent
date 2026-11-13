package aimodel

import "testing"

func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvPremium, "")
	t.Setenv(EnvLight, "")
	for _, s := range Stages() {
		t.Setenv(EnvOf(s), "")
	}
}

// 何も設定しなければ今までと同じ（導入だけでコストが変わらない）。
func TestFor_未設定なら今までの既定(t *testing.T) {
	clearEnv(t)
	want := map[Stage]string{
		StageInterviewResponse: "gpt-4o-mini",
		StageInterviewReport:   "",
		StageScoreEvaluation:   "",
		StageMatchingReason:    "",
		StageCompanyExtract:    "gpt-4o-mini",
		StageCompanyParse:      "gpt-4o-mini",
		StageChatSummary:       "",
	}
	for _, s := range Stages() {
		if got := For(s); got != want[s] {
			t.Errorf("%s = %q, want %q", s, got, want[s])
		}
	}
	if len(want) != len(Stages()) {
		t.Fatalf("工程の数が変わった。期待値の表も更新すること")
	}
}

func TestFor_層の設定と工程の設定(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvPremium, "gpt-4o")
	t.Setenv(EnvLight, "gpt-4.1-nano")
	t.Setenv("OPENAI_INTERVIEW_MODEL", "gpt-4o-mini") // 工程ごとの設定が層より優先

	tests := map[Stage]string{
		StageInterviewResponse: "gpt-4o-mini",
		StageInterviewReport:   "gpt-4o",
		StageScoreEvaluation:   "gpt-4o",
		StageMatchingReason:    "gpt-4o",
		StageCompanyExtract:    "gpt-4.1-nano",
		StageCompanyParse:      "gpt-4.1-nano",
		StageChatSummary:       "gpt-4.1-nano",
	}
	for s, want := range tests {
		if got := For(s); got != want {
			t.Errorf("%s = %q, want %q", s, got, want)
		}
	}
}

// 学生に見える判断は上位、内部の定型処理は軽量。分類を変えるときはテストも直す。
func TestTierOf(t *testing.T) {
	premium := []Stage{StageInterviewResponse, StageInterviewReport, StageScoreEvaluation, StageMatchingReason}
	light := []Stage{StageCompanyExtract, StageCompanyParse, StageChatSummary}
	for _, s := range premium {
		if TierOf(s) != Premium {
			t.Errorf("%s は上位の層のはず", s)
		}
	}
	for _, s := range light {
		if TierOf(s) != Light {
			t.Errorf("%s は軽量の層のはず", s)
		}
	}
	if For("unknown") != "" {
		t.Error("未知の工程は呼び出し側の既定のまま")
	}
}
