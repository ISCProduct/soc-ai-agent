package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	TaskEmailVerification   = "email:verification"
	TaskEmailReVerification = "email:reverification"
	TaskEmailRegistration   = "email:registration"
	TaskEmailPasswordReset  = "email:password_reset"
	TaskInterviewReport     = "interview:report"
	TaskDiagnosisQuality    = "diagnosis:quality"

	QueueDefault  = "default"
	QueueCritical = "critical"
)

// Client は asynq へのエンキューを担う（#617）。
type Client struct {
	client *asynq.Client
	// inspector は TaskID で重複排除したジョブの状態確認・削除に使う（EnqueueInterviewReport 参照）。
	inspector *asynq.Inspector
}

// NewClient は Redis クライアントから asynq Client を生成する。redis が nil なら nil。
func NewClient(rdb *redis.Client) *Client {
	if rdb == nil {
		return nil
	}
	opt := asynq.RedisClientOpt{
		Addr:     rdb.Options().Addr,
		Password: rdb.Options().Password,
		DB:       rdb.Options().DB,
		Username: rdb.Options().Username,
	}
	return &Client{client: asynq.NewClient(opt), inspector: asynq.NewInspector(opt)}
}

// Close はクライアントを閉じる。
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	if c.inspector != nil {
		_ = c.inspector.Close()
	}
	return c.client.Close()
}

// EmailVerificationPayload は認証メールジョブのペイロード。
type EmailVerificationPayload struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Token  string `json:"token"`
	AppURL string `json:"app_url"`
}

// EmailRegistrationPayload は仮登録メールジョブのペイロード。
type EmailRegistrationPayload struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// EmailPasswordResetPayload は PW リセットメールジョブのペイロード。
type EmailPasswordResetPayload struct {
	Email  string `json:"email"`
	Token  string `json:"token"`
	AppURL string `json:"app_url"`
}

// InterviewReportPayload は面接レポート生成ジョブのペイロード。
type InterviewReportPayload struct {
	SessionID uint `json:"session_id"`
}

// DiagnosisQualityPayload は診断妥当性評価ジョブのペイロード。
type DiagnosisQualityPayload struct {
	UserID    uint   `json:"user_id"`
	SessionID string `json:"session_id"`
}

func (c *Client) enqueue(taskType, queueName string, payload any, maxRetry int, timeout time.Duration, extra ...asynq.Option) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("queue client is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := asynq.NewTask(taskType, body)
	opts := []asynq.Option{
		asynq.Queue(queueName),
		asynq.MaxRetry(maxRetry),
		asynq.Timeout(timeout),
		asynq.Retention(24 * time.Hour),
	}
	opts = append(opts, extra...)
	info, err := c.client.Enqueue(task, opts...)
	if err != nil {
		return err
	}
	log.Printf("[queue] enqueued type=%s id=%s queue=%s", taskType, info.ID, info.Queue)
	return nil
}

func (c *Client) EnqueueEmailVerification(p EmailVerificationPayload) error {
	return c.enqueue(TaskEmailVerification, QueueCritical, p, 5, 2*time.Minute)
}

func (c *Client) EnqueueEmailReVerification(p EmailVerificationPayload) error {
	return c.enqueue(TaskEmailReVerification, QueueCritical, p, 5, 2*time.Minute)
}

func (c *Client) EnqueueEmailRegistration(p EmailRegistrationPayload) error {
	return c.enqueue(TaskEmailRegistration, QueueCritical, p, 5, 2*time.Minute)
}

func (c *Client) EnqueueEmailPasswordReset(p EmailPasswordResetPayload) error {
	return c.enqueue(TaskEmailPasswordReset, QueueCritical, p, 5, 2*time.Minute)
}

// interviewReportTimeout はレポート生成ジョブの1回あたりの実行時間上限。
const interviewReportTimeout = 10 * time.Minute

// InterviewReportTaskID はレポート生成ジョブのセッション単位の ID。
func InterviewReportTaskID(sessionID uint) string {
	return fmt.Sprintf("interview-report-%d", sessionID)
}

