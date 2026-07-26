package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"log"
	"math"
	"strings"
)

type AnalysisScores struct {
	JobScore      float64 `json:"job_score"`
	InterestScore float64 `json:"interest_score"`
	AptitudeScore float64 `json:"aptitude_score"`
	FutureScore   float64 `json:"future_score"`
	FinalScore    float64 `json:"final_score"`
}

type AnalysisProgress struct {
	Job      float64 `json:"job"`
	Interest float64 `json:"interest"`
	Aptitude float64 `json:"aptitude"`
	Future   float64 `json:"future"`
	Overall  float64 `json:"overall"`
}

type AxisScore struct {
	Axis  string  `json:"axis"`
	Score float64 `json:"score"`
}

type CategoryRecommendation struct {
	Category string `json:"category"`
	Score    int    `json:"score"`
}

type CompanyRecommendation struct {
	ID    uint    `json:"id"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type AnalysisRecommendations struct {
	TopCategories []CategoryRecommendation `json:"top_categories"`
	TopCompanies  []CompanyRecommendation  `json:"top_companies"`
}

type JobSuitabilityRole struct {
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

type LLMStructuredSummary struct {
	Strengths               []string `json:"strengths"`
	Concerns                []string `json:"concerns"`
	RecommendedWorkingStyle string   `json:"recommended_working_style"`
}

type AnalysisSummary struct {
	Scores                AnalysisScores          `json:"scores"`
	Progress              AnalysisProgress        `json:"progress"`
	AptitudeAxes          []AxisScore             `json:"aptitude_axes"`
	FutureSignals         []string                `json:"future_signals,omitempty"`
	Recommendations       AnalysisRecommendations `json:"recommendations"`
	JobSuitabilityComment string                  `json:"job_suitability_comment,omitempty"`
	SuggestedRoles        []JobSuitabilityRole    `json:"suggested_roles,omitempty"`
	ScoreComment          string                  `json:"score_comment,omitempty"`
	// LLM による要約（生テキスト）
	LLMRawSummary string `json:"llm_raw_summary,omitempty"`
	// LLM による構造化サマリ（可能な場合）
	LLMStructured *LLMStructuredSummary `json:"llm_structured_summary,omitempty"`
}

type FutureAnalyzer interface {
	Score(messages []models.ChatMessage) (float64, []string)
}

type RuleBasedFutureAnalyzer struct {
	keywords  []string
	threshold int
}

func NewRuleBasedFutureAnalyzer() *RuleBasedFutureAnalyzer {
	return &RuleBasedFutureAnalyzer{
		keywords: []string{
			"成長", "挑戦", "将来", "キャリア", "スキル", "学び", "伸ば", "向上",
			"リーダー", "マネジメント", "起業", "海外", "グローバル", "研究", "開発",
		},
		threshold: 5,
	}
}

func (a *RuleBasedFutureAnalyzer) Score(messages []models.ChatMessage) (float64, []string) {
	if len(messages) == 0 {
		return 0, nil
	}

	found := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		text := strings.ToLower(msg.Content)
		for _, keyword := range a.keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				found[keyword] = true
			}
		}
	}

	if len(found) == 0 {
		return 0, nil
	}

	matched := make([]string, 0, len(found))
	for keyword := range found {
		matched = append(matched, keyword)
	}

	score := math.Min(1, float64(len(found))/float64(a.threshold))
	return score, matched
}

type AnalysisScoringService struct {
	userWeightScoreRepo     repository.UserWeightScoreRepository
	chatMessageRepo         repository.ChatMessageRepository
	progressRepo            repository.UserAnalysisProgressRepository
	conversationContextRepo repository.ConversationContextRepository
	userEmbeddingRepo       repository.UserEmbeddingRepository
	jobEmbeddingRepo        repository.JobCategoryEmbeddingRepository
	matchRepo               repository.UserCompanyMatchRepository
	futureAnalyzer          FutureAnalyzer
	aiClient                *openai.Client
}

func NewAnalysisScoringService(
	userWeightScoreRepo repository.UserWeightScoreRepository,
	chatMessageRepo repository.ChatMessageRepository,
	progressRepo repository.UserAnalysisProgressRepository,
	conversationContextRepo repository.ConversationContextRepository,
	userEmbeddingRepo repository.UserEmbeddingRepository,
	jobEmbeddingRepo repository.JobCategoryEmbeddingRepository,
	matchRepo repository.UserCompanyMatchRepository,
	aiClient *openai.Client,
	futureAnalyzer FutureAnalyzer,
) *AnalysisScoringService {
	if futureAnalyzer == nil {
		futureAnalyzer = NewRuleBasedFutureAnalyzer()
	}
	return &AnalysisScoringService{
		userWeightScoreRepo:     userWeightScoreRepo,
		chatMessageRepo:         chatMessageRepo,
		progressRepo:            progressRepo,
		conversationContextRepo: conversationContextRepo,
		userEmbeddingRepo:       userEmbeddingRepo,
		jobEmbeddingRepo:        jobEmbeddingRepo,
		matchRepo:               matchRepo,
		futureAnalyzer:          futureAnalyzer,
		aiClient:                aiClient,
	}
}

func (s *AnalysisScoringService) BuildAnalysisSummary(ctx context.Context, userID uint, sessionID string) (*AnalysisSummary, error) {
	_ = ctx
	jobScore, err := s.calculateJobScore(userID, sessionID)
	if err != nil {
		return nil, err
	}
	interestScore := s.calculateInterestScore(userID, sessionID)
	aptitudeScore, axes := s.calculateAptitudeScore(userID, sessionID)
	futureScore, signals := s.calculateFutureScore(sessionID)

	finalScore := (jobScore * 0.4) + (interestScore * 0.25) + (aptitudeScore * 0.2) + (futureScore * 0.15)

	progress := s.calculateProgress(userID, sessionID)
	recommendations := s.buildRecommendations(userID, sessionID)

	scores, _ := s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
	jobSuitabilityComment, suggestedRoles := buildJobSuitabilityComment(scores)

	allScores := AnalysisScores{
		JobScore:      jobScore,
		InterestScore: interestScore,
		AptitudeScore: aptitudeScore,
		FutureScore:   futureScore,
		FinalScore:    finalScore,
	}
	scoreComment := buildScoreComment(allScores)

	summary := &AnalysisSummary{
		Scores:                allScores,
		Progress:              progress,
		AptitudeAxes:          axes,
		FutureSignals:         signals,
		Recommendations:       recommendations,
		JobSuitabilityComment: jobSuitabilityComment,
		SuggestedRoles:        suggestedRoles,
		ScoreComment:          scoreComment,
	}

	// LLMによる簡易サマリ（利用可能な場合）
	if s.aiClient != nil && s.chatMessageRepo != nil && s.conversationContextRepo != nil {
		// 直近のユーザーメッセージを収集
		msgs, err := s.chatMessageRepo.FindRecentBySessionID(sessionID, 30)
		if err == nil {
			// プロンプト構築：スコア要約 + 最近メッセージ
			contextBytes, _ := json.Marshal(map[string]any{
				"scores":          summary.Scores,
				"progress":        summary.Progress,
				"recommendations": summary.Recommendations,
			})
			var lastUserTexts []string
			for _, m := range msgs {
				if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
					lastUserTexts = append(lastUserTexts, m.Content)
				}
			}
			userContext := strings.Join(lastUserTexts, "\n---\n")

			systemPrompt := `あなたは採用支援の専門家です。以下の情報を元に、JSON形式で要約を出力してください。
出力フォーマット: {"strengths": ["..."], "concerns": ["..."], "recommended_working_style": "..."}
日本語で簡潔に記述してください。`
			userPrompt := "解析メタ情報: " + string(contextBytes) + "\n\n直近のユーザーメッセージ:\n" + userContext

			raw, err := s.aiClient.ChatCompletionJSON(context.Background(), systemPrompt, userPrompt, 0.2, 400)
			if err == nil && strings.TrimSpace(raw) != "" {
				// パースを試みる
				var parsed LLMStructuredSummary
				if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
					summary.LLMStructured = &parsed
					summary.LLMRawSummary = raw
					// 永続化
					if err := s.conversationContextRepo.SetSessionSummary(userID, sessionID, raw); err != nil {
						log.Printf("failed to persist llm summary: %v", err)
					}
				} else {
					// JSONでない場合は生テキストとして保存
					summary.LLMRawSummary = raw
					if err := s.conversationContextRepo.SetSessionSummary(userID, sessionID, raw); err != nil {
						log.Printf("failed to persist llm summary: %v", err)
					}
				}
			} else if err != nil {
				log.Printf("LLM summary generation failed: %v", err)
			}
		}
	}

	return summary, nil
}
