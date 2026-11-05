package admin

import (
	"context"
	"errors"
	"fmt"

	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/chat"
	"Backend/internal/services/costs"
)

const userScoreRecalculationBatchSize = 10

type UserScoreRecalculationService struct {
	scores *repositories.UserWeightScoreRepository
	jobs   *repositories.UserScoreRecalculationRepository
	chat   *chat.ChatService
}

func NewUserScoreRecalculationService(
	scores *repositories.UserWeightScoreRepository,
	jobs *repositories.UserScoreRecalculationRepository,
	chatService *chat.ChatService,
) *UserScoreRecalculationService {
	return &UserScoreRecalculationService{scores: scores, jobs: jobs, chat: chatService}
}

type UserScoreRecalculationPreview struct {
	CalculationVersion  string  `json:"calculation_version"`
	EligibleSessions    uint    `json:"eligible_sessions"`
	UnsupportedSessions uint    `json:"unsupported_sessions"`
	EstimatedLLMCalls   uint    `json:"estimated_llm_calls"`
	EstimatedCostUSD    float64 `json:"estimated_cost_usd"`
	CostEstimateNote    string  `json:"cost_estimate_note"`
}

func (s *UserScoreRecalculationService) Preview() (*UserScoreRecalculationPreview, error) {
	stats, err := s.scores.RecalculationStats(models.CurrentUserScoreCalculationVersion)
	if err != nil {
		return nil, fmt.Errorf("スコア再計算対象の取得に失敗: %w", err)
	}
	model, billable := s.chat.RecalculationModel()
	estimatedCost := 0.0
	if billable && stats.EligibleAnswers > 0 {
		// すべての回答に短文・境界値評価が発生した場合として保守的に見積もる。
		estimatedCost = costs.EstimateCostUSD(model, 2048, 256) * float64(stats.EligibleAnswers)
	}
	return &UserScoreRecalculationPreview{
		CalculationVersion:  models.CurrentUserScoreCalculationVersion,
		EligibleSessions:    stats.EligibleSessions,
		UnsupportedSessions: stats.UnsupportedSessions,
		EstimatedLLMCalls:   stats.EligibleAnswers,
		EstimatedCostUSD:    estimatedCost,
		CostEstimateNote:    "回答1件あたり入力2048・出力256トークン、全回答でLLM評価が発生する前提の概算です。実際の呼び出し数と費用はこれを下回る場合があります。",
	}, nil
}

func (s *UserScoreRecalculationService) Start() (*models.UserScoreRecalculationJob, error) {
	preview, err := s.Preview()
	if err != nil {
		return nil, err
	}
	stats, err := s.scores.RecalculationStats(models.CurrentUserScoreCalculationVersion)
	if err != nil {
		return nil, fmt.Errorf("スコア再計算対象の取得に失敗: %w", err)
	}
	status := models.UserScoreRecalculationJobRunning
	if preview.EligibleSessions == 0 {
		status = models.UserScoreRecalculationJobCompleted
	}
	job := &models.UserScoreRecalculationJob{
		CalculationVersion: models.CurrentUserScoreCalculationVersion,
		Status:             status,
		MaxScoreID:         stats.MaxScoreID,
		TotalSessions:      preview.EligibleSessions,
		EstimatedCostUSD:   preview.EstimatedCostUSD,
	}
	if err := s.jobs.Create(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *UserScoreRecalculationService) GetJob(id uint) (*models.UserScoreRecalculationJob, error) {
	return s.jobs.FindByID(id)
}

func (s *UserScoreRecalculationService) Pause(id uint) (*models.UserScoreRecalculationJob, error) {
	job, err := s.jobs.FindByID(id)
	if err != nil {
		return nil, err
	}
	if job.Status != models.UserScoreRecalculationJobRunning {
		return nil, fmt.Errorf("実行中のジョブではありません")
	}
	if err := s.jobs.UpdateStatus(id, models.UserScoreRecalculationJobPaused); err != nil {
		return nil, err
	}
	return s.jobs.FindByID(id)
}

func (s *UserScoreRecalculationService) Resume(id uint) (*models.UserScoreRecalculationJob, error) {
	job, err := s.jobs.FindByID(id)
	if err != nil {
		return nil, err
	}
	if job.Status != models.UserScoreRecalculationJobPaused {
		return nil, fmt.Errorf("一時停止中のジョブではありません")
	}
	if err := s.jobs.UpdateStatus(id, models.UserScoreRecalculationJobRunning); err != nil {
		return nil, err
	}
	return s.jobs.FindByID(id)
}

func (s *UserScoreRecalculationService) RunBatch(ctx context.Context, id uint) (result *models.UserScoreRecalculationJob, retErr error) {
	job, err := s.jobs.FindByID(id)
	if err != nil {
		return nil, err
	}
	if job.Status != models.UserScoreRecalculationJobRunning {
		return nil, fmt.Errorf("実行中のジョブではありません")
	}
	leaseToken, err := s.jobs.ClaimBatch(id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := s.jobs.ReleaseBatch(id, leaseToken); err != nil && retErr == nil {
			retErr = fmt.Errorf("再計算ジョブのバッチロック解除に失敗: %w", err)
		}
	}()
	targets, err := s.scores.FindNextRecalculationTargets(
		job.CalculationVersion, job.MaxScoreID, job.CursorScoreID, userScoreRecalculationBatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("再計算対象の取得に失敗: %w", err)
	}
	if len(targets) == 0 {
		if err := s.jobs.CompleteIfFinished(id, true); err != nil {
			return nil, err
		}
		return s.jobs.FindByID(id)
	}

	for _, target := range targets {
		job, err = s.jobs.FindByID(id)
		if err != nil {
			return nil, err
		}
		if job.Status != models.UserScoreRecalculationJobRunning {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		scores, recalcErr := s.chat.RecalculateChatSession(ctx, target.UserID, target.SessionID)
		if recalcErr == nil {
			if len(scores) == 0 {
				recalcErr = errors.New("chat session produced no scores")
			}
		}
		if recalcErr == nil {
			recalcErr = s.scores.ReplaceChatSessionScores(
				target.UserID, target.SessionID, scores, job.CalculationVersion,
			)
		}
		if err := s.jobs.Advance(id, target.ScoreID, leaseToken, recalcErr != nil, sanitizedRecalculationError(recalcErr)); err != nil {
			return nil, fmt.Errorf("再計算ジョブの進捗保存に失敗: %w", err)
		}
	}

	job, err = s.jobs.FindByID(id)
	if err != nil {
		return nil, err
	}
	if job.Status != models.UserScoreRecalculationJobRunning {
		return job, nil
	}
	next, err := s.scores.FindNextRecalculationTargets(
		job.CalculationVersion, job.MaxScoreID, job.CursorScoreID, 1,
	)
	if err != nil {
		return nil, fmt.Errorf("再計算対象の確認に失敗: %w", err)
	}
	if len(next) == 0 {
		if err := s.jobs.CompleteIfFinished(id, true); err != nil {
			return nil, err
		}
	}
	return s.jobs.FindByID(id)
}

func sanitizedRecalculationError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("chat session recalculation failed")
}

func IsRecalculationJobAlreadyRunning(err error) bool {
	return errors.Is(err, repositories.ErrUserScoreRecalculationRunning)
}
