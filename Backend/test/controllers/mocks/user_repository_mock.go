package mocks

import (
	"Backend/domain/entity"

	"github.com/stretchr/testify/mock"
)

// UserRepositoryMock UserRepositoryのモック実装
type UserRepositoryMock struct {
	mock.Mock
}

func (m *UserRepositoryMock) CreateUser(user *entity.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *UserRepositoryMock) GetUserByEmail(email string) (*entity.User, error) {
	args := m.Called(email)
	if v := args.Get(0); v != nil {
		return v.(*entity.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) GetUserByID(id uint) (*entity.User, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*entity.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) ListUsers() ([]entity.User, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]entity.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) ListUsersPaged(limit, offset int, query string) ([]entity.User, int64, error) {
	args := m.Called(limit, offset, query)
	if v := args.Get(0); v != nil {
		return v.([]entity.User), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

func (m *UserRepositoryMock) UpdateUser(user *entity.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *UserRepositoryMock) DeleteUser(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *UserRepositoryMock) GetUserByVerificationToken(token string) (*entity.User, error) {
	args := m.Called(token)
	if v := args.Get(0); v != nil {
		return v.(*entity.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) GetUserByPasswordResetToken(token string) (*entity.User, error) {
	args := m.Called(token)
	if v := args.Get(0); v != nil {
		return v.(*entity.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserRepositoryMock) GetUserByOAuth(provider, oauthID string) (*entity.User, error) {
	args := m.Called(provider, oauthID)
	if v := args.Get(0); v != nil {
		return v.(*entity.User), args.Error(1)
	}
	return nil, args.Error(1)
}
