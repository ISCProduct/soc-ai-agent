package services

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
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

type AnalysisSummary struct {
	Scores                AnalysisScores          `json:"scores"`
	Progress              AnalysisProgress        `json:"progress"`
	AptitudeAxes          []AxisScore             `json:"aptitude_axes"`
	FutureSignals         []string                `json:"future_signals,omitempty"`
	Recommendations       AnalysisRecommendations `json:"recommendations"`
	JobSuitabilityComment string                  `json:"job_suitability_comment,omitempty"`
	SuggestedRoles        []JobSuitabilityRole    `json:"suggested_roles,omitempty"`
	ScoreComment          string                  `json:"score_comment,omitempty"`
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
}

func NewAnalysisScoringService(
	userWeightScoreRepo repository.UserWeightScoreRepository,
	chatMessageRepo repository.ChatMessageRepository,
	progressRepo repository.UserAnalysisProgressRepository,
	conversationContextRepo repository.ConversationContextRepository,
	userEmbeddingRepo repository.UserEmbeddingRepository,
	jobEmbeddingRepo repository.JobCategoryEmbeddingRepository,
	matchRepo repository.UserCompanyMatchRepository,
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

	return &AnalysisSummary{
		Scores:                allScores,
		Progress:              progress,
		AptitudeAxes:          axes,
		FutureSignals:         signals,
		Recommendations:       recommendations,
		JobSuitabilityComment: jobSuitabilityComment,
		SuggestedRoles:        suggestedRoles,
		ScoreComment:          scoreComment,
	}, nil
}

func (s *AnalysisScoringService) calculateJobScore(userID uint, sessionID string) (float64, error) {
	if s.userEmbeddingRepo == nil || s.jobEmbeddingRepo == nil || s.conversationContextRepo == nil {
		return s.phaseCompletionScore("job_analysis", userID, sessionID), nil
	}

	jobCategoryID, err := s.conversationContextRepo.GetJobCategoryID(sessionID)
	if err != nil || jobCategoryID == 0 {
		return s.phaseCompletionScore("job_analysis", userID, sessionID), nil
	}

	userEmbedding, err := s.userEmbeddingRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return s.phaseCompletionScore("job_analysis", userID, sessionID), nil
	}
	jobEmbedding, err := s.jobEmbeddingRepo.FindByJobCategoryID(jobCategoryID)
	if err != nil {
		return s.phaseCompletionScore("job_analysis", userID, sessionID), nil
	}

	userVector, err := parseEmbedding(userEmbedding.Embedding)
	if err != nil {
		return 0, err
	}
	jobVector, err := parseEmbedding(jobEmbedding.Embedding)
	if err != nil {
		return 0, err
	}

	return cosineSimilarity(userVector, jobVector), nil
}

func (s *AnalysisScoringService) phaseCompletionScore(phaseName string, userID uint, sessionID string) float64 {
	if s.progressRepo == nil {
		return 0
	}
	records, err := s.progressRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return 0
	}
	for _, record := range records {
		if record.Phase != nil && record.Phase.PhaseName == phaseName {
			return clamp01(record.CompletionScore / 100.0)
		}
	}
	return 0
}

func (s *AnalysisScoringService) calculateInterestScore(userID uint, sessionID string) float64 {
	if s.matchRepo == nil {
		return s.phaseCompletionScore("interest_analysis", userID, sessionID)
	}

	stats, err := s.matchRepo.GetMatchStatistics(userID, sessionID)
	if err != nil {
		return s.phaseCompletionScore("interest_analysis", userID, sessionID)
	}

	totalMatches, _ := stats["total_matches"].(int64)
	viewedCount, _ := stats["viewed_count"].(int64)
	favoritedCount, _ := stats["favorited_count"].(int64)
	appliedCount, _ := stats["applied_count"].(int64)

	phaseScore := s.phaseCompletionScore("interest_analysis", userID, sessionID)

	// マッチ企業との操作実績がなければチャット回答の達成度をそのまま返す
	if viewedCount+appliedCount+favoritedCount == 0 {
		return phaseScore
	}

	raw := (float64(viewedCount) * 0.7) + (float64(appliedCount) * 1.0) + (float64(favoritedCount) * 1.2)
	max := float64(totalMatches) * (0.7 + 1.0 + 1.2)
	if max <= 0 {
		return phaseScore
	}
	// チャット達成度60% + マッチ操作40% でブレンド
	return clamp01(phaseScore*0.6 + clamp01(raw/max)*0.4)
}

