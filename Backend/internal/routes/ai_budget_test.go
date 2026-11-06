package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/services/costs"
	"Backend/internal/usagectx"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

type fakeAIBudgetStatus struct {
	status costs.AIBudgetStatus
	err    error
	calls  int
}

func (f *fakeAIBudgetStatus) Status(context.Context, uint) (costs.AIBudgetStatus, error) {
	f.calls++
	return f.status, f.err
}

// runBudgetRoute は認証済みまたは未ログインの状態でミドルウェアを通し、ハンドラが呼ばれたかを返す。
func runBudgetRoute(t *testing.T, mw echo.MiddlewareFunc, userID uint, handler echo.HandlerFunc) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	e := echo.New()
	reached := false
	e.POST("/target", func(c echo.Context) error {
		reached = true
		if handler != nil {
			return handler(c)
		}
		return c.NoContent(http.StatusNoContent)
	}, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if userID > 0 {
				ctx := context.WithValue(c.Request().Context(), middleware.UserIDContextKey, userID)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}, mw)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/target", nil))
	return rec, reached
}

func TestEchoAIBudget(t *testing.T) {
	tests := []struct {
		name        string
		userID      uint
		service     *fakeAIBudgetStatus
		wantStatus  int
		wantReached bool
		wantCalls   int
	}{
		{
			name: "枠内ならハンドラへ進む", userID: 1, service: &fakeAIBudgetStatus{status: costs.AIBudgetStatus{Allowed: true}},
			wantStatus: http.StatusNoContent, wantReached: true, wantCalls: 1,
		},
		{
			name: "超過は429でハンドラへ進まない", userID: 1, service: &fakeAIBudgetStatus{},
			wantStatus: http.StatusTooManyRequests, wantCalls: 1,
		},
		{
			name: "判定できないときは503", userID: 1, service: &fakeAIBudgetStatus{err: errors.New("db down")},
			wantStatus: http.StatusServiceUnavailable, wantCalls: 1,
		},
		{
			name: "未ログインは判定しない", service: &fakeAIBudgetStatus{},
			wantStatus: http.StatusNoContent, wantReached: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, reached := runBudgetRoute(t, echoAIBudget(tt.service), tt.userID, nil)
			if rec.Code != tt.wantStatus || reached != tt.wantReached || tt.service.calls != tt.wantCalls {
				t.Fatalf("status=%d reached=%v calls=%d, want %d %v %d", rec.Code, reached, tt.service.calls, tt.wantStatus, tt.wantReached, tt.wantCalls)
			}
		})
	}
}

func TestEchoAIBudget_超過の応答にコードだけを返す(t *testing.T) {
	rec, _ := runBudgetRoute(t, echoAIBudget(&fakeAIBudgetStatus{}), 1, nil)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["code"] != httpapi.AIBudgetExceededCode {
		t.Fatalf("code = %v", body["code"])
	}
	// 学校の利用額は学生へ返さない。
	if _, ok := body["budget"]; ok || strings.Contains(rec.Body.String(), "used_usd") {
		t.Fatalf("response leaks usage: %s", rec.Body.String())
	}
}

func TestEchoAIBudget_サービス未設定はログイン利用者を止める(t *testing.T) {
	rec, reached := runBudgetRoute(t, echoAIBudget(nil), 1, nil)
	if rec.Code != http.StatusServiceUnavailable || reached {
		t.Fatalf("status=%d reached=%v", rec.Code, reached)
	}
}

