package admin

import (
	"context"
	"errors"
	"fmt"
	"log"

	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/chat"
	"Backend/internal/services/costs"
)

const userScoreRecalculationBatchSize = 10

var (
	ErrRecalculationJobNotRunning = errors.New("実行中のジョブではありません")
	ErrRecalculationJobNotPaused  = errors.New("一時停止中のジョブではありません")
)

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
	// ActiveJob は実行中または一時停止中のジョブ。別の管理者や別の端末からも
	// 続行・中止できるように返す。無ければ null。
	ActiveJob *models.UserScoreRecalculationJob `json:"active_job"`
}

func (s *UserScoreRecalculationService) Preview() (*UserScoreRecalculationPreview, error) {
	stats, err := s.scores.RecalculationStats(models.CurrentUserScoreCalculationVersion)
	if err != nil {
		return nil, fmt.Errorf("スコア再計算対象の取得に失敗: %w", err)
	}
	activeJob, err := s.jobs.FindActive()
	if err != nil {
		return nil, fmt.Errorf("進行中の再計算ジョブの取得に失敗: %w", err)
	}
	preview := s.estimate(stats)
	preview.ActiveJob = activeJob
	return preview, nil
}

func (s *UserScoreRecalculationService) estimate(stats repositories.UserScoreRecalculationStats) *UserScoreRecalculationPreview {
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
	}
}

func (s *UserScoreRecalculationService) Start() (*models.UserScoreRecalculationJob, error) {
	stats, err := s.scores.RecalculationStats(models.CurrentUserScoreCalculationVersion)
	if err != nil {
		return nil, fmt.Errorf("スコア再計算対象の取得に失敗: %w", err)
	}
	preview := s.estimate(stats)
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
		return nil, ErrRecalculationJobNotRunning
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
		return nil, ErrRecalculationJobNotPaused
	}
	if err := s.jobs.UpdateStatus(id, models.UserScoreRecalculationJobRunning); err != nil {
		return nil, err
	}
	return s.jobs.FindByID(id)
}

// Cancel は一時停止中のジョブを中止する。再計算済みのセッションは元に戻らない。
// 未処理のセッションは旧計算版のまま残るので、次のジョブの対象になる。
func (s *UserScoreRecalculationService) Cancel(id uint) (*models.UserScoreRecalculationJob, error) {
	job, err := s.jobs.FindByID(id)
	if err != nil {
		return nil, err
	}
	if job.Status != models.UserScoreRecalculationJobPaused {
		return nil, ErrRecalculationJobNotPaused
	}
	if err := s.jobs.Cancel(id); err != nil {
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
		return nil, ErrRecalculationJobNotRunning
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
		// 一時停止をセッションごとに拾うため毎回読み直す。1セッションで LLM を
		// 何度も呼ぶので、主キー検索1回の負荷より、止まるまでの費用を優先する。
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
		failure, err := s.recalculateTarget(ctx, id, target, job.CalculationVersion)
		if err != nil {
			return nil, err
		}
		if err := s.jobs.Advance(id, target.ScoreID, leaseToken, failure); err != nil {
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

// recalculateTarget は1セッションを再計算して保存し、失敗の分類を返す（成功なら空）。
// リクエストが切れた場合だけ err を返す。LLM の失敗はルール採点へ切り替わり、
// エラーとして返らない。切断の直後に保存すると LLM の評価が欠けた値で上書きするので、
// 保存せず進捗も進めず、次のバッチで処理し直す。
func (s *UserScoreRecalculationService) recalculateTarget(
	ctx context.Context,
	jobID uint,
	target repositories.UserScoreRecalculationTarget,
	version string,
) (string, error) {
	scores, err := s.chat.RecalculateChatSession(ctx, target.UserID, target.SessionID)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		logRecalculationFailure(jobID, target, models.UserScoreRecalculationFailureLoad, err)
		return models.UserScoreRecalculationFailureLoad, nil
	}
	if len(scores) == 0 {
		logRecalculationFailure(jobID, target, models.UserScoreRecalculationFailureNoScores, errors.New("採点できる回答がありません"))
		return models.UserScoreRecalculationFailureNoScores, nil
	}
	if err := s.scores.ReplaceChatSessionScores(target.UserID, target.SessionID, scores, version); err != nil {
		logRecalculationFailure(jobID, target, models.UserScoreRecalculationFailureSave, err)
		return models.UserScoreRecalculationFailureSave, nil
	}
	return "", nil
}

func logRecalculationFailure(jobID uint, target repositories.UserScoreRecalculationTarget, failure string, err error) {
	log.Printf("[UserScoreRecalculation] job=%d score_id=%d user_id=%d session_id=%s failure=%s: %v",
		jobID, target.ScoreID, target.UserID, target.SessionID, failure, err)
}

func IsRecalculationJobAlreadyRunning(err error) bool {
	return errors.Is(err, repositories.ErrUserScoreRecalculationRunning)
}

// RecalculationConflictMessage は、状態の競合で操作できないときに管理者へ見せる文を返す。
// 競合でなければ false。
func RecalculationConflictMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrRecalculationJobNotRunning), errors.Is(err, ErrRecalculationJobNotPaused):
		return err.Error(), true
	case errors.Is(err, repositories.ErrUserScoreRecalculationBatchBusy):
		return "このジョブのバッチを別の処理が実行中です", true
	case errors.Is(err, repositories.ErrUserScoreRecalculationStatusChanged):
		return "ジョブの状態が変わったか、実行中のバッチが残っています。少し待ってからやり直してください", true
	}
	return "", false
}
