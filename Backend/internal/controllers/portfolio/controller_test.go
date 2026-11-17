package portfolio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"Backend/internal/middleware"
	portfoliosvc "Backend/internal/services/portfolio"

	"github.com/labstack/echo/v4"
)

// #2030: ポートフォリオの API は、宛先を認証から決める。

type fakeBuilder struct {
	user     uint
	sections []string
}

func (f *fakeBuilder) Build(userID uint, sections []string) (*portfoliosvc.Portfolio, error) {
	f.user, f.sections = userID, sections
	return &portfoliosvc.Portfolio{Sections: sections}, nil
}

func call(target string, userID uint, b Builder) (*httptest.ResponseRecorder, error) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if userID > 0 {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, userID))
	}
	rec := httptest.NewRecorder()
	return rec, NewController(b).Get(e.NewContext(req, rec))
}

func TestGet_クエリのuser_idは使わない(t *testing.T) {
	b := &fakeBuilder{}
	rec, err := call("/api/portfolio?user_id=999&sections=es,interview", 7, b)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if b.user != 7 || !slices.Equal(b.sections, []string{"es", "interview"}) {
		t.Fatalf("user=%d sections=%v", b.user, b.sections)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("個人情報を含む応答をキャッシュさせない")
	}
}

func TestGet_未ログインと不正な指定(t *testing.T) {
	var he *echo.HTTPError
	if _, err := call("/api/portfolio", 0, &fakeBuilder{}); !errors.As(err, &he) || he.Code != http.StatusUnauthorized {
		t.Fatalf("未ログイン: err = %v", err)
	}
	if _, err := call("/api/portfolio?sections=unknown", 7, &fakeBuilder{}); !errors.As(err, &he) || he.Code != http.StatusBadRequest {
		t.Fatalf("知らない内容だけ: err = %v", err)
	}
}
