package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/api/applications", nil), rec)
			if err := h(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tt.wantStatus || called != tt.wantCalled {
				t.Fatalf("status=%d called=%v, want status=%d called=%v", rec.Code, called, tt.wantStatus, tt.wantCalled)
			}
		})
	}
}

func TestApplyAvailability(t *testing.T) {
	for env, want := range map[string]string{"": `{"enabled":false}`, "true": `{"enabled":true}`} {
		t.Setenv("STUDENT_APPLY_ENABLED", env)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/api/applications/availability", nil), rec)
		if err := applyAvailability(c); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != want {
			t.Fatalf("env=%q body=%s, want %s", env, got, want)
		}
	}
}
