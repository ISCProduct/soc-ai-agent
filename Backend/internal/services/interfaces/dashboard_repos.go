package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
)

// DashboardSessionRepo AdminDashboardControllerが使うセッションリポジトリの最小インターフェース
type DashboardSessionRepo interface {
	GetUserStatsBatch(userIDs []uint) (map[uint]repositories.UserSessionStat, error)
	ListFinishedSessionIDsByUser(userID uint) ([]uint, error)
	ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error)
}

// DashboardReportRepo AdminDashboardControllerが使うレポートリポジトリの最小インターフェース
type DashboardReportRepo interface {
	FindBySessionIDs(sessionIDs []uint) ([]models.InterviewReport, error)
}
