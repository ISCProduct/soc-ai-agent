package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"Backend/domain/entity"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// EmailSender はキューワーカーが使うメール送信面。
type EmailSender interface {
	SendVerificationEmail(user *entity.User, token, appURL string) error
	SendReVerificationEmail(user *entity.User, token, appURL string) error
	SendRegistrationEmail(email, token string) error
	SendPasswordResetEmail(email, token, appURL string) error
}

// InterviewReportGenerator は面接レポート生成面。
type InterviewReportGenerator interface {
	GenerateReportForSession(ctx context.Context, sessionID uint) error
}

// Server は asynq ワーカーを起動する（#617）。
type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// NewServer は Redis から asynq Server を構築する。
func NewServer(rdb *redis.Client) *Server {
	if rdb == nil {
		return nil
	}
	srv := asynq.NewServer(RedisOptFromClient(rdb), asynq.Config{
		Concurrency: 4,
		Queues: map[string]int{
			QueueCritical: 6,
			QueueDefault:  3,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Printf("[queue] task failed type=%s err=%v", task.Type(), err)
		}),
	})
	return &Server{server: srv, mux: asynq.NewServeMux()}
}

// RegisterHandlers はメール・面接レポートのハンドラを登録する。
func (s *Server) RegisterHandlers(email EmailSender, interview InterviewReportGenerator) {
	if s == nil {
		return
	}
	s.mux.HandleFunc(TaskEmailVerification, handleEmailVerification(email))
	s.mux.HandleFunc(TaskEmailReVerification, handleEmailReVerification(email))
	s.mux.HandleFunc(TaskEmailRegistration, handleEmailRegistration(email))
	s.mux.HandleFunc(TaskEmailPasswordReset, handleEmailPasswordReset(email))
	s.mux.HandleFunc(TaskInterviewReport, handleInterviewReport(interview))
}

// Start はブロッキングせずにワーカーを開始する。
func (s *Server) Start() error {
	if s == nil || s.server == nil {
		return nil
	}
	go func() {
		log.Printf("[queue] asynq worker starting")
		if err := s.server.Run(s.mux); err != nil {
			log.Printf("[queue] asynq worker stopped: %v", err)
		}
	}()
	return nil
}

// Shutdown はワーカーを停止する。
func (s *Server) Shutdown() {
	if s == nil || s.server == nil {
		return
	}
	s.server.Shutdown()
}

func handleEmailVerification(email EmailSender) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p EmailVerificationPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		user := &entity.User{ID: p.UserID, Email: p.Email, Name: p.Name}
		if err := email.SendVerificationEmail(user, p.Token, p.AppURL); err != nil {
			return err
		}
		return nil
	}
}

func handleEmailReVerification(email EmailSender) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p EmailVerificationPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		user := &entity.User{ID: p.UserID, Email: p.Email, Name: p.Name}
		return email.SendReVerificationEmail(user, p.Token, p.AppURL)
	}
}

func handleEmailRegistration(email EmailSender) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p EmailRegistrationPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		return email.SendRegistrationEmail(p.Email, p.Token)
	}
}

func handleEmailPasswordReset(email EmailSender) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p EmailPasswordResetPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		return email.SendPasswordResetEmail(p.Email, p.Token, p.AppURL)
	}
}

func handleInterviewReport(interview InterviewReportGenerator) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p InterviewReportPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		if interview == nil {
			return fmt.Errorf("interview report generator is nil")
		}
		return interview.GenerateReportForSession(ctx, p.SessionID)
	}
}