func TestEchoDeferredAIBudget(t *testing.T) {
	t.Run("AIを呼ばないならハンドラは判定せずに返せる", func(t *testing.T) {
		service := &fakeAIBudgetStatus{}
		rec, reached := runBudgetRoute(t, echoDeferredAIBudget(service), 1, nil)
		if rec.Code != http.StatusNoContent || !reached || service.calls != 0 {
			t.Fatalf("status=%d reached=%v calls=%d", rec.Code, reached, service.calls)
		}
	})
	t.Run("AIを呼ぶ直前に超過を返す", func(t *testing.T) {
		service := &fakeAIBudgetStatus{}
		var got error
		runBudgetRoute(t, echoDeferredAIBudget(service), 1, func(c echo.Context) error {
			got = usagectx.CheckAIBudget(c.Request().Context())
			return c.NoContent(http.StatusNoContent)
		})
		if !errors.Is(got, usagectx.ErrAIBudgetExceeded) {
			t.Fatalf("CheckAIBudget = %v", got)
		}
	})
	t.Run("判定できないときは利用不可を返す", func(t *testing.T) {
		service := &fakeAIBudgetStatus{err: errors.New("db down")}
		var got error
		runBudgetRoute(t, echoDeferredAIBudget(service), 1, func(c echo.Context) error {
			got = usagectx.CheckAIBudget(c.Request().Context())
			return c.NoContent(http.StatusNoContent)
		})
		if !errors.Is(got, usagectx.ErrAIBudgetUnavailable) {
			t.Fatalf("CheckAIBudget = %v", got)
		}
	})
	t.Run("未ログインは判定を載せない", func(t *testing.T) {
		service := &fakeAIBudgetStatus{}
		var got error
		runBudgetRoute(t, echoDeferredAIBudget(service), 0, func(c echo.Context) error {
			got = usagectx.CheckAIBudget(c.Request().Context())
			return c.NoContent(http.StatusNoContent)
		})
		if got != nil || service.calls != 0 {
			t.Fatalf("CheckAIBudget = %v calls=%d", got, service.calls)
		}
	})
}

// 予算判定が付くルートを固定する。実際のルート登録へリクエストを送るので、
// Setup 関数からガードが外れるとここで落ちる。
func TestAIBudgetGuardsAreAttachedToRoutes(t *testing.T) {
	const secret = "test-secret"
	const immediate, deferred = 450, 451
	guard := func(code int) echo.MiddlewareFunc {
		return func(echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				// 認証の後段で動くこと（利用者が載っていること）も確かめる。
				if _, ok := httpapi.UserID(c); !ok {
					return c.NoContent(http.StatusUnauthorized)
				}
				return c.NoContent(code)
			}
		}
	}
	budget := AIBudgetGuards{Immediate: guard(immediate), Deferred: guard(deferred)}

	e := echo.New()
	e.Use(echomw.Recover())
	api := e.Group("/api")
	SetupChatRoutes(api, nil, nil, secret, nil, nil, budget)
	SetupCompanyRoutes(api, nil, secret, nil, nil, budget)
	SetupResumeRoutes(api, nil, secret, nil, nil, budget)
	SetupInterviewRoutes(api, nil, nil, secret, nil, nil, budget)
	SetupGitHubRoutes(api, nil, secret, nil, nil, budget)
	SetupESRoutes(api, nil, nil, secret, nil, nil, budget)
	SetupAssessmentRoutes(api, nil, secret, nil, nil, budget)

	token, err := middleware.GenerateJWT(1, "user@example.com", secret)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPost, "/api/chat", immediate},
		{http.MethodGet, "/api/chat/analysis", immediate},
		{http.MethodPost, "/api/chat/send-report", immediate},
		{http.MethodPost, "/api/questions/generate", immediate},
		{http.MethodPost, "/api/resume/review", immediate},
		{http.MethodPost, "/api/resume/review/stream", immediate},
		{http.MethodPost, "/api/es/review", immediate},
		{http.MethodPost, "/api/es/rewrite", immediate},
		{http.MethodPost, "/api/realtime/token", immediate},
		{http.MethodPost, "/api/interviews/1/start", immediate},
		{http.MethodPost, "/api/interviews/1/report/regenerate", immediate},
		{http.MethodGet, "/api/interviews/1/phrase-suggestions", immediate},
		{http.MethodGet, "/api/companies/web-search", deferred},
		{http.MethodPost, "/api/companies/validate", deferred},
		{http.MethodPost, "/api/github/repo/summarize", deferred},
		{http.MethodPost, "/api/assessments/sessions/1/submit", deferred},
		// 保存済みのレポートをメールで送るだけで AI を呼ばない。
		{http.MethodPost, "/api/interviews/1/send-report", 0},
		// 開始後の面接は予算超過で中断しない。ガードに当たらなければよい（want は 0）。
		// ハンドラが nil なので、ガードを通り抜けた先は 400 や Recover の 500 になる。
		{http.MethodPost, "/api/interviews/1/utterances", 0},
		{http.MethodPost, "/api/interviews/1/finish", 0},
		{http.MethodPost, "/api/interviews/1/turn", 0},
		{http.MethodPost, "/api/interviews/1/start-turn", 0},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("X-User-Token", token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if tt.want == 0 {
				if rec.Code == immediate || rec.Code == deferred {
					t.Fatalf("status = %d, the route must not have a budget guard", rec.Code)
				}
				return
			}
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
