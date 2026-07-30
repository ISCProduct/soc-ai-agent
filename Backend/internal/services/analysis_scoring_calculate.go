package services

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
