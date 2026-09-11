package interview_test

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/interview"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
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
func (m *mockSessionRepo) ListAll(limit, offset int, schoolID *uint, companyID *uint) ([]models.InterviewSession, error) {
	args := m.Called(limit, offset, schoolID, companyID)
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
func (m *mockSessionRepo) CountAll(schoolID *uint, companyID *uint) (int64, error) {
	args := m.Called(schoolID, companyID)
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
func (m *mockUserRepo) ListUsersPaged(l, o int, q string, schoolID *uint) ([]entity.User, int64, error) {
	args := m.Called(l, o, q, schoolID)
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
	svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, nil, nil)

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
			if r.URL.Path == "/audio/speech" {
				var body map[string]any
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
		svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := &models.InterviewSession{
			ID:                100,
			UserID:            1,
			InterviewerGender: "male",
			Language:          "ja",
		}
		sRepo.On("FindByID", uint(100)).Return(session, nil)

		_, err := svc.StartTurn(context.Background(), 1, 100, "", "", "", "", "", 0, 0, 0, 0, 0)
		assert.NoError(t, err)
	})

	t.Run("Female voice shimmer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/audio/speech" {
				var body map[string]any
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
		svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := &models.InterviewSession{
			ID:                101,
			UserID:            1,
			InterviewerGender: "female",
			Language:          "ja",
		}
		sRepo.On("FindByID", uint(101)).Return(session, nil)

		_, err := svc.StartTurn(context.Background(), 1, 101, "", "", "", "", "", 0, 0, 0, 0, 0)
		assert.NoError(t, err)
	})
}

// TestInterviewService_TurnDegradesGracefullyOnAPIFailure は #910 の回帰テスト。
// Transcribe/ChatInterview/TTS のいずれかがAPIエラーを返しても、Turn は
// エラーを返さず面接官としてのフォールバック応答で継続することを検証する。
func TestInterviewService_TurnDegradesGracefullyOnAPIFailure(t *testing.T) {
	newSession := func(id uint) *models.InterviewSession {
		return &models.InterviewSession{ID: id, UserID: 1, InterviewerGender: "female", Language: "ja"}
	}

	t.Run("Transcribe失敗時は聞き取れなかった扱いで継続する", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/audio/transcriptions":
				w.WriteHeader(http.StatusInternalServerError)
			case "/audio/speech":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("audio"))
			default:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"choices":[{"message":{"content":"次の質問です"}}],"text":"次の質問です"}`))
			}
		}))
		defer server.Close()

		sRepo := new(mockSessionRepo)
		uRepo := new(mockUserRepo)
		client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
		svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := newSession(200)
		sRepo.On("FindByID", uint(200)).Return(session, nil)

		result, err := svc.Turn(context.Background(), 1, 200, []byte("dummy-audio"), nil, "", "", "", "", "", 0, 1, 0, 1, 1, 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, "（聞き取れませんでした）", result.UserText)
	})

	t.Run("Chat失敗時は言い換え応答にフォールバックする", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/audio/transcriptions":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"text":"わかりません"}`))
			case "/chat/completions":
				w.WriteHeader(http.StatusInternalServerError)
			case "/audio/speech":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("audio"))
			}
		}))
		defer server.Close()

		sRepo := new(mockSessionRepo)
		uRepo := new(mockUserRepo)
		client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
		svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := newSession(201)
		sRepo.On("FindByID", uint(201)).Return(session, nil)

		result, err := svc.Turn(context.Background(), 1, 201, []byte("dummy-audio"), nil, "", "", "", "", "", 0, 1, 0, 1, 1, 0, 0)
		assert.NoError(t, err)
		assert.NotEmpty(t, result.AIText)
		assert.NotContains(t, result.AIText, "chat error")
	})

	t.Run("TTS失敗時は音声なしで継続する", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/audio/transcriptions":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"text":"よろしくお願いします"}`))
			case "/audio/speech":
				w.WriteHeader(http.StatusInternalServerError)
			default:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"choices":[{"message":{"content":"次の質問です"}}],"text":"次の質問です"}`))
			}
		}))
		defer server.Close()

		sRepo := new(mockSessionRepo)
		uRepo := new(mockUserRepo)
		client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
		svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, client, nil)

		session := newSession(202)
		sRepo.On("FindByID", uint(202)).Return(session, nil)

		result, err := svc.Turn(context.Background(), 1, 202, []byte("dummy-audio"), nil, "", "", "", "", "", 0, 1, 0, 1, 1, 0, 0)
		assert.NoError(t, err)
		assert.Empty(t, result.Audio)
	})
}

// TestInterviewService_EnsureSessionOwnership は #941 の回帰テスト。
// UploadVideo等がセッション所有者チェックに使う EnsureSessionOwnership の
// 許可/拒否ロジックをテーブル駆動で検証する。
func TestInterviewService_EnsureSessionOwnership(t *testing.T) {
	tests := []struct {
		name      string
		actorID   uint
		sessionID uint
		session   *models.InterviewSession
		actor     *entity.User
		wantErr   string // 空文字なら成功期待
	}{
		{
			name:      "本人のセッションは許可",
			actorID:   1,
			sessionID: 100,
			session:   &models.InterviewSession{ID: 100, UserID: 1},
			wantErr:   "",
		},
		{
			name:      "他人のセッションは拒否",
			actorID:   2,
			sessionID: 100,
			session:   &models.InterviewSession{ID: 100, UserID: 1},
			actor:     &entity.User{IsAdmin: false},
			wantErr:   "forbidden",
		},
		{
			name:      "管理者は他人のセッションでも許可",
			actorID:   2,
			sessionID: 100,
			session:   &models.InterviewSession{ID: 100, UserID: 1},
			actor:     &entity.User{IsAdmin: true},
			wantErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sRepo := new(mockSessionRepo)
			uRepo := new(mockUserRepo)
			svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, nil, nil)

			sRepo.On("FindByID", tt.sessionID).Return(tt.session, nil)
			if tt.actorID != tt.session.UserID {
				uRepo.On("GetUserByID", tt.actorID).Return(tt.actor, nil)
			}

			err := svc.EnsureSessionOwnership(tt.actorID, tt.sessionID)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}

	t.Run("セッションが存在しない場合はリポジトリのエラーをそのまま返す", func(t *testing.T) {
		sRepo := new(mockSessionRepo)
		uRepo := new(mockUserRepo)
		svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, nil, nil)

		sRepo.On("FindByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

		err := svc.EnsureSessionOwnership(1, 999)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}