func (s *AnalysisScoringService) calculateAptitudeScore(userID uint, sessionID string) (float64, []AxisScore) {
	scores, err := s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return 0, nil
	}

	scoreMap := make(map[string]float64, len(scores))
	for _, score := range scores {
		scoreMap[score.WeightCategory] = float64(score.Score)
	}

	axisCategories := map[string][]string{
		"論理性": {"技術志向", "細部志向"},
		"協調性": {"チームワーク志向", "コミュニケーション力"},
		"自律性": {"成長志向", "チャレンジ志向", "リーダーシップ志向"},
	}

	var axisScores []AxisScore
	var sum float64
	var count float64

	for axis, categories := range axisCategories {
		axisScore := averageCategoryScore(scoreMap, categories)
		axisScores = append(axisScores, AxisScore{
			Axis:  axis,
			Score: axisScore,
		})
		sum += axisScore
		count++
	}

	if count == 0 {
		return 0, axisScores
	}
	return clamp01(sum / count), axisScores
}

func (s *AnalysisScoringService) calculateFutureScore(sessionID string) (float64, []string) {
	if s.chatMessageRepo == nil || s.futureAnalyzer == nil {
		return 0, nil
	}
	messages, err := s.chatMessageRepo.FindBySessionID(sessionID)
	if err != nil {
		return 0, nil
	}
	score, signals := s.futureAnalyzer.Score(messages)
	return clamp01(score), signals
}

func (s *AnalysisScoringService) calculateProgress(userID uint, sessionID string) AnalysisProgress {
	progress := AnalysisProgress{}
	if s.progressRepo == nil {
		return progress
	}

	records, err := s.progressRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return progress
	}

	for _, record := range records {
		if record.Phase == nil {
			continue
		}
		score := clamp01(record.CompletionScore / 100.0)
		switch record.Phase.PhaseName {
		case "job_analysis":
			progress.Job = score
		case "interest_analysis":
			progress.Interest = score
		case "aptitude_analysis":
			progress.Aptitude = score
		case "future_analysis":
			progress.Future = score
		}
	}

	progress.Overall = clamp01((progress.Job + progress.Interest + progress.Aptitude + progress.Future) / 4.0)
	return progress
}

func (s *AnalysisScoringService) buildRecommendations(userID uint, sessionID string) AnalysisRecommendations {
	recommendations := AnalysisRecommendations{}

	topCategories, err := s.userWeightScoreRepo.FindTopCategories(userID, sessionID, 3)
	if err == nil {
		for _, score := range topCategories {
			recommendations.TopCategories = append(recommendations.TopCategories, CategoryRecommendation{
				Category: score.WeightCategory,
				Score:    score.Score,
			})
		}
	}

	if s.matchRepo == nil {
		return recommendations
	}

	topMatches, err := s.matchRepo.FindTopMatchesByUserAndSession(userID, sessionID, 3)
	if err != nil {
		return recommendations
	}

	for _, match := range topMatches {
		if match.Company == nil || match.Company.ID == 0 {
			continue
		}
		recommendations.TopCompanies = append(recommendations.TopCompanies, CompanyRecommendation{
			ID:    match.Company.ID,
			Name:  match.Company.Name,
			Score: match.MatchScore,
		})
	}
	return recommendations
}

