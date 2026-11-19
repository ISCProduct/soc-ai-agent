package chat

import (
	"Backend/internal/models"
	"Backend/internal/services/judgment"
)

// 職種の回答は、1回の回答で job_analysis フェーズを一括で完了にする（completeJobAnalysisPhase）。
// そのフェーズの valid_answers は必要数（4）まで引き上げられるので、実際の回答数としては1回に数え直す。
const jobAnalysisPhaseName = "job_analysis"

// defaultTargetQuestions は、応答の TotalQuestions（目安の質問数）に入れる値。
// 質問数は固定しない（判定できたら完了）ので、完了の条件ではなく、旧クライアント向けの表示用の目安。
// 画面の進捗は JudgmentPercent を使う。
const defaultTargetQuestions = 15

// validAnswersFromProgress は、フェーズごとの進捗から、有効に答えた質問数を数える。
// 最後のフェーズの質問数を超えて続けた回答も、最後のフェーズの valid_answers に積まれるので、そのまま足せる。
func validAnswersFromProgress(progresses []models.UserAnalysisProgress, phaseByID map[uint]*models.AnalysisPhase) int {
	total := 0
	for _, p := range progresses {
		n := p.ValidAnswers
		phase := p.Phase
		if phase == nil {
			phase = phaseByID[p.PhaseID]
		}
		if phase != nil && phase.PhaseName == jobAnalysisPhaseName && n > 1 {
			n = 1
		}
		total += n
	}
	return total
}

// scoreMapOf は、カテゴリごとのスコア（0〜100）。
func scoreMapOf(scores []models.UserWeightScore) map[string]int {
	m := make(map[string]int, len(scores))
	for _, sc := range scores {
		m[sc.WeightCategory] = sc.Score
	}
	return m
}

// judgmentFor は、保存済みの状態から、判定の進み具合を計算する。
// 質問数は固定しない。判定できたら（15 問に満たなくても）完了、判定できなければ（15 問を超えても）続ける。
func (s *ChatService) judgmentFor(userID uint, sessionID string) judgment.Result {
	if s.progressRepo == nil || s.phaseRepo == nil || s.userWeightScoreRepo == nil {
		return judgment.Result{}
	}
	progresses, err := s.progressRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return judgment.Result{}
	}
	phases, err := s.phaseRepo.FindAll()
	if err != nil {
		return judgment.Result{}
	}
	phaseByID := make(map[uint]*models.AnalysisPhase, len(phases))
	for i := range phases {
		phaseByID[phases[i].ID] = &phases[i]
	}
	scores, err := s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return judgment.Result{}
	}
	return judgment.Evaluate(judgment.Input{
		ValidAnswers: validAnswersFromProgress(progresses, phaseByID),
		Scores:       scoreMapOf(scores),
	})
}
