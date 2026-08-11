package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/models"
	openaiPkg "Backend/internal/openai"
	"Backend/internal/services"

	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repositories ────────────────────────────────────────────────────────

type mockInterviewSessionRepo struct {
	session *models.InterviewSession
	err     error
}

func (m *mockInterviewSessionRepo) Create(s *models.InterviewSession) error { return nil }
func (m *mockInterviewSessionRepo) FindByID(id uint) (*models.InterviewSession, error) {
	return m.session, m.err
}
func (m *mockInterviewSessionRepo) Update(s *models.InterviewSession) error { return nil }
func (m *mockInterviewSessionRepo) ListByUser(userID uint, limit, offset int) ([]models.InterviewSession, error) {
	return nil, nil
}
func (m *mockInterviewSessionRepo) ListAll(limit, offset int, schoolID *uint) ([]models.InterviewSession, error) {
	return nil, nil
}
func (m *mockInterviewSessionRepo) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	return nil, nil
}
func (m *mockInterviewSessionRepo) CountByUser(userID uint) (int64, error) { return 0, nil }
func (m *mockInterviewSessionRepo) CountAll(schoolID *uint) (int64, error) { return 0, nil }
func (m *mockInterviewSessionRepo) CountByUserAndDay(userID uint, day time.Time) (int64, error) {
	return 0, nil
}

type mockInterviewUtterRepo struct {
	utterances []models.InterviewUtterance
	err        error
}

func (m *mockInterviewUtterRepo) Create(u *models.InterviewUtterance) error { return nil }
func (m *mockInterviewUtterRepo) FindBySessionID(sessionID uint) ([]models.InterviewUtterance, error) {
	return m.utterances, m.err
}

// mockUserRepo は repository.UserRepository の最小モック実装。
type mockUserRepo2 struct {
	user *entity.User
	err  error
}

func (m *mockUserRepo2) GetUserByID(id uint) (*entity.User, error)         { return m.user, m.err }
func (m *mockUserRepo2) CreateUser(u *entity.User) error                   { return nil }
func (m *mockUserRepo2) GetUserByEmail(email string) (*entity.User, error) { return nil, nil }
func (m *mockUserRepo2) ListUsers() ([]entity.User, error)                 { return nil, nil }
func (m *mockUserRepo2) ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]entity.User, int64, error) {
	return nil, 0, nil
}
func (m *mockUserRepo2) UpdateUser(u *entity.User) error { return nil }
func (m *mockUserRepo2) DeleteUser(id uint) error        { return nil }
func (m *mockUserRepo2) GetUserByVerificationToken(token string) (*entity.User, error) {
	return nil, nil
}
func (m *mockUserRepo2) GetUserByPasswordResetToken(token string) (*entity.User, error) {
	return nil, nil
}
func (m *mockUserRepo2) GetUserByOAuth(provider, oauthID string) (*entity.User, error) {
	return nil, nil
}

// ─── ヘルパー ─────────────────────────────────────────────────────────────────

// newTestInterviewService はモックリポジトリとOpenAIテストサーバーを注入したサービスを返す。
func newTestInterviewService(
	sessionRepo *mockInterviewSessionRepo,
	utterRepo *mockInterviewUtterRepo,
	userRepo *mockUserRepo2,
	aiClient *openaiPkg.Client,
) *services.InterviewService {
	return services.NewInterviewService(sessionRepo, utterRepo, nil, userRepo, nil, aiClient, nil)
}

// newOpenAITestServer はOpenAI Chat Completions レスポンスを返すテストHTTPサーバーを起動する。
func newOpenAITestServer(t *testing.T, responseBody string) (*httptest.Server, *openaiPkg.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := openai.ChatCompletionResponse{
			ID:    "test-id",
			Model: "gpt-4o",
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: responseBody,
					},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, openaiPkg.NewWithBaseURL(srv.URL, "gpt-4o")
}

// ─── BuildTranscript ─────────────────────────────────────────────────────────

func TestBuildTranscript_Empty(t *testing.T) {
	result := services.BuildTranscript(nil)
	assert.Equal(t, "", result)
}

func TestBuildTranscript_UserOnly(t *testing.T) {
	utterances := []models.InterviewUtterance{
		{Role: "user", Text: "チームでの開発経験があります。"},
		{Role: "user", Text: "リーダーを担当しました。"},
	}
	result := services.BuildTranscript(utterances)
	assert.Contains(t, result, "User: チームでの開発経験があります。")
	assert.Contains(t, result, "User: リーダーを担当しました。")
}

func TestBuildTranscript_MixedRoles(t *testing.T) {
	utterances := []models.InterviewUtterance{
		{Role: "ai", Text: "自己紹介をお願いします。"},
		{Role: "user", Text: "田中と申します。"},
		{Role: "ai", Text: "志望動機を聞かせてください。"},
		{Role: "user", Text: "御社に興味があります。"},
	}
	result := services.BuildTranscript(utterances)
	assert.Contains(t, result, "Interviewer: 自己紹介をお願いします。")
	assert.Contains(t, result, "User: 田中と申します。")
	assert.Contains(t, result, "Interviewer: 志望動機を聞かせてください。")
	assert.Contains(t, result, "User: 御社に興味があります。")
}

func TestBuildTranscript_TrimsWhitespace(t *testing.T) {
	utterances := []models.InterviewUtterance{
		{Role: "user", Text: "  スペースあり  "},
	}
	result := services.BuildTranscript(utterances)
	assert.Contains(t, result, "User: スペースあり")
}

