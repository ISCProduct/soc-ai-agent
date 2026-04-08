package services_test

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepositories
type mockSessionRepo struct{ mock.Mock }

func (m *mockSessionRepo) Create(s *models.InterviewSession) error { return m.Called(s).Error(0) }
func (m *mockSessionRepo) FindByID(id uint) (*models.InterviewSession, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.InterviewSession), args.Error(1)
}
func (m *mockSessionRepo) Update(s *models.InterviewSession) error { return m.Called(s).Error(0) }
func (m *mockSessionRepo) ListByUser(userID uint, limit, offset int) ([]models.InterviewSession, error) {
	args := m.Called(userID, limit, offset)
	return args.Get(0).([]models.InterviewSession), args.Error(1)
}
func (m *mockSessionRepo) ListAll(limit, offset int) ([]models.InterviewSession, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]models.InterviewSession), args.Error(1)
}
func (m *mockSessionRepo) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	args := m.Called(userID, limit)
	return args.Get(0).([]models.InterviewSession), args.Error(1)
}
func (m *mockSessionRepo) CountByUser(userID uint) (int64, error) {
	args := m.Called(userID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockSessionRepo) CountAll() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockSessionRepo) CountByUserAndDay(userID uint, day time.Time) (int64, error) {
	args := m.Called(userID, day)
	return args.Get(0).(int64), args.Error(1)
}

type mockUtterRepo struct{ mock.Mock }

func (m *mockUtterRepo) Create(u *models.InterviewUtterance) error { return m.Called(u).Error(0) }
func (m *mockUtterRepo) FindBySessionID(sessionID uint) ([]models.InterviewUtterance, error) {
	args := m.Called(sessionID)
	return args.Get(0).([]models.InterviewUtterance), args.Error(1)
}

type mockReportRepo struct{ mock.Mock }

func (m *mockReportRepo) FindBySessionID(sessionID uint) (*models.InterviewReport, error) {
	args := m.Called(sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.InterviewReport), args.Error(1)
}
func (m *mockReportRepo) Upsert(r *models.InterviewReport) error { return m.Called(r).Error(0) }

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) CreateUser(u *entity.User) error { return m.Called(u).Error(0) }
func (m *mockUserRepo) GetUserByEmail(e string) (*entity.User, error) {
	args := m.Called(e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.User), args.Error(1)
}
func (m *mockUserRepo) GetUserByID(id uint) (*entity.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.User), args.Error(1)
}
func (m *mockUserRepo) ListUsers() ([]entity.User, error) {
	args := m.Called()
	return args.Get(0).([]entity.User), args.Error(1)
}
func (m *mockUserRepo) ListUsersPaged(l, o int, q string) ([]entity.User, int64, error) {
	args := m.Called(l, o, q)
	return args.Get(0).([]entity.User), args.Get(1).(int64), args.Error(2)
}
func (m *mockUserRepo) UpdateUser(u *entity.User) error { return m.Called(u).Error(0) }
func (m *mockUserRepo) DeleteUser(id uint) error        { return m.Called(id).Error(0) }
func (m *mockUserRepo) GetUserByVerificationToken(t string) (*entity.User, error) {
	args := m.Called(t)
	return args.Get(0).(*entity.User), args.Error(1)
}
func (m *mockUserRepo) GetUserByPasswordResetToken(t string) (*entity.User, error) {
	args := m.Called(t)
	return args.Get(0).(*entity.User), args.Error(1)
}
func (m *mockUserRepo) GetUserByOAuth(p, o string) (*entity.User, error) {
	args := m.Called(p, o)
	return args.Get(0).(*entity.User), args.Error(1)
}

func TestInterviewService_CreateSession(t *testing.T) {
	sRepo := new(mockSessionRepo)
	uRepo := new(mockUserRepo)
	svc := services.NewInterviewService(sRepo, nil, nil, uRepo, nil, nil, nil)

	t.Run("Success male", func(t *testing.T) {
		uRepo.On("GetUserByID", uint(1)).Return(&entity.User{ID: 1}, nil).Once()
		sRepo.On("Create", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			s := args.Get(0).(*models.InterviewSession)
			assert.Equal(t, "male", s.InterviewerGender)
			assert.Equal(t, "ja", s.Language)
		}).Once()

		resp, err := svc.CreateSession(1, "ja", "male")
		assert.NoError(t, err)
		assert.Equal(t, "male", resp.InterviewerGender)
	})

	t.Run("Default female", func(t *testing.T) {
		uRepo.On("GetUserByID", uint(1)).Return(&entity.User{ID: 1}, nil).Once()
		sRepo.On("Create", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			s := args.Get(0).(*models.InterviewSession)
			assert.Equal(t, "female", s.InterviewerGender)
		}).Once()

		resp, err := svc.CreateSession(1, "ja", "invalid")
		assert.NoError(t, err)
		assert.Equal(t, "female", resp.InterviewerGender)
	})
}

func TestInterviewService_TTSVoiceSelection(t *testing.T) {
	sRepo := new(mockSessionRepo)
	uRepo := new(mockUserRepo)

	t.Run("Male voice onyx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/audio/speech" {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["voice"].(string) == "onyx" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("audio"))
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Hello"}}],"text":"hello"}`))
		}))
		defer server.Close()

		client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
		svc := services.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := &models.InterviewSession{
			ID:                100,
			UserID:            1,
			InterviewerGender: "male",
			Language:          "ja",
		}
		sRepo.On("FindByID", uint(100)).Return(session, nil)

		_, err := svc.StartTurn(context.Background(), 1, 100, "", "", "", "", "")
		assert.NoError(t, err)
	})

	t.Run("Female voice shimmer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/audio/speech" {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["voice"].(string) == "shimmer" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("audio"))
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Hello"}}],"text":"hello"}`))
		}))
		defer server.Close()

		client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
		svc := services.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := &models.InterviewSession{
			ID:                101,
			UserID:            1,
			InterviewerGender: "female",
			Language:          "ja",
		}
		sRepo.On("FindByID", uint(101)).Return(session, nil)

		_, err := svc.StartTurn(context.Background(), 1, 101, "", "", "", "", "")
		assert.NoError(t, err)
	})
}
