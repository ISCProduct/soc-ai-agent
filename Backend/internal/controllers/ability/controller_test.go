package ability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/internal/middleware"
	"Backend/internal/models"
	abilitysvc "Backend/internal/services/ability"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	ownSession     = "11111111-1111-1111-1111-111111111111"
	doneSession    = "22222222-2222-2222-2222-222222222222"
	expiredSession = "33333333-3333-3333-3333-333333333333"
	missingSession = "44444444-4444-4444-4444-444444444444"
)

// fakeRepo はコントローラの経路で呼ばれる分だけを実装する（他は埋め込みの nil で panic）。
type fakeRepo struct {
	abilitysvc.Repository
	answers map[uint]int
}

func (f *fakeRepo) FindSessionByPublicID(userID uint, publicID string) (*models.AbilitySession, error) {
	if userID != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)
	switch publicID {
	case ownSession:
		return &models.AbilitySession{ID: 10, PublicID: ownSession, UserID: 1, TemplateID: 1, TemplateVersion: 1,
			Status: models.AbilityStatusInProgress, DeadlineAt: &future}, nil
	case doneSession:
		return &models.AbilitySession{ID: 20, PublicID: doneSession, UserID: 1, TemplateID: 1, TemplateVersion: 1,
			Status: models.AbilityStatusSubmitted}, nil
	case expiredSession:
		return &models.AbilitySession{ID: 30, PublicID: expiredSession, UserID: 1, TemplateID: 1, TemplateVersion: 1,
			Status: models.AbilityStatusInProgress, DeadlineAt: &past}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepo) FindTemplate(id uint) (*models.AbilityTemplate, error) {
	limit := 900
	return &models.AbilityTemplate{ID: id, Title: "能力検査", Version: 1, TimeLimitSeconds: &limit}, nil
}

func (f *fakeRepo) ListQuestions(uint, int) ([]models.AbilityQuestion, error) {
	return []models.AbilityQuestion{
		{ID: 1, Section: models.AbilitySectionVerbal, Body: "語彙", Choices: `["a","b","c","d"]`, CorrectChoice: 0},
		{ID: 2, Section: models.AbilitySectionNumerical, Body: "計算", Choices: `["a","b","c","d"]`, CorrectChoice: 1},
	}, nil
}

func (f *fakeRepo) UpsertAnswer(_, questionID uint, choice int) error {
	f.answers[questionID] = choice
	return nil
}

func (f *fakeRepo) ListAnswers(uint) ([]models.AbilityAnswer, error) {
	out := []models.AbilityAnswer{}
	for q, v := range f.answers {
		out = append(out, models.AbilityAnswer{QuestionID: q, Choice: v})
	}
	return out, nil
}

func (f *fakeRepo) Submit(s *models.AbilitySession, score func(map[uint]int) (map[string]any, error)) error {
	now := time.Now()
	s.Status = models.AbilityStatusSubmitted
	s.SubmittedAt = &now
	// 本物はトランザクション内で回答を読んで採点する。ここでも同じ順序で呼ぶ。
	_, err := score(f.answers)
	return err
}

func (f *fakeRepo) FindResult(uint) (map[string]any, error) {
	return map[string]any{"verbal": map[string]any{"correct": float64(1), "total": float64(1)}}, nil
}

func newServer(t *testing.T, userID uint) (*echo.Echo, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{answers: map[uint]int{}}
	c := NewController(abilitysvc.NewService(repo))
	e := echo.New()
	g := e.Group("/api/ability-assessments", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if userID != 0 {
				r := ctx.Request()
				ctx.SetRequest(r.WithContext(context.WithValue(r.Context(), middleware.UserIDContextKey, userID)))
			}
			return next(ctx)
		}
	})
	g.GET("/sessions/:sessionId", c.Get)
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
	const base = "/api/ability-assessments/sessions/"
	tests := []struct {
		name   string
		userID uint
		method string
		path   string
		body   string
		want   int
	}{
		{"未ログインは401", 0, http.MethodPost, base + ownSession + "/submit", "", http.StatusUnauthorized},
		{"UUIDでないセッションIDは400", 1, http.MethodPost, base + "12/submit", "", http.StatusBadRequest},
		{"他人のセッションは404", 2, http.MethodGet, base + ownSession, "", http.StatusNotFound},
		{"存在しないセッションは404", 1, http.MethodGet, base + missingSession, "", http.StatusNotFound},
		{"提出済みの再提出は409", 1, http.MethodPost, base + doneSession + "/submit", "", http.StatusConflict},
		{"提出済みへの回答は409", 1, http.MethodPut, base + doneSession + "/answers/1", `{"choice":0}`, http.StatusConflict},
		// 410: 制限時間切れ。404(無い)・409(提出済み)と区別して伝える。
		{"制限時間切れの取得は410", 1, http.MethodGet, base + expiredSession, "", http.StatusGone},
		{"制限時間切れへの回答は410", 1, http.MethodPut, base + expiredSession + "/answers/1", `{"choice":0}`, http.StatusGone},
		// 時間切れでも提出はできる（これまでの回答で採点する）
		{"制限時間切れでも提出は200", 1, http.MethodPost, base + expiredSession + "/submit", "", http.StatusOK},
		{"選択肢の範囲外は400", 1, http.MethodPut, base + ownSession + "/answers/1", `{"choice":9}`, http.StatusBadRequest},
		{"負の添字は400", 1, http.MethodPut, base + ownSession + "/answers/1", `{"choice":-1}`, http.StatusBadRequest},
		{"検査に無い問題への回答は400", 1, http.MethodPut, base + ownSession + "/answers/99", `{"choice":0}`, http.StatusBadRequest},
		{"choice 未指定は400", 1, http.MethodPut, base + ownSession + "/answers/1", `{}`, http.StatusBadRequest},
		{"choice=0 は正当な回答として受ける", 1, http.MethodPut, base + ownSession + "/answers/1", `{"choice":0}`, http.StatusNoContent},
		{"未提出の結果は404", 1, http.MethodGet, base + ownSession + "/result", "", http.StatusNotFound},
		{"提出済みの結果は200", 1, http.MethodGet, base + doneSession + "/result", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newServer(t, tt.userID)
			if got := do(e, tt.method, tt.path, tt.body).Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

// 受検中のレスポンスに正解を出さない。HTTP の出口でも確かめる
// （ビュー型のテストと二重だが、詰め替えを挟む層が増えても落ちるようにする）。
func TestController_GetHidesCorrectChoice(t *testing.T) {
	e, _ := newServer(t, 1)
	rec := do(e, http.MethodGet, "/api/ability-assessments/sessions/"+ownSession, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, key := range []string{"correct_choice", "CorrectChoice"} {
		if strings.Contains(body, key) {
			t.Errorf("レスポンスに %q が出た: %s", key, body)
		}
	}
	if !strings.Contains(body, "choices") {
		t.Errorf("選択肢が返っていない: %s", body)
	}
}
