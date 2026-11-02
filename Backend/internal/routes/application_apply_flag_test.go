package routes

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationcontrollers "Backend/internal/controllers/application"
	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// 応募を閉じている間は新規登録を止め、開いたら通すこと（SOCAIAGENT-403）。
func TestRequireStudentApply(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		wantStatus int
		wantCalled bool
	}{
		{"未設定は閉じている", "", http.StatusForbidden, false},
		{"falseは閉じている", "false", http.StatusForbidden, false},
		{"解釈できない値は閉じている", "yes please", http.StatusForbidden, false},
		{"trueで開く", "true", http.StatusOK, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STUDENT_APPLY_ENABLED", tt.env)
			called := false
			h := requireStudentApply(func(c echo.Context) error {
				called = true
				return c.NoContent(http.StatusOK)
			})
			c := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/api/applications", nil), httptest.NewRecorder())
			err := h(c)
			status := http.StatusOK
			var he *echo.HTTPError
			if errors.As(err, &he) {
				status = he.Code
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status != tt.wantStatus || called != tt.wantCalled {
				t.Fatalf("status=%d called=%v, want status=%d called=%v", status, called, tt.wantStatus, tt.wantCalled)
			}
		})
	}
}

func TestApplyAvailability(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"未設定は閉じている", "", `{"enabled":false}`},
		{"falseは閉じている", "false", `{"enabled":false}`},
		{"解釈できない値は閉じている", "yes please", `{"enabled":false}`},
		{"前後の空白は無視してtrueで開く", " true ", `{"enabled":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STUDENT_APPLY_ENABLED", tt.env)
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/api/applications/availability", nil), rec)
			if err := applyAvailability(c); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.want {
				t.Fatalf("body=%s, want %s", got, tt.want)
			}
		})
	}
}

// ミドルウェアを付け忘れる退行を検出する。コントローラは空のまま渡すので、
// 止められなければ nil のサービスを呼んで panic する。
func TestSetupApplicationRoutes_POSTは閉じている間403(t *testing.T) {
	const secret = "test-user-secret"
	token, err := middleware.GenerateJWT(1, "student@example.com", secret)
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	SetupApplicationRoutes(e.Group("/api"), &applicationcontrollers.ApplicationController{}, nil, secret, nil, nil)

	t.Setenv("STUDENT_APPLY_ENABLED", "")
	send := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("X-User-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := send(http.MethodPost, "/api/applications"); rec.Code != http.StatusForbidden {
		t.Fatalf("POST /api/applications status=%d, want 403", rec.Code)
	}
	rec := send(http.MethodGet, "/api/applications/availability")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("GET availability status=%d body=%s", rec.Code, rec.Body.String())
	}
}
