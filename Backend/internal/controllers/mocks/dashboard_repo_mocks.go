package mocks

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"time"

	"github.com/stretchr/testify/mock"
)

// DashboardSessionRepoMock DashboardSessionRepo のモック実装
type DashboardSessionRepoMock struct {
	mock.Mock
}

func (m *DashboardSessionRepoMock) GetUserStatsBatch(userIDs []uint) (map[uint]repositories.UserSessionStat, error) {
	args := m.Called(userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uint]repositories.UserSessionStat), args.Error(1)
}

func (m *DashboardSessionRepoMock) ListFinishedSessionIDsByUser(userID uint) ([]uint, error) {
	args := m.Called(userID)
	return args.Get(0).([]uint), args.Error(1)
}

func (m *DashboardSessionRepoMock) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	args := m.Called(userID, limit)
	return args.Get(0).([]models.InterviewSession), args.Error(1)
}

// DashboardReportRepoMock DashboardReportRepo のモック実装
type DashboardReportRepoMock struct {
	mock.Mock
}

func (m *DashboardReportRepoMock) FindBySessionIDs(sessionIDs []uint) ([]models.InterviewReport, error) {
	args := m.Called(sessionIDs)
	return args.Get(0).([]models.InterviewReport), args.Error(1)
}

// stubLastSessionAt はテスト用に時刻ポインタを返すヘルパー
func stubTime(t time.Time) *time.Time { return &t }
