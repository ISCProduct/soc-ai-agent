package services

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/prompts"
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
)

type MatchingService struct {
	userWeightScoreRepo repository.UserWeightScoreRepository
	companyRepo         repository.CompanyRepository
	matchRepo           repository.UserCompanyMatchRepository
	aiClient            *openai.Client
}

func NewMatchingService(
	userWeightScoreRepo repository.UserWeightScoreRepository,
	companyRepo repository.CompanyRepository,
	matchRepo repository.UserCompanyMatchRepository,
	aiClient *openai.Client,
) *MatchingService {
	return &MatchingService{
		userWeightScoreRepo: userWeightScoreRepo,
		companyRepo:         companyRepo,
		matchRepo:           matchRepo,
		aiClient:            aiClient,
	}
}

// CalculateMatching ユーザーと企業のマッチングを計算
func (s *MatchingService) CalculateMatching(ctx context.Context, userID uint, sessionID string) error {
	log.Printf("[CalculateMatching] Starting matching calculation for user %d, session %s\n", userID, sessionID)

	// 1. ユーザーのスコアを取得
	userScores, err := s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get user scores: %w", err)
	}

	if len(userScores) == 0 {
		log.Printf("[CalculateMatching] No user scores found for user %d, session %s\n", userID, sessionID)
		return nil
	}

	// スコアをマップに変換
	scoreMap := make(map[string]float64)
	for _, score := range userScores {
		scoreMap[score.WeightCategory] = float64(score.Score)
	}

	log.Printf("[CalculateMatching] User scores: %v\n", scoreMap)

	// 2. 全企業のプロファイルを取得
	companies, err := s.companyRepo.FindAllActive(10000, 0)
	if err != nil {
		return fmt.Errorf("failed to get companies: %w", err)
	}

	log.Printf("[CalculateMatching] Found %d active companies\n", len(companies))

	// 3. 各企業とのマッチングを計算
	matchCount := 0
	for _, company := range companies {
		// 企業のweightプロファイルを取得
		profile, err := s.companyRepo.GetWeightProfile(company.ID, nil)
		if err != nil {
			log.Printf("[CalculateMatching] Warning: No profile for company %d: %v\n", company.ID, err)
			continue
		}

		// マッチングスコアを計算
		match := s.calculateMatchScore(scoreMap, profile)
		match.UserID = userID
		match.SessionID = sessionID
		match.CompanyID = company.ID
		match.Company = mapper.CompanyToEntity(&company)

		reason, err := s.GenerateMatchReason(ctx, match, userScores)
		if err != nil {
			log.Printf("[CalculateMatching] Warning: Failed to generate AI reason for company %d: %v\n", company.ID, err)
			reason = BuildMatchReason(match, userScores)
		}
		match.MatchReason = reason

		// マッチング結果を保存
		if err := s.matchRepo.CreateOrUpdate(match); err != nil {
			log.Printf("[CalculateMatching] Warning: Failed to save match for company %d: %v\n", company.ID, err)
			continue
		}
		matchCount++
	}

	log.Printf("[CalculateMatching] Completed: %d matches created for user %d, session %s\n", matchCount, userID, sessionID)
	return nil
}

// calculateMatchScore ユーザースコアと企業プロファイルからマッチングスコアを計算
func (s *MatchingService) calculateMatchScore(
	userScores map[string]float64,
	companyProfile *models.CompanyWeightProfile,
) *entity.UserCompanyMatch {
	match := &entity.UserCompanyMatch{}
	evaluatedCount := 0
	totalScore := 0.0

	// 各カテゴリのマッチ度を計算（0-100のスケールで）
	// マッチ度 = 100 - |ユーザースコア - 企業重視度|
	match.TechnicalMatch, evaluatedCount, totalScore = scoredMatch(userScores, "技術志向", float64(companyProfile.TechnicalOrientation), evaluatedCount, totalScore)
	match.TeamworkMatch, evaluatedCount, totalScore = scoredMatch(userScores, "チームワーク志向", float64(companyProfile.TeamworkOrientation), evaluatedCount, totalScore)
	match.LeadershipMatch, evaluatedCount, totalScore = scoredMatch(userScores, "リーダーシップ志向", float64(companyProfile.LeadershipOrientation), evaluatedCount, totalScore)
	match.CreativityMatch, evaluatedCount, totalScore = scoredMatch(userScores, "創造性志向", float64(companyProfile.CreativityOrientation), evaluatedCount, totalScore)
	match.StabilityMatch, evaluatedCount, totalScore = scoredMatch(userScores, "安定志向", float64(companyProfile.StabilityOrientation), evaluatedCount, totalScore)
	match.GrowthMatch, evaluatedCount, totalScore = scoredMatch(userScores, "成長志向", float64(companyProfile.GrowthOrientation), evaluatedCount, totalScore)
	match.WorkLifeMatch, evaluatedCount, totalScore = scoredMatch(userScores, "ワークライフバランス", float64(companyProfile.WorkLifeBalance), evaluatedCount, totalScore)
	match.ChallengeMatch, evaluatedCount, totalScore = scoredMatch(userScores, "チャレンジ志向", float64(companyProfile.ChallengeSeeking), evaluatedCount, totalScore)
	match.DetailMatch, evaluatedCount, totalScore = scoredMatch(userScores, "細部志向", float64(companyProfile.DetailOrientation), evaluatedCount, totalScore)
	match.CommunicationMatch, evaluatedCount, totalScore = scoredMatch(userScores, "コミュニケーション力", float64(companyProfile.CommunicationSkill), evaluatedCount, totalScore)

	// 総合マッチ度を計算（全カテゴリの平均）
	if evaluatedCount > 0 {
		match.MatchScore = totalScore / float64(evaluatedCount)
	} else {
		match.MatchScore = 0
	}

	return match
}

