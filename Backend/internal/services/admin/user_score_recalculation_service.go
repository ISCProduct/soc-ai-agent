package admin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strconv"
	"strings"

	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/chat"
	"Backend/internal/services/costs"
	"Backend/internal/services/matching"
)

const (
	userScoreRecalculationBatchSize = 10
	// recalculationDryRunMaxSessions はドライランで再計算するセッションの上限。LLM の費用がかかる。
	recalculationDryRunMaxSessions = 5
	// recalculationRollbackBatchSize は1回のロールバックで戻すセッションの上限。
	// マッチングの作り直しがあるので、1リクエストを長くしない。
	recalculationRollbackBatchSize = 20
)

var (
	ErrRecalculationJobNotRunning = errors.New("実行中のジョブではありません")
	ErrRecalculationJobNotPaused  = errors.New("一時停止中のジョブではありません")
	ErrRecalculationJobActive     = errors.New("実行中または一時停止中のジョブがある間は元に戻せません")
)

// RecalculationMatchingRunner はスコア置換後に企業マッチングを作り直す。
type RecalculationMatchingRunner interface {
	CalculateMatching(ctx context.Context, userID uint, sessionID string) error
}

type UserScoreRecalculationService struct {
	scores   *repositories.UserWeightScoreRepository
	jobs     *repositories.UserScoreRecalculationRepository
	chat     *chat.ChatService
	matching RecalculationMatchingRunner
}

func NewUserScoreRecalculationService(
	scores *repositories.UserWeightScoreRepository,
	jobs *repositories.UserScoreRecalculationRepository,
	chatService *chat.ChatService,
	matching RecalculationMatchingRunner,
) *UserScoreRecalculationService {
	return &UserScoreRecalculationService{scores: scores, jobs: jobs, chat: chatService, matching: matching}
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
	calls := estimateRecalculationLLMCalls(stats, matching.ReasonUsesAI(), config.MatchingReasonAITopN())
	estimatedCost := 0.0
	if billable && calls > 0 {
		// すべての回答に短文・境界値評価が発生した場合として保守的に見積もる。
		estimatedCost = costs.EstimateCostUSD(model, 2048, 256) * float64(calls)
	}
	return &UserScoreRecalculationPreview{
		CalculationVersion:  models.CurrentUserScoreCalculationVersion,
		EligibleSessions:    stats.EligibleSessions,
		UnsupportedSessions: stats.UnsupportedSessions,
		EstimatedLLMCalls:   calls,
		EstimatedCostUSD:    estimatedCost,
		CostEstimateNote: "回答1件あたり入力2048・出力256トークン、全回答でLLM評価が発生する前提の概算です。" +
			"ローカル推論先の構成では、全件が OpenAI へフォールバックした場合の上限を示します。" +
			"実際の呼び出し数と費用はこれを下回る場合があります。" +
			"MATCHING_REASON_USE_AI が有効なら、セッションごとのマッチング理由の生成も含みます。",
	}
}

