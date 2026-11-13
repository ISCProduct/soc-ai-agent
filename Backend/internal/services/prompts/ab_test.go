package prompts

import (
	"context"
	"math"
	"testing"

	"Backend/internal/usagectx"
)

// #1976: プロンプトの A/B 割当。

func registerTestCandidate(t *testing.T, stage Stage, version string, bodies map[string]string) {
	t.Helper()
	t.Cleanup(RegisterCandidateBodies(stage, version, bodies))
}

func TestAssignPromptVersion_同じIDは同じ版になる(t *testing.T) {
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補"})
	t.Setenv("PROMPT_AB_SCORE_EVALUATION", "v2:50,v3:50")

	for id := uint(1); id <= 200; id++ {
		first, group := AssignPromptVersion(StageScoreEvaluation, UserSubject(id))
		if group != first {
			t.Fatalf("id=%d: group=%q version=%q", id, group, first)
		}
		for range 3 {
			if again, _ := AssignPromptVersion(StageScoreEvaluation, UserSubject(id)); again != first {
				t.Fatalf("id=%d: 割当が変わった %q → %q", id, first, again)
			}
		}
	}
}

func TestAssignPromptVersion_分布は設定の比率に近い(t *testing.T) {
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補"})
	t.Setenv("PROMPT_AB_SCORE_EVALUATION", "v2:70,v3:30")

	const n = 20000
	counts := map[string]int{}
	for id := uint(1); id <= n; id++ {
		version, _ := AssignPromptVersion(StageScoreEvaluation, UserSubject(id))
		counts[version]++
	}
	got := float64(counts["v3"]) / n
	if math.Abs(got-0.30) > 0.015 || counts["v2"]+counts["v3"] != n {
		t.Fatalf("v3 の割合 = %.4f（counts=%v）, want 0.30 ± 0.015", got, counts)
	}
}

func TestAssignPromptVersion_無効な設定は今の版のまま(t *testing.T) {
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補"})
	registerTestCandidate(t, StageCompanyExtraction, "v3", map[string]string{BodyCompanyExtractSearch: "検索だけ"})
	tests := []struct {
		name  string
		stage Stage
		env   string
		value string
	}{
		{name: "未設定", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: ""},
		{name: "候補が1つ", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v3:100"},
		{name: "本文が無い版", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v2:50,v9:50"},
		{name: "比率が0", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v2:0,v3:50"},
		{name: "比率が数字でない", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v2:half,v3:50"},
		{name: "比率が上限超え", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v2:10001,v3:1"},
		{name: "版の重複", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v3:50,v3:50"},
		{name: "版の形式が不正", stage: StageScoreEvaluation, env: "PROMPT_AB_SCORE_EVALUATION", value: "v2:50,v 3:50"},
		{name: "本文を差し替えられない工程", stage: StageQuestionGeneration, env: "PROMPT_AB_QUESTION_GENERATION", value: "v2:50,v3:50"},
		{name: "本文の一部しか無い版", stage: StageCompanyExtraction, env: "PROMPT_AB_COMPANY_EXTRACTION", value: "v2:50,v3:50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)
			for id := uint(1); id <= 50; id++ {
				version, group := AssignPromptVersion(tt.stage, UserSubject(id))
				if version != "v2" || group != "" {
					t.Fatalf("id=%d: version=%q group=%q, want v2 と空", id, version, group)
				}
			}
		})
	}
}

func TestAssignPromptVersion_単位が無ければ対象外(t *testing.T) {
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補"})
	t.Setenv("PROMPT_AB_SCORE_EVALUATION", "v2:1,v3:10000")
	for _, subject := range []Subject{{}, UserSubject(0), CompanySubject(0), {Kind: "tenant", ID: 5}} {
		if version, group := AssignPromptVersion(StageScoreEvaluation, subject); version != "v2" || group != "" {
			t.Fatalf("subject=%+v: version=%q group=%q", subject, version, group)
		}
	}
}

func TestBody(t *testing.T) {
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補の本文"})
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "今の版は今の本文", version: "v2", want: "今の本文"},
		{name: "版が空なら今の本文", version: "", want: "今の本文"},
		{name: "候補の版は候補の本文", version: "v3", want: "候補の本文"},
		{name: "本文の無い版は今の本文", version: "v9", want: "今の本文"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Body(StageScoreEvaluation, tt.version, BodyAnswerQuality, "今の本文"); got != tt.want {
				t.Fatalf("Body = %q, want %q", got, tt.want)
			}
		})
	}
}

// HTTP の経路は context の実行主体（利用者）を単位に割り当て、群を利用量の記録へ載せる。
func TestWithPromptVersion_実行主体で割り当てて群を載せる(t *testing.T) {
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補"})
	t.Setenv("PROMPT_AB_SCORE_EVALUATION", "v2:50,v3:50")

	ctx := usagectx.WithActor(context.Background(), 42, 0)
	ctx, version := WithPromptVersion(ctx, StageScoreEvaluation)
	want, _ := AssignPromptVersion(StageScoreEvaluation, UserSubject(42))
	got := usagectx.PromptVersionFromContext(ctx)
	if version != want || got.Version != want || got.Group != want || got.Stage != string(StageScoreEvaluation) {
		t.Fatalf("version=%q context=%+v, want %q", version, got, want)
	}

	// 実行主体が無ければ今の版で、群は空
	ctx, version = WithPromptVersion(context.Background(), StageScoreEvaluation)
	if got := usagectx.PromptVersionFromContext(ctx); version != "v2" || got.Group != "" {
		t.Fatalf("version=%q context=%+v", version, got)
	}
}

// 企業情報の抽出は企業を単位にする。同じ ID でも利用者と企業は別の割当になりうる。
func TestWithPromptVersionFor_企業を単位にする(t *testing.T) {
	registerTestCandidate(t, StageCompanyExtraction, "v3", map[string]string{
		BodyCompanyExtractSearch: "検索の候補", BodyCompanyExtractFacts: "事実の候補",
	})
	t.Setenv("PROMPT_AB_COMPANY_EXTRACTION", "v2:50,v3:50")

	ctx := usagectx.WithActor(context.Background(), 1, 0)
	ctx, version := WithPromptVersionFor(ctx, StageCompanyExtraction, CompanySubject(9))
	want, _ := AssignPromptVersion(StageCompanyExtraction, CompanySubject(9))
	if got := usagectx.PromptVersionFromContext(ctx); version != want || got.Group != want {
		t.Fatalf("version=%q context=%+v, want %q", version, got, want)
	}
}
