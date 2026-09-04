package queue

import (
	"encoding/json"
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

	QueueDefault  = "default"
	QueueCritical = "critical"
)

// Client は asynq へのエンキューを担う（#617）。
type Client struct {
	client *asynq.Client
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
	return &Client{client: asynq.NewClient(opt)}
}

// Close はクライアントを閉じる。
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
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

func (c *Client) enqueue(taskType, queueName string, payload any, maxRetry int, timeout time.Duration) error {
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

func (c *Client) EnqueueInterviewReport(sessionID uint) error {
	return c.enqueue(TaskInterviewReport, QueueDefault, InterviewReportPayload{SessionID: sessionID}, 3, 10*time.Minute)
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
