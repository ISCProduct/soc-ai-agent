package assessment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	assessmentsvc "Backend/internal/services/assessment"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

const (
	ownSession   = "11111111-1111-1111-1111-111111111111"
	doneSession  = "22222222-2222-2222-2222-222222222222"
	otherSession = "33333333-3333-3333-3333-333333333333"
)

// fakeRepo はコントローラの経路で呼ばれる分だけを実装する（他は埋め込みの nil で panic）。
type fakeRepo struct {
	assessmentsvc.Repository
	answers map[uint]int
}

func (f *fakeRepo) FindSessionByPublicID(userID uint, publicID string) (*models.AssessmentSession, error) {
	if userID != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	switch publicID {
	case ownSession:
		return &models.AssessmentSession{ID: 10, PublicID: ownSession, UserID: 1, TemplateID: 1, TemplateVersion: 1, Status: models.AssessmentStatusInProgress}, nil
	case doneSession:
		return &models.AssessmentSession{ID: 20, PublicID: doneSession, UserID: 1, TemplateID: 1, TemplateVersion: 1, Status: models.AssessmentStatusSubmitted}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// 受検の入口（Service.session）は、学校独自テンプレートの受検を現在の所属校と
// 照合するためにテンプレートと所属校を引く（#1675）。ここでは運営標準（school_id は NULL）
// を返し、学校の照合に入らない経路を再現する。
func (f *fakeRepo) FindTemplate(id uint) (*models.AssessmentTemplate, error) {
	return &models.AssessmentTemplate{ID: id, Version: 1, IsStandard: true}, nil
}

func (f *fakeRepo) FindUserSchoolID(uint) (*uint, error) { return nil, nil }

func (f *fakeRepo) ListQuestions(uint, int) ([]models.AssessmentQuestion, error) {
	return []models.AssessmentQuestion{
		{ID: 1, WeightCategory: "チームワーク志向"},
		{ID: 2, WeightCategory: "チームワーク志向", ReverseScored: true},
	}, nil
}

func (f *fakeRepo) UpsertAnswer(_, questionID uint, value int) error {
	f.answers[questionID] = value
	return nil
}

func (f *fakeRepo) ListAnswers(uint) ([]models.AssessmentAnswer, error) {
	out := []models.AssessmentAnswer{}
	for q, v := range f.answers {
		out = append(out, models.AssessmentAnswer{QuestionID: q, Value: v})
	}
	return out, nil
}

func newServer(t *testing.T, userID uint) (*echo.Echo, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{answers: map[uint]int{}}
	c := NewController(assessmentsvc.NewService(repo, nil, nil))
	e := echo.New()
	g := e.Group("/api/assessments", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if userID != 0 {
				r := ctx.Request()
				ctx.SetRequest(r.WithContext(context.WithValue(r.Context(), middleware.UserIDContextKey, userID)))
			}
			return next(ctx)
		}
	})
	g.PUT("/sessions/:sessionId/answers/:questionId", c.SaveAnswer)
	g.POST("/sessions/:sessionId/submit", c.Submit)
	g.GET("/sessions/:sessionId/result", c.Result)
	return e, repo
}

func do(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestController_StatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		userID uint
		method string
		path   string
		body   string
		want   int
	}{
		{"未ログインは401", 0, http.MethodPost, "/api/assessments/sessions/" + ownSession + "/submit", "", http.StatusUnauthorized},
		{"UUIDでないセッションIDは400", 1, http.MethodPost, "/api/assessments/sessions/12/submit", "", http.StatusBadRequest},
		{"他人のセッションは404", 2, http.MethodPost, "/api/assessments/sessions/" + ownSession + "/submit", "", http.StatusNotFound},
		{"存在しないセッションは404", 1, http.MethodGet, "/api/assessments/sessions/" + otherSession + "/result", "", http.StatusNotFound},
		{"提出済みの再提出は409", 1, http.MethodPost, "/api/assessments/sessions/" + doneSession + "/submit", "", http.StatusConflict},
		{"提出済みへの回答は409", 1, http.MethodPut, "/api/assessments/sessions/" + doneSession + "/answers/1", `{"value":3}`, http.StatusConflict},
		{"未回答があると提出は400", 1, http.MethodPost, "/api/assessments/sessions/" + ownSession + "/submit", "", http.StatusBadRequest},
		{"範囲外の回答は400", 1, http.MethodPut, "/api/assessments/sessions/" + ownSession + "/answers/1", `{"value":6}`, http.StatusBadRequest},
		{"検査に無い問題への回答は400", 1, http.MethodPut, "/api/assessments/sessions/" + ownSession + "/answers/99", `{"value":3}`, http.StatusBadRequest},
		{"未提出の結果は404", 1, http.MethodGet, "/api/assessments/sessions/" + ownSession + "/result", "", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newServer(t, tt.userID)
			rec := do(e, tt.method, tt.path, tt.body)
			assert.Equal(t, tt.want, rec.Code, rec.Body.String())
		})
	}
}

func TestController_SaveAnswerTwiceKeepsLatest(t *testing.T) {
	e, repo := newServer(t, 1)
	path := "/api/assessments/sessions/" + ownSession + "/answers/1"
	assert.Equal(t, http.StatusNoContent, do(e, http.MethodPut, path, `{"value":2}`).Code)
	assert.Equal(t, http.StatusNoContent, do(e, http.MethodPut, path, `{"value":2}`).Code)
	assert.Equal(t, http.StatusNoContent, do(e, http.MethodPut, path, `{"value":4}`).Code)
	assert.Equal(t, map[uint]int{1: 4}, repo.answers)
}