func buildJobSuitabilityComment(scores []entity.UserWeightScore) (string, []JobSuitabilityRole) {
	if len(scores) == 0 {
		return "", nil
	}

	scoreMap := make(map[string]int)
	for _, s := range scores {
		scoreMap[s.WeightCategory] = s.Score
	}

	type roleCandidate struct {
		title  string
		reason string
		weight int
	}

	roleMappings := []struct {
		categories []string
		role       roleCandidate
	}{
		{
			categories: []string{"リーダーシップ志向", "成長志向", "チャレンジ志向"},
			role: roleCandidate{
				title:  "プロジェクトマネージャー / テックリード",
				reason: "リーダーシップと成長志向を活かし、チームを率いながら技術的課題を解決する役割に向いています",
			},
		},
		{
			categories: []string{"リーダーシップ志向", "技術志向"},
			role: roleCandidate{
				title:  "エンジニアリングマネージャー",
				reason: "技術的な深い理解とリーダーシップを組み合わせ、エンジニアチームを牽引できます",
			},
		},
		{
			categories: []string{"チームワーク志向", "コミュニケーション力"},
			role: roleCandidate{
				title:  "ITコンサルタント / スクラムマスター",
				reason: "コミュニケーション力と協調性を活かし、チーム横断的な課題解決や調整役として活躍できます",
			},
		},
		{
			categories: []string{"技術志向", "細部志向"},
			role: roleCandidate{
				title:  "バックエンド / インフラエンジニア",
				reason: "技術への探求心と細部へのこだわりを活かした、品質重視の技術職に適しています",
			},
		},
		{
			categories: []string{"成長志向", "チャレンジ志向"},
			role: roleCandidate{
				title:  "スタートアップ / 新規事業エンジニア",
				reason: "変化への適応力と挑戦意欲を活かし、スピード感ある環境で大きな裁量を持って働けます",
			},
		},
	}

	type scoredRole struct {
		role  roleCandidate
		total int
	}
	var candidates []scoredRole
	for _, mapping := range roleMappings {
		total := 0
		matched := 0
		for _, cat := range mapping.categories {
			if v, ok := scoreMap[cat]; ok && v > 0 {
				total += v
				matched++
			}
		}
		if matched >= 1 {
			candidates = append(candidates, scoredRole{role: mapping.role, total: total})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].total > candidates[j].total
	})

	maxRoles := 3
	if len(candidates) < maxRoles {
		maxRoles = len(candidates)
	}
	if maxRoles == 0 {
		return "", nil
	}

	var roles []JobSuitabilityRole
	for i := 0; i < maxRoles; i++ {
		roles = append(roles, JobSuitabilityRole{
			Title:  candidates[i].role.title,
			Reason: candidates[i].role.reason,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	var strengthParts []string
	for i := 0; i < len(scores) && i < 3; i++ {
		if scores[i].Score > 0 {
			strengthParts = append(strengthParts, strings.TrimSuffix(scores[i].WeightCategory, "志向"))
		}
	}

	strengthText := "複数の強み"
	if len(strengthParts) > 0 {
		strengthText = strings.Join(strengthParts, "・")
	}

	comment := fmt.Sprintf(
		"分析結果から、あなたには%sという強みがあります。これらの特性から、以下の職種が特に向いていると考えられます。",
		strengthText,
	)

	return comment, roles
}

func buildScoreComment(scores AnalysisScores) string {
	var parts []string

	jobPct := scores.JobScore * 100
	switch {
	case jobPct >= 80:
		parts = append(parts, "志望職種への適性が高い")
	case jobPct >= 50:
		parts = append(parts, "志望職種への適性が一定水準ある")
	case jobPct > 0:
		parts = append(parts, "志望職種への理解をさらに深めると良い")
	}

	interestPct := scores.InterestScore * 100
	switch {
	case interestPct >= 80:
		parts = append(parts, "企業への関心・意欲が非常に高い")
	case interestPct >= 50:
		parts = append(parts, "企業への関心・意欲が示されている")
	case interestPct > 0:
		parts = append(parts, "企業への関心をさらに深めると良い")
	}

	aptitudePct := scores.AptitudeScore * 100
	switch {
	case aptitudePct >= 80:
		parts = append(parts, "多面的な適性が高く評価されている")
	case aptitudePct >= 50:
		parts = append(parts, "複数の適性が確認されている")
	case aptitudePct > 0:
		parts = append(parts, "適性をさらに伸ばす余地がある")
	}

	futurePct := scores.FutureScore * 100
	switch {
	case futurePct >= 80:
		parts = append(parts, "将来への展望・成長意欲が強く感じられる")
	case futurePct >= 50:
		parts = append(parts, "将来志向が見られる")
	case futurePct > 0:
		parts = append(parts, "将来ビジョンをより明確にするとよい")
	}

	if len(parts) == 0 {
		return "チャット診断を完了させることで、より詳細な分析コメントが表示されます。"
	}

	comment := strings.Join(parts, "、") + "。"

	finalPct := scores.FinalScore * 100
	switch {
	case finalPct >= 80:
		comment += "総合的に非常に優れたプロフィールです。自信を持って就活に臨んでください。"
	case finalPct >= 60:
		comment += "総合的にバランスの取れたプロフィールです。強みをアピールしながら就活を進めましょう。"
	case finalPct >= 40:
		comment += "いくつかの強みが見られます。診断をさらに深めることでより精度の高いマッチングが可能です。"
	default:
		comment += "診断をさらに進めることで、あなたにぴったりの企業が見つかります。"
	}

	return comment
}

func parseEmbedding(raw string) ([]float64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var vec []float64
	if err := json.Unmarshal([]byte(raw), &vec); err != nil {
		return nil, err
	}
	return vec, nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return clamp01(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func averageCategoryScore(scoreMap map[string]float64, categories []string) float64 {
	if len(categories) == 0 {
		return 0
	}
	var sum float64
	var count float64
	for _, category := range categories {
		score, ok := scoreMap[category]
		if !ok {
			continue
		}
		sum += clamp01(score / 100.0)
		count++
	}
	if count == 0 {
		return 0
	}
	return clamp01(sum / count)
}