func scoredMatch(userScores map[string]float64, category string, companyWeight float64, evaluatedCount int, totalScore float64) (float64, int, float64) {
	userScore, ok := userScores[category]
	if !ok {
		return 0, evaluatedCount, totalScore
	}
	matchScore := calculateCategoryMatch(userScore, companyWeight)
	return matchScore, evaluatedCount + 1, totalScore + matchScore
}

// calculateCategoryMatch カテゴリごとのマッチ度を計算
// ユーザースコアと企業重視度の差が小さいほど高スコア
func calculateCategoryMatch(userScore, companyWeight float64) float64 {
	diff := math.Abs(userScore - companyWeight)
	return math.Max(0, 100.0-diff)
}

// GetTopMatches マッチング度の高い企業を取得
func (s *MatchingService) GetTopMatches(ctx context.Context, userID uint, sessionID string, limit int) ([]*entity.UserCompanyMatch, error) {
	return s.matchRepo.FindTopMatchesByUserAndSession(userID, sessionID, limit)
}

// ToggleFavorite お気に入りをトグル
func (s *MatchingService) ToggleFavorite(matchID uint) error {
	return s.matchRepo.ToggleFavorite(matchID)
}

type MatchingDiagnostics struct {
	UserScoreCount     int64 `json:"user_score_count"`
	ActiveCompanyCount int64 `json:"active_company_count"`
	WeightProfileCount int64 `json:"weight_profile_count"`
}

func (s *MatchingService) GetDiagnostics(userID uint, sessionID string) (*MatchingDiagnostics, error) {
	userScoreCount, err := s.userWeightScoreRepo.CountByUserAndSession(userID, sessionID)
	if err != nil {
		return nil, err
	}
	activeCompanyCount, err := s.companyRepo.CountActive()
	if err != nil {
		return nil, err
	}
	weightProfileCount, err := s.companyRepo.CountWeightProfiles()
	if err != nil {
		return nil, err
	}
	return &MatchingDiagnostics{
		UserScoreCount:     userScoreCount,
		ActiveCompanyCount: activeCompanyCount,
		WeightProfileCount: weightProfileCount,
	}, nil
}

// GenerateMatchReason AIを使ってマッチング理由を生成（オプション）
func (s *MatchingService) GenerateMatchReason(ctx context.Context, match *entity.UserCompanyMatch, userScores []entity.UserWeightScore) (string, error) {
	if match == nil {
		return "", fmt.Errorf("match is nil")
	}
	if match.Company == nil {
		return "", fmt.Errorf("company is nil")
	}
	if s.aiClient == nil {
		return BuildMatchReason(match, userScores), nil
	}

	userPrompt := buildMatchingReasonUserPrompt(match, userScores)
	reason, err := s.aiClient.ResponsesWithTemperature(
		ctx,
		prompts.MatchingReasonSystemPrompt,
		userPrompt,
		0.4,
	)
	if err != nil {
		return "", err
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("empty matching reason response")
	}
	return reason, nil
}

func buildMatchingReasonUserPrompt(match *entity.UserCompanyMatch, userScores []entity.UserWeightScore) string {
	company := match.Company
	return fmt.Sprintf(
		prompts.MatchingReasonUserPromptTemplate,
		prompts.MatchingReasonPromptVersion,
		company.Name,
		fallbackValue(company.Industry, "未設定"),
		fallbackValue(company.MainBusiness, "未設定"),
		fallbackValue(company.Culture, "未設定"),
		fallbackValue(company.WorkStyle, "未設定"),
		fallbackValue(company.DevelopmentStyle, "未設定"),
		fallbackValue(company.TechStack, "未設定"),
		formatTopUserScores(userScores),
		formatTopMatchAxes(match),
		match.MatchScore,
	)
}

func formatTopUserScores(scores []entity.UserWeightScore) string {
	if len(scores) == 0 {
		return "データなし"
	}

	sorted := append([]entity.UserWeightScore(nil), scores...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	parts := make([]string, 0, 3)
	for i := 0; i < len(sorted) && i < 3; i++ {
		if sorted[i].Score <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", sorted[i].WeightCategory, sorted[i].Score))
	}
	if len(parts) == 0 {
		return "有効なスコアなし"
	}
	return strings.Join(parts, ", ")
}

func formatTopMatchAxes(match *entity.UserCompanyMatch) string {
	axes := []struct {
		name  string
		score float64
	}{
		{name: "技術志向", score: match.TechnicalMatch},
		{name: "チームワーク志向", score: match.TeamworkMatch},
		{name: "リーダーシップ志向", score: match.LeadershipMatch},
		{name: "創造性志向", score: match.CreativityMatch},
		{name: "安定志向", score: match.StabilityMatch},
		{name: "成長志向", score: match.GrowthMatch},
		{name: "ワークライフバランス", score: match.WorkLifeMatch},
		{name: "チャレンジ志向", score: match.ChallengeMatch},
		{name: "細部志向", score: match.DetailMatch},
		{name: "コミュニケーション力", score: match.CommunicationMatch},
	}
	sort.Slice(axes, func(i, j int) bool {
		return axes[i].score > axes[j].score
	})

	parts := make([]string, 0, 3)
	for i := 0; i < len(axes) && i < 3; i++ {
		parts = append(parts, fmt.Sprintf("%s:%.1f", axes[i].name, axes[i].score))
	}
	return strings.Join(parts, ", ")
}

func fallbackValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
