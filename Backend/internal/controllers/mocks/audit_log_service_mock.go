package mocks

import (
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// AuditLogServiceMock AuditLogServiceのモック実装
type AuditLogServiceMock struct {
	mock.Mock
}

func (m *AuditLogServiceMock) Record(actorEmail, action, targetType string, targetID uint, metadata map[string]interface{}) {
	m.Called(actorEmail, action, targetType, targetID, metadata)
}

func (m *AuditLogServiceMock) List(limit int) ([]models.AuditLog, error) {
	args := m.Called(limit)
	if v := args.Get(0); v != nil {
		return v.([]models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}