// estimateRecalculationLLMCalls は LLM 呼び出しの上限を返す。回答ごとの評価に加えて、
// マッチング理由を AI で作る設定なら、置換のたびに上位 topN 社ぶんの理由を生成する。
func estimateRecalculationLLMCalls(stats repositories.UserScoreRecalculationStats, reasonUsesAI bool, topN int) uint {
	calls := stats.EligibleAnswers
	if reasonUsesAI && topN > 0 {
		calls += stats.EligibleSessions * uint(topN)
	}
	return calls
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

	degraded := 0
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
		// リースは取得時から一定時間で切れる。セッションの区切りで延ばさないと、長い
		// バッチの途中で別のリクエストが同じジョブを取得し、同じセッションの LLM を呼ぶ。
		// 延ばせなければ別のバッチが取得済みなので、LLM を呼ぶ前に打ち切る。
		if err := s.jobs.RenewBatch(id, leaseToken); err != nil {
			return nil, err
		}
		failure, err := s.recalculateTarget(ctx, id, target, job.CalculationVersion)
		if err != nil {
			return nil, err
		}
		if err := s.jobs.Advance(id, target.ScoreID, leaseToken, failure); err != nil {
			return nil, fmt.Errorf("再計算ジョブの進捗保存に失敗: %w", err)
		}
		// LLM の障害が続くなら止める（B3）。続けても劣化値は保存しないが、LLM の呼び出しの費用だけがかかる
		if failure == models.UserScoreRecalculationFailureLLMDegraded {
			degraded++
		} else {
			degraded = 0
		}
		if degraded >= recalculationMaxConsecutiveLLMDegraded {
			log.Printf("[UserScoreRecalculation] job=%d LLM が %d セッション続けて使えないため一時停止します", id, degraded)
			if err := s.jobs.UpdateStatus(id, models.UserScoreRecalculationJobPaused); err != nil {
				return nil, err
			}
			break
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
// リクエストが切れた場合だけ err を返す。切断の直後に保存すると LLM の評価が欠けた値で
// 上書きするので、保存せず進捗も進めず、次のバッチで処理し直す。LLM が使えなかった
// セッションも保存せず llm_degraded に数える（計算版を進めないので次のジョブで拾い直す）。
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
	if errors.Is(err, chat.ErrRecalculationAxisUnknown) {
		// 軸を推測して置換すると元の軸のスコアが消えるので、スコアは残して計算版だけ進める
		return s.markUnscorableSession(jobID, target, version, models.UserScoreRecalculationFailureAxisUnknown,
			"保存軸もテンプレートの軸も無い回答があるため、スコアは残し、計算版だけ現行にしました"), nil
	}
	if errors.Is(err, chat.ErrRecalculationLLMUnavailable) {
		// 劣化値で確定させない。保存も計算版の更新もしないので、次のジョブで拾い直す
		logRecalculationFailure(jobID, target, models.UserScoreRecalculationFailureLLMDegraded, err)
		return models.UserScoreRecalculationFailureLLMDegraded, nil
	}
	if err != nil {
		logRecalculationFailure(jobID, target, models.UserScoreRecalculationFailureLoad, err)
		return models.UserScoreRecalculationFailureLoad, nil
	}
	if len(scores) == 0 {
		return s.markUnscorableSession(jobID, target, version, models.UserScoreRecalculationFailureNoScores,
			"採点できる回答がありません。スコアは残し、計算版だけ現行にしました"), nil
	}
	return s.saveRecalculatedScores(ctx, jobID, target, scores, version), nil
}

// markUnscorableSession は採点できる回答が無いセッションを、スコアを残したまま現行の
// 計算版にして、失敗の分類を返す。計算版が古いままだと次のジョブで先頭に戻り、履歴が
// 変わらなくても同じ LLM 評価の費用がジョブのたびにかかる。一時的な失敗（load_failed）と
// 違い、やり直しても結果は変わらない。
func (s *UserScoreRecalculationService) markUnscorableSession(
	jobID uint,
	target repositories.UserScoreRecalculationTarget,
	version string,
	failureKind string,
	reason string,
) string {
	if err := s.scores.MarkChatSessionCalculationVersion(target.UserID, target.SessionID, version); err != nil {
		failure := models.UserScoreRecalculationFailureSave
		if errors.Is(err, repositories.ErrRecalculationTargetChanged) {
			failure = models.UserScoreRecalculationFailureTargetChanged
		}
		logRecalculationFailure(jobID, target, failure, err)
		return failure
	}
	logRecalculationFailure(jobID, target, failureKind, errors.New(reason))
	return failureKind
}

// saveRecalculatedScores はスコアを置換して企業マッチングを作り直し、失敗の分類を返す（成功なら空）。
func (s *UserScoreRecalculationService) saveRecalculatedScores(
	ctx context.Context,
	jobID uint,
	target repositories.UserScoreRecalculationTarget,
	scores map[string]int,
	version string,
) string {
	if err := s.scores.ReplaceChatSessionScores(jobID, target.UserID, target.SessionID, scores, version); err != nil {
		failure := models.UserScoreRecalculationFailureSave
		if errors.Is(err, repositories.ErrRecalculationTargetChanged) {
			failure = models.UserScoreRecalculationFailureTargetChanged
		}
		logRecalculationFailure(jobID, target, failure, err)
		return failure
	}
	// 保存済みの user_company_matches は自動では更新されない。作り直さないと、
	// 利用者には旧スコアの企業順位が出続ける。スコアは保存済みなので、失敗しても
	// 進捗は進める（次の診断やスコア反映で作り直される）。
	if err := s.matching.CalculateMatching(ctx, target.UserID, target.SessionID); err != nil {
		logRecalculationFailure(jobID, target, models.UserScoreRecalculationFailureMatching, err)
		return models.UserScoreRecalculationFailureMatching
	}
	return ""
}

func logRecalculationFailure(jobID uint, target repositories.UserScoreRecalculationTarget, failure string, err error) {
	log.Printf("[UserScoreRecalculation] job=%d score_id=%d user_id=%d session_id=%s failure=%s: %v",
		jobID, target.ScoreID, target.UserID, target.SessionID, failure, err)
}

// recalculationMaxConsecutiveLLMDegraded は、LLM が使えないセッションがこの数だけ続いたらジョブを一時停止する数。
const recalculationMaxConsecutiveLLMDegraded = 3

// UserScoreRecalculationRollback は1回のロールバックの結果。Remaining が 0 になるまで、
// NextAfterID を after_id に渡して繰り返す。
type UserScoreRecalculationRollback struct {
	Restored       int   `json:"restored"`
	Skipped        int   `json:"skipped"`
	MatchingFailed int   `json:"matching_failed"`
	Remaining      int64 `json:"remaining"`
	NextAfterID    uint  `json:"next_after_id"`
}

// Rollback はジョブが置き換えたスコアを元に戻し、企業マッチングも作り直す。
// afterID より後のスナップショットから処理する（戻さなかった分の控えは残すため、ID のカーソルで進める）。
func (s *UserScoreRecalculationService) Rollback(ctx context.Context, id, afterID uint) (*UserScoreRecalculationRollback, error) {
	job, err := s.jobs.FindByID(id)
	if err != nil {
		return nil, err
	}
	active, err := s.jobs.FindActive()
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrRecalculationJobActive
	}
	batch, err := s.scores.RollbackRecalculatedSessions(id, job.CalculationVersion, recalculationRollbackBatchSize, afterID)
	if err != nil {
		return nil, fmt.Errorf("スコアの復元に失敗: %w", err)
	}
	result := &UserScoreRecalculationRollback{Restored: len(batch.Restored), Skipped: batch.Skipped, NextAfterID: batch.LastSnapshotID}
	for _, target := range batch.Restored {
		if err := s.matching.CalculateMatching(ctx, target.UserID, target.SessionID); err != nil {
			logRecalculationFailure(id, target, models.UserScoreRecalculationFailureMatching, err)
			result.MatchingFailed++
		}
	}
	result.Remaining, err = s.jobs.CountSnapshots(id, batch.LastSnapshotID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RecalculationDryRunScore は1カテゴリの現在の値と再計算後の値。値がなければ nil。
type RecalculationDryRunScore struct {
	Category     string `json:"category"`
	Current      *int   `json:"current"`
	Recalculated *int   `json:"recalculated"`
}

// RecalculationDryRunSession は1セッションのドライラン結果。
type RecalculationDryRunSession struct {
	ScoreID uint                       `json:"score_id"`
	UserID  uint                       `json:"user_id"`
	Scores  []RecalculationDryRunScore `json:"scores"`
	Failure string                     `json:"failure,omitempty"`
}

// UserScoreRecalculationDryRun はドライランの結果。Text は比較表。
type UserScoreRecalculationDryRun struct {
	Sessions []RecalculationDryRunSession `json:"sessions"`
	Text     string                       `json:"text"`
}

// DryRun は対象の先頭 limit セッションを再計算し、保存せずに現在の値と比べる。
// 再計算と同じ LLM を呼ぶので、費用を抑えるためセッション数に上限を置く。
func (s *UserScoreRecalculationService) DryRun(ctx context.Context, limit int) (*UserScoreRecalculationDryRun, error) {
	limit = max(1, min(limit, recalculationDryRunMaxSessions))
	stats, err := s.scores.RecalculationStats(models.CurrentUserScoreCalculationVersion)
	if err != nil {
		return nil, fmt.Errorf("スコア再計算対象の取得に失敗: %w", err)
	}
	targets, err := s.scores.FindNextRecalculationTargets(models.CurrentUserScoreCalculationVersion, stats.MaxScoreID, 0, limit)
	if err != nil {
		return nil, fmt.Errorf("再計算対象の取得に失敗: %w", err)
	}
	sessions := make([]RecalculationDryRunSession, 0, len(targets))
	for _, target := range targets {
		session := RecalculationDryRunSession{ScoreID: target.ScoreID, UserID: target.UserID}
		recalculated, err := s.chat.RecalculateChatSession(ctx, target.UserID, target.SessionID)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			logRecalculationFailure(0, target, models.UserScoreRecalculationFailureLoad, err)
			session.Failure = models.UserScoreRecalculationFailureLoad
			sessions = append(sessions, session)
			continue
		}
		current, err := s.scores.ChatSessionScoreMap(target.UserID, target.SessionID)
		if err != nil {
			return nil, fmt.Errorf("現在のスコアの取得に失敗: %w", err)
		}
		session.Scores = compareRecalculatedScores(current, recalculated)
		sessions = append(sessions, session)
	}
	return &UserScoreRecalculationDryRun{Sessions: sessions, Text: FormatUserScoreRecalculationDryRun(sessions)}, nil
}

func compareRecalculatedScores(current, recalculated map[string]int) []RecalculationDryRunScore {
	categories := make(map[string]struct{}, len(current)+len(recalculated))
	for category := range current {
		categories[category] = struct{}{}
	}
	for category := range recalculated {
		categories[category] = struct{}{}
	}
	scores := make([]RecalculationDryRunScore, 0, len(categories))
	for _, category := range slices.Sorted(maps.Keys(categories)) {
		score := RecalculationDryRunScore{Category: category}
		if value, ok := current[category]; ok {
			score.Current = &value
		}
		if value, ok := recalculated[category]; ok {
			score.Recalculated = &value
		}
		scores = append(scores, score)
	}
	return scores
}

// FormatUserScoreRecalculationDryRun はドライランの結果を比較表にする。
// flywheel.FormatScoreMappingDryRun と同じく、表の形はゴールデンで固定する。
func FormatUserScoreRecalculationDryRun(sessions []RecalculationDryRunSession) string {
	var b strings.Builder
	b.WriteString("=== スコア再計算のドライラン（保存していません）===\n")
	if len(sessions) == 0 {
		b.WriteString("再計算の対象はありません\n")
		return b.String()
	}
	for _, session := range sessions {
		fmt.Fprintf(&b, "\n--- user_id=%d score_id=%d ---\n", session.UserID, session.ScoreID)
		if session.Failure != "" {
			fmt.Fprintf(&b, "再計算できませんでした: %s\n", session.Failure)
			continue
		}
		if len(session.Scores) == 0 {
			b.WriteString("採点できる回答がありません\n")
			continue
		}
		b.WriteString("現在 | 再計算後 | 差分 | カテゴリ\n")
		for _, score := range session.Scores {
			fmt.Fprintf(&b, "%4s | %8s | %4s | %s\n",
				dryRunValue(score.Current), dryRunValue(score.Recalculated), dryRunDelta(score), score.Category)
		}
	}
	return b.String()
}

func dryRunValue(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

func dryRunDelta(score RecalculationDryRunScore) string {
	if score.Current == nil || score.Recalculated == nil {
		return "-"
	}
	return fmt.Sprintf("%+d", *score.Recalculated-*score.Current)
}

func IsRecalculationJobAlreadyRunning(err error) bool {
	return errors.Is(err, repositories.ErrUserScoreRecalculationRunning)
}

// RecalculationConflictMessage は、状態の競合で操作できないときに管理者へ見せる文を返す。
// 競合でなければ false。
func RecalculationConflictMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrRecalculationJobNotRunning), errors.Is(err, ErrRecalculationJobNotPaused), errors.Is(err, ErrRecalculationJobActive):
		return err.Error(), true
	case errors.Is(err, repositories.ErrUserScoreRecalculationBatchBusy):
		return "このジョブのバッチを別の処理が実行中です", true
	case errors.Is(err, repositories.ErrUserScoreRecalculationStatusChanged):
		return "ジョブの状態が変わったか、実行中のバッチが残っています。少し待ってからやり直してください", true
	}
	return "", false
}
