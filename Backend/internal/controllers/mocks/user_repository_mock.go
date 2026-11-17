package mocks

import (
	"Backend/internal/models"
	"time"

	"github.com/stretchr/testify/mock"
)

// UserRepositoryMock UserRepositoryのモック実装
type UserRepositoryMock struct {
	mock.Mock
}

func (m *UserRepositoryMock) CreateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *UserRepositoryMock) GetUserByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if v := args.Get(0); v != nil {
		return v.(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) GetUserByID(id uint) (*models.User, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]models.User, int64, error) {
	args := m.Called(limit, offset, query, schoolID)
	if v := args.Get(0); v != nil {
		return v.([]models.User), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

func (m *UserRepositoryMock) UpdateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *UserRepositoryMock) DeleteUser(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *UserRepositoryMock) RenewEmailVerification(userID uint, token string, expires time.Time) (bool, error) {
	args := m.Called(userID, token, expires)
	return args.Bool(0), args.Error(1)
}

func (m *UserRepositoryMock) GetUserByVerificationToken(token string) (*models.User, error) {
	args := m.Called(token)
	if v := args.Get(0); v != nil {
		return v.(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) GetUserByPasswordResetToken(token string) (*models.User, error) {
	args := m.Called(token)
	if v := args.Get(0); v != nil {
		return v.(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) GetUserByOAuth(provider, oauthID string) (*models.User, error) {
	args := m.Called(provider, oauthID)
	if v := args.Get(0); v != nil {
		return v.(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}
