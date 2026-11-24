package interview

import (
	"Backend/internal/models"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// promptNoncePattern は WrapUntrustedText が毎回変えるノンス（base32 26文字）にマッチする。
var promptNoncePattern = regexp.MustCompile(`_[A-Z2-7]{26}_(START|END)>>>`)

// normalizePromptNonce はノンスを固定文字列に置き換え、出力を golden と比べられるようにする。
func normalizePromptNonce(s string) string {
	return promptNoncePattern.ReplaceAllString(s, "_NONCE_${1}>>>")
}

// TestBuildInterviewSystemPromptGolden は buildInterviewSystemPrompt の出力を固定する特性テスト（#2569）。
// 節ごとの関数へ分けても組み立て順と文字列が1バイトも変わらないことを確かめる。
// プロンプトを意図して変えたときだけ UPDATE_SYSTEM_PROMPT_GOLDEN=1 で golden を作り直す。
// 入力はすべて合成データ。
func TestBuildInterviewSystemPromptGolden(t *testing.T) {
	intent := "数字で語れるか"
	qid1, qid2 := uint(11), uint(12)
	customWithIntent := []models.InterviewCompanyQuestion{
		{ID: qid1, Category: "志望動機", QuestionText: "弊社を選んだ理由を教えてください。", IsRequired: true, EvaluationIntent: &intent},
		{ID: qid2, Category: "チーム", QuestionText: "チームでの役割を教えてください。"},
	}
	customPlain := []models.InterviewCompanyQuestion{
		{ID: 21, Category: "経験", QuestionText: "学生時代の取り組みを教えてください。", IsRequired: true},
		{ID: 22, Category: "将来", QuestionText: "5年後の姿を教えてください。"},
	}
	skills := []models.SkillScore{
		{Category: models.SkillCategoryFrontend, Score: 45},
		{Category: models.SkillCategoryBackend, Score: 80},
		{Category: models.SkillCategoryInfra, Score: 10},
		{Category: models.SkillCategoryDB, Score: 30},
	}
	lowSkills := []models.SkillScore{{Category: models.SkillCategoryBackend, Score: 20}}

	type args struct {
		companyName, companyReading, position, companyInfo, companyType string
		customQuestions                                                 []models.InterviewCompanyQuestion
		skillScores                                                     []models.SkillScore
		turnCount, remainingSeconds, questionIndex, totalQuestions      int
		questionElapsedSeconds, questionDurationSeconds                 int
		directive                                                       *questionDirective
	}
	tests := []struct {
		name string
		a    args
	}{
		{"minimal_defaults", args{}},
		{"standard_topic_pace", args{
			companyName: "テスト商事", companyReading: "てすとしょうじ", position: "営業",
			companyInfo: "文化: フラット\n働き方: リモート可", companyType: "general",
			turnCount: 5, remainingSeconds: 90, questionIndex: 3, totalQuestions: 5,
			questionElapsedSeconds: 60, questionDurationSeconds: 240,
		}},
		{"directive_standard_category_engineer_sier", args{
			companyName: "サンプル情報システム", position: "バックエンドエンジニア", companyType: "sier",
			customQuestions: customWithIntent, skillScores: skills,
			turnCount: 2, remainingSeconds: 600, questionIndex: 2, totalQuestions: 4,
			questionElapsedSeconds: 30, questionDurationSeconds: 180,
			directive: &questionDirective{
				Text: "弊社を選んだ理由を教えてください。", Category: "自己紹介・志望動機",
				CompanyQuestionID: &qid1, TotalQuestions: 4,
			},
		}},
		{"directive_deepening_company_category", args{
			companyName: "テスト工業", position: "企画", companyInfo: "  ",
			customQuestions: customWithIntent,
			turnCount:       3, remainingSeconds: 100, questionIndex: 9, totalQuestions: 4,
			questionElapsedSeconds: 500, questionDurationSeconds: 120,
			directive: &questionDirective{
				Text: "定型の深掘り文", Category: "企業独自カテゴリ", IsDeepening: true,
				CompanyQuestionID: &qid1, DeepeningOf: "チームでの役割を教えてください。",
				DeepeningFocus: "結果（Result）を具体的に聞いてください。",
			},
		}},
		{"directive_deepening_no_focus_no_intent", args{
			position:  "開発",
			turnCount: 1, questionIndex: 1, totalQuestions: 3, questionElapsedSeconds: -5,
			directive: &questionDirective{
				Text: "定型", Category: "強み・弱み", IsDeepening: true, DeepeningOf: "強みを教えてください。",
			},
		}},
		{"custom_questions_without_directive_low_skills", args{
			companyName: "テスト食品", position: "Developer",
			customQuestions: customPlain, skillScores: lowSkills,
			turnCount: 4, remainingSeconds: 120, questionIndex: 2, totalQuestions: 5,
		}},
		{"custom_questions_with_intent_non_engineer", args{
			position: "事務", customQuestions: customWithIntent, skillScores: skills,
			turnCount: 99, remainingSeconds: 60, questionIndex: 5, totalQuestions: 5,
		}},
		{"directive_empty_text_category_only", args{
			companyReading: "よみのみ", companyType: "sier",
			customQuestions: customPlain,
			turnCount:       7, remainingSeconds: 30,
			directive: &questionDirective{Text: "  ", Category: "逆質問"},
		}},
		{"directive_blank_category", args{
			companyName: "テスト物流",
			turnCount:   3, remainingSeconds: 200,
			directive: &questionDirective{Text: "次の質問です。", Category: "  "},
		}},
	}
	update := os.Getenv("UPDATE_SYSTEM_PROMPT_GOLDEN") == "1"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.a
			got := normalizePromptNonce(buildInterviewSystemPrompt(
				a.companyName, a.companyReading, a.position, a.companyInfo, a.companyType,
				a.customQuestions, a.skillScores,
				a.turnCount, a.remainingSeconds, a.questionIndex, a.totalQuestions,
				a.questionElapsedSeconds, a.questionDurationSeconds, a.directive,
			))
			path := filepath.Join("testdata", "system_prompt", tt.name+".golden")
			if update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("golden の読み込みに失敗: %v", err)
			}
			if got != string(want) {
				t.Errorf("システムプロンプトが golden と一致しない（%s）。\n--- 実際の出力 ---\n%s\n--- golden ---\n%s", path, got, want)
			}
		})
	}
}