// EnqueueInterviewReport はレポート生成ジョブをセッション単位で重複排除して投入する(#1476)。
//
// 生成前のセッションへ FinishSession と RegenerateReport が重なると、同じセッションに
// 複数のジョブが走る。各ジョブが LLM を呼んで Upsert するため、費用が二重に掛かり、
// 後勝ちでレポートが上書きされ、スコアの移動平均も二重に効く。
//
// 重複排除は asynq.Unique ではなく asynq.TaskID で行う。
//   - Unique のロックは「投入からTTLまで」しか効かない。TTL(=1回の実行上限)は
//     キュー待ち＋最大3回の再試行を含まないため、再試行待ちのジョブに重ねて投入できてしまう。
//   - TaskID は pending/active/scheduled/retry の全状態と衝突するので、
//     ジョブが生きている間はずっと重複を弾ける。
//
// TaskID の弱点は「終わったタスク」とも衝突すること（Retention(24h) の completed、
// 再試行を使い切った archived）。ここを素通しにすると、#1476 で足した唯一の回復手段である
// 再生成が最大24時間塞がれる。そこで衝突時はタスクの状態を見て、
// 終わっているものだけ消してから入れ直す（＝回復は決して塞がらない）。
func (c *Client) EnqueueInterviewReport(sessionID uint) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("queue client is not configured")
	}
	taskID := InterviewReportTaskID(sessionID)
	err := c.enqueueInterviewReport(sessionID, taskID)
	if !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	replaceable, err := c.dropFinishedTask(QueueDefault, taskID)
	if err != nil {
		return err
	}
	if !replaceable {
		// まだ生きているジョブがある＝望む状態なので成功扱いにする。
		// ここでエラーを返すと呼び出し側がフォールバックの channel へ二重投入してしまう。
		log.Printf("[queue] interview report already queued session=%d", sessionID)
		return nil
	}
	err = c.enqueueInterviewReport(sessionID, taskID)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		// 消した直後に別の要求が入れ直した。望む状態なので成功扱い。
		return nil
	}
	return err
}

func (c *Client) enqueueInterviewReport(sessionID uint, taskID string) error {
	return c.enqueue(TaskInterviewReport, QueueDefault, InterviewReportPayload{SessionID: sessionID},
		3, interviewReportTimeout, asynq.TaskID(taskID))
}

// dropFinishedTask は ID が衝突したタスクが「終わっている」なら削除し true を返す。
// 生きている（pending/active/scheduled/retry/aggregating）なら消さずに false を返す。
func (c *Client) dropFinishedTask(queueName, taskID string) (bool, error) {
	if c.inspector == nil {
		return false, nil
	}
	info, err := c.inspector.GetTaskInfo(queueName, taskID)
	switch {
	case errors.Is(err, asynq.ErrTaskNotFound), errors.Is(err, asynq.ErrQueueNotFound):
		// 衝突判定との間に消えた。入れ直してよい。
		return true, nil
	case err != nil:
		return false, err
	}
	if info.State != asynq.TaskStateArchived && info.State != asynq.TaskStateCompleted {
		return false, nil
	}
	log.Printf("[queue] replacing finished task id=%s state=%s", taskID, info.State)
	if err := c.inspector.DeleteTask(queueName, taskID); err != nil && !errors.Is(err, asynq.ErrTaskNotFound) {
		return false, err
	}
	return true, nil
}

func (c *Client) EnqueueDiagnosisQuality(userID uint, sessionID string) error {
	return c.enqueue(TaskDiagnosisQuality, QueueDefault, DiagnosisQualityPayload{UserID: userID, SessionID: sessionID}, 3, 5*time.Minute)
}

// RedisOptFromClient は go-redis Client から asynq の RedisConnOpt を作る。
func RedisOptFromClient(rdb *redis.Client) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     rdb.Options().Addr,
		Password: rdb.Options().Password,
		DB:       rdb.Options().DB,
		Username: rdb.Options().Username,
	}
}
