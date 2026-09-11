package repository

import (
	"Backend/internal/models"
	"context"
	"time"
)

// InterviewSessionRepository は面接セッションの永続化インターフェース。
type InterviewSessionRepository interface {
	Create(session *models.InterviewSession) error
	FindByID(id uint) (*models.InterviewSession, error)
	Update(session *models.InterviewSession) error
	ListByUser(userID uint, limit int, offset int) ([]models.InterviewSession, error)
	// schoolID / companyID が非nilの場合はその条件で絞り込む。
	ListAll(limit int, offset int, schoolID *uint, companyID *uint) ([]models.InterviewSession, error)
	// ListFinishedByUser は完了済み（status="finished"）セッションを新しい順に最大 limit 件返す。
	// トレンド分析用。
	ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error)
	CountByUser(userID uint) (int64, error)
	CountAll(schoolID *uint, companyID *uint) (int64, error)
	CountByUserAndDay(userID uint, day time.Time) (int64, error)
}

// InterviewUtteranceRepository は面接発話ログの永続化インターフェース。
type InterviewUtteranceRepository interface {
	Create(utterance *models.InterviewUtterance) error
	FindBySessionID(sessionID uint) ([]models.InterviewUtterance, error)
}

// InterviewReportRepository は面接レポートの永続化インターフェース。
type InterviewReportRepository interface {
	FindBySessionID(sessionID uint) (*models.InterviewReport, error)
	Upsert(report *models.InterviewReport) error
}

// InterviewVideoRepository は面接動画メタデータの永続化インターフェース。
type InterviewVideoRepository interface {
	Create(ctx context.Context, v *models.InterviewVideo) error
	UpdateStatus(ctx context.Context, id uint, status, errorMessage string, driveFileID, driveFileURL string, uploadedAt *time.Time) error
	FindBySessionID(ctx context.Context, sessionID uint) ([]models.InterviewVideo, error)
	FindByID(ctx context.Context, id uint) (*models.InterviewVideo, error)
}

// InterviewCompanyQuestionRepository は企業別カスタム面接質問の永続化インターフェース。
type InterviewCompanyQuestionRepository interface {
	FindByCompanyID(companyID uint) ([]models.InterviewCompanyQuestion, error)
	FindByCompanyAndPosition(companyID uint, position string) ([]models.InterviewCompanyQuestion, error)
	FindByID(id uint) (*models.InterviewCompanyQuestion, error)
	Create(q *models.InterviewCompanyQuestion) error
	Update(q *models.InterviewCompanyQuestion) error
	Delete(id uint) error
}

// InterviewQuestionStateRepository は面接中の質問キュー状態を永続化する。
type InterviewQuestionStateRepository interface {
	CountBySessionID(sessionID uint) (int64, error)
	CreateBatch(states []models.InterviewQuestionState) error
	Create(state *models.InterviewQuestionState) error
	Update(state *models.InterviewQuestionState) error
	FindBySessionID(sessionID uint) ([]models.InterviewQuestionState, error)
	FindLatestAsked(sessionID uint) (*models.InterviewQuestionState, error)
}