// ─── ExtractJSONObject ────────────────────────────────────────────────────────

func TestExtractJSONObject_Clean(t *testing.T) {
	input := `{"suggestions": [{"original": "頑張りました", "suggestions": ["尽力しました"]}]}`
	result := services.ExtractJSONObject(input)
	assert.Equal(t, input, result)
}

func TestExtractJSONObject_WithMarkdownFence(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	result := services.ExtractJSONObject(input)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestExtractJSONObject_WithLeadingText(t *testing.T) {
	// ExtractJSONObject は先頭のテキストと末尾のテキストを両方除去する
	input := `以下がJSONです: {"key": "value"} 終わり`
	result := services.ExtractJSONObject(input)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestExtractJSONObject_Empty(t *testing.T) {
	result := services.ExtractJSONObject("")
	assert.Equal(t, "", result)
}

// ─── GetPhraseSuggestions ─────────────────────────────────────────────────────

func TestGetPhraseSuggestions_Forbidden(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 99}, // 別ユーザーのセッション
		err:     nil,
	}
	utterRepo := &mockInterviewUtterRepo{}
	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{err: errors.New("not found")}, nil)

	// userID=1 が ownerID=99 のセッションにアクセス → forbidden
	_, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.Error(t, err)
	assert.Equal(t, "forbidden", err.Error())
}

func TestGetPhraseSuggestions_SessionNotFound(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: nil,
		err:     errors.New("record not found"),
	}
	utterRepo := &mockInterviewUtterRepo{}
	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{err: errors.New("not found")}, nil)

	_, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.Error(t, err)
	assert.Equal(t, "record not found", err.Error())
}

func TestGetPhraseSuggestions_NoUserUtterances(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 1},
	}
	// AI発話のみ、ユーザー発話なし
	utterRepo := &mockInterviewUtterRepo{
		utterances: []models.InterviewUtterance{
			{Role: "ai", Text: "自己紹介をお願いします。"},
		},
	}
	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{err: errors.New("not found")}, nil)

	result, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.NoError(t, err)
	assert.Empty(t, result, "ユーザー発話がなければ空スライスを返す")
}

func TestGetPhraseSuggestions_UtteranceFetchError(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 1},
	}
	utterRepo := &mockInterviewUtterRepo{
		err: errors.New("db connection error"),
	}
	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{err: errors.New("not found")}, nil)

	_, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.Error(t, err)
	assert.Equal(t, "db connection error", err.Error())
}

func TestGetPhraseSuggestions_Success(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 1},
	}
	utterances := []models.InterviewUtterance{
		{Role: "ai", Text: "自己PRをお願いします。"},
		{Role: "user", Text: "チームに貢献できたと思います。"},
		{Role: "ai", Text: "具体的には？"},
		{Role: "user", Text: "なんとなく頑張りました。"},
	}
	utterRepo := &mockInterviewUtterRepo{utterances: utterances}

	mockJSON := `{"suggestions": [{"original": "なんとなく頑張りました", "suggestions": ["KPIを20%改善しました", "週次レビューを主導しました"]}]}`
	_, aiClient := newOpenAITestServer(t, mockJSON)

	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{}, aiClient)

	result, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "なんとなく頑張りました", result[0].Original)
	assert.Equal(t, []string{"KPIを20%改善しました", "週次レビューを主導しました"}, result[0].Suggestions)
}

func TestGetPhraseSuggestions_MultipleSuggestions(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 2, UserID: 2},
	}
	utterances := []models.InterviewUtterance{
		{Role: "user", Text: "いろいろ経験しました。"},
		{Role: "user", Text: "なんとかやりました。"},
	}
	utterRepo := &mockInterviewUtterRepo{utterances: utterances}

	mockJSON := `{"suggestions": [
		{"original": "いろいろ経験しました", "suggestions": ["Reactを用いたSPA開発を3件担当しました", "5名のチームでスクラム開発を実践しました"]},
		{"original": "なんとかやりました", "suggestions": ["期限内に全タスクを完了しました", "障害対応をリードし復旧時間を50%短縮しました"]}
	]}`
	_, aiClient := newOpenAITestServer(t, mockJSON)

	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{}, aiClient)

	result, err := svc.GetPhraseSuggestions(context.Background(), 2, 2)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "いろいろ経験しました", result[0].Original)
	assert.Len(t, result[0].Suggestions, 2)
	assert.Equal(t, "なんとかやりました", result[1].Original)
	assert.Len(t, result[1].Suggestions, 2)
}

func TestGetPhraseSuggestions_InvalidJSON(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 1},
	}
	utterRepo := &mockInterviewUtterRepo{
		utterances: []models.InterviewUtterance{
			{Role: "user", Text: "頑張りました。"},
		},
	}
	// 不正なJSONをOpenAIが返した場合
	_, aiClient := newOpenAITestServer(t, `not a valid json`)

	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{}, aiClient)

	_, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse suggestions")
}

func TestGetPhraseSuggestions_EmptySuggestionsArray(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 1},
	}
	utterRepo := &mockInterviewUtterRepo{
		utterances: []models.InterviewUtterance{
			{Role: "user", Text: "素晴らしい経験です。"},
		},
	}
	// 提案なし（全て良い表現）
	_, aiClient := newOpenAITestServer(t, `{"suggestions": []}`)

	svc := newTestInterviewService(sessionRepo, utterRepo, &mockUserRepo2{}, aiClient)

	result, err := svc.GetPhraseSuggestions(context.Background(), 1, 1)
	require.NoError(t, err)
	assert.Empty(t, result)
}
