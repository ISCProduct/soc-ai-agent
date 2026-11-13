package graduateoutcome

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/repositories"
	svc "Backend/internal/services/graduateoutcome"

	"github.com/labstack/echo/v4"
)

// #2032: 学生は認証から決める。学校はクエリで指定させない。

type fakeService struct {
	user   uint
	filter repositories.GraduateOutcomeFilter
}

func (f *fakeService) List(userID uint, filter repositories.GraduateOutcomeFilter) (*svc.List, error) {
	f.user, f.filter = userID, filter
	return &svc.List{}, nil
}
func (f *fakeService) Company(userID, companyID uint) (*svc.CompanyDetail, error) {
	f.user = userID
	return &svc.CompanyDetail{}, nil
}

func call(target string, userID uint, s Service) error {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if userID > 0 {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, userID))
	}
	return NewController(s).List(e.NewContext(req, httptest.NewRecorder()))
}

func TestList(t *testing.T) {
	s := &fakeService{}
	if err := call("/api/graduate-outcomes?year=2025&industry=%E8%A3%BD%E9%80%A0%E6%A5%AD&school_id=99&user_id=99", 7, s); err != nil {
		t.Fatalf("err = %v", err)
	}
	if s.user != 7 || s.filter.Year != 2025 || s.filter.Industry != "製造業" {
		t.Fatalf("user=%d filter=%+v", s.user, s.filter)
	}
	var he *echo.HTTPError
	if err := call("/api/graduate-outcomes?year=abc", 7, s); !errors.As(err, &he) || he.Code != http.StatusBadRequest {
		t.Fatalf("不正な年度: err = %v", err)
	}
	if err := call("/api/graduate-outcomes", 0, s); !errors.As(err, &he) || he.Code != http.StatusUnauthorized {
		t.Fatalf("未ログイン: err = %v", err)
	}
}
