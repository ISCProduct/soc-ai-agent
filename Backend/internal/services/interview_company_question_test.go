package services

import (
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
)

type interviewCompanyQuestionRepoStub struct {
	questions    []models.InterviewCompanyQuestion
	err          error
	findCalls    int
	gotCompanyID uint
	gotPosition  string
}

func (r *interviewCompanyQuestionRepoStub) FindByCompanyID(uint) ([]models.InterviewCompanyQuestion, error) {
	return nil, nil
}

func (r *interviewCompanyQuestionRepoStub) FindByCompanyAndPosition(companyID uint, position string) ([]models.InterviewCompanyQuestion, error) {
	r.findCalls++
	r.gotCompanyID = companyID
	r.gotPosition = position
	return r.questions, r.err
}

func (r *interviewCompanyQuestionRepoStub) FindByID(uint) (*models.InterviewCompanyQuestion, error) {
	return nil, nil
}

func (r *interviewCompanyQuestionRepoStub) Create(*models.InterviewCompanyQuestion) error {
	return nil
}

func (r *interviewCompanyQuestionRepoStub) Update(*models.InterviewCompanyQuestion) error {
	return nil
}

func (r *interviewCompanyQuestionRepoStub) Delete(uint) error {
	return nil
}

func TestInterviewServiceFetchCustomQuestions(t *testing.T) {
	questions := []models.InterviewCompanyQuestion{
		{CompanyID: 42, Position: "", QuestionText: "全職種向け質問"},
		{CompanyID: 42, Position: "バックエンドエンジニア", QuestionText: "職種別質問"},
	}

	tests := []struct {
		name          string
		companyID     uint
		position      string
		repo          *interviewCompanyQuestionRepoStub
		wantQuestions int
		wantCalls     int
	}{
		{
			name:      "company_idが0ならデフォルト動作へフォールバックする",
			companyID: 0,
			repo:      &interviewCompanyQuestionRepoStub{questions: questions},
			wantCalls: 0,
		},
		{
			name:          "企業IDと職種を指定して共通質問と職種別質問を取得する",
			companyID:     42,
			position:      "バックエンドエンジニア",
			repo:          &interviewCompanyQuestionRepoStub{questions: questions},
			wantQuestions: 2,
			wantCalls:     1,
		},
		{
			name:      "リポジトリエラー時は面接を継続できるよう空にする",
			companyID: 42,
			position:  "バックエンドエンジニア",
			repo:      &interviewCompanyQuestionRepoStub{err: errors.New("database unavailable")},
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
			svc.SetCompanyQuestionRepo(tt.repo)

			got := svc.fetchCustomQuestions(tt.companyID, tt.position)

			if len(got) != tt.wantQuestions {
				t.Fatalf("question count = %d, want %d", len(got), tt.wantQuestions)
			}
			if tt.repo.findCalls != tt.wantCalls {
				t.Fatalf("FindByCompanyAndPosition calls = %d, want %d", tt.repo.findCalls, tt.wantCalls)
			}
			if tt.wantCalls > 0 {
				if tt.repo.gotCompanyID != tt.companyID || tt.repo.gotPosition != tt.position {
					t.Fatalf("repository args = (%d, %q), want (%d, %q)", tt.repo.gotCompanyID, tt.repo.gotPosition, tt.companyID, tt.position)
				}
			}
		})
	}
}

func TestBuildInterviewSystemPromptCustomQuestions(t *testing.T) {
	questions := []models.InterviewCompanyQuestion{
		{Category: "志望動機", QuestionText: "当社を志望した理由を教えてください。", IsRequired: true},
		{Category: "技術", QuestionText: "最近解決した技術課題を教えてください。", IsRequired: false},
	}

	tests := []struct {
		name        string
		questions   []models.InterviewCompanyQuestion
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:      "必須質問と推奨質問を別セクションへ注入する",
			questions: questions,
			wantContain: []string{
				"【必須質問（必ず全て質問してください）】",
				"- [志望動機] 当社を志望した理由を教えてください。",
				"【推奨質問（会話の流れに応じて取り入れてください）】",
				"- [技術] 最近解決した技術課題を教えてください。",
			},
		},
		{
			name:       "カスタム質問がなければ質問セクションを追加しない",
			wantAbsent: []string{"【必須質問", "【推奨質問"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := buildInterviewSystemPrompt(
				"テスト株式会社", "", "バックエンドエンジニア", "", "general",
				tt.questions, nil, 0, 0, 1, 5, 0, 180, nil,
			)

			for _, want := range tt.wantContain {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt does not contain %q", want)
				}
			}
			for _, unwanted := range tt.wantAbsent {
				if strings.Contains(prompt, unwanted) {
					t.Errorf("prompt unexpectedly contains %q", unwanted)
				}
			}
		})
	}
}

func TestBuildInterviewSystemPromptSkillHints(t *testing.T) {
	scores := []models.SkillScore{
		{Category: models.SkillCategoryBackend, Score: 85},
		{Category: models.SkillCategoryInfra, Score: 70},
		{Category: models.SkillCategoryFrontend, Score: 20},
	}

	tests := []struct {
		name        string
		position    string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:     "エンジニア職には閾値以上のスキル質問を追加する",
			position: "バックエンドエンジニア",
			wantContain: []string{
				"【GitHubスキル分析に基づく技術質問ガイドライン】",
				"バックエンド（スコア85）",
				"インフラ（スコア70）",
			},
			wantAbsent: []string{"フロントエンド（スコア20）"},
		},
		{
			name:       "非エンジニア職にはスキル質問を追加しない",
			position:   "法人営業",
			wantAbsent: []string{"【GitHubスキル分析に基づく技術質問ガイドライン】", "バックエンド（スコア85）"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := buildInterviewSystemPrompt(
				"テスト株式会社", "", tt.position, "", "general",
				nil, scores, 0, 0, 1, 5, 0, 180, nil,
			)

			for _, want := range tt.wantContain {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt does not contain %q", want)
				}
			}
			for _, unwanted := range tt.wantAbsent {
				if strings.Contains(prompt, unwanted) {
					t.Errorf("prompt unexpectedly contains %q", unwanted)
				}
			}
		})
	}
}

func TestBuildTechQuestionHintsOrdersScoresDescending(t *testing.T) {
	hints := buildTechQuestionHints([]models.SkillScore{
		{Category: models.SkillCategoryInfra, Score: 60},
		{Category: models.SkillCategoryBackend, Score: 90},
	})

	backendIndex := strings.Index(hints, "バックエンド（スコア90）")
	infraIndex := strings.Index(hints, "インフラ（スコア60）")
	if backendIndex < 0 || infraIndex < 0 {
		t.Fatalf("expected skill hints are missing: %s", hints)
	}
	if backendIndex >= infraIndex {
		t.Fatalf("skill hints are not ordered by score descending: %s", hints)
	}
}
