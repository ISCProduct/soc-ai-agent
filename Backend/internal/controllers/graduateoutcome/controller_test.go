package graduateoutcome

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	svc "Backend/internal/services/graduateoutcome"

	"github.com/labstack/echo/v4"
)

// #2032: 学生は認証から決める。学校はクエリで指定させない。

type fakeService struct {
	user    uint
	company uint
	filter  svc.Filter
	err     error
}

func (f *fakeService) List(userID uint, filter svc.Filter) (*svc.List, error) {
	f.user, f.filter = userID, filter
	if f.err != nil {
		return nil, f.err
	}
	return &svc.List{}, nil
}
func (f *fakeService) Company(userID, companyID uint) (*svc.CompanyDetail, error) {
	f.user, f.company = userID, companyID
	if f.err != nil {
		return nil, f.err
	}
	return &svc.CompanyDetail{}, nil
}

func call(target string, userID uint, s Service, company bool, id string) error {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if userID > 0 {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, userID))
	}
	ctx := e.NewContext(req, httptest.NewRecorder())
	if company {
		ctx.SetParamNames("id")
		ctx.SetParamValues(id)
		return NewController(s).Company(ctx)
	}
	return NewController(s).List(ctx)
}

func code(err error) int {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he.Code
	}
	if err == nil {
		return http.StatusOK
	}
	return -1
}

func TestList(t *testing.T) {
	s := &fakeService{}
	if err := call("/api/graduate-outcomes?year=2025&industry=%E8%A3%BD%E9%80%A0%E6%A5%AD&school_id=99&user_id=99", 7, s, false, ""); err != nil {
		t.Fatalf("err = %v", err)
	}
	if s.user != 7 || s.filter.Year != 2025 || s.filter.Industry != "製造業" {
		t.Fatalf("user=%d filter=%+v", s.user, s.filter)
	}
	if got := code(call("/api/graduate-outcomes", 0, s, false, "")); got != http.StatusUnauthorized {
		t.Fatalf("未ログイン: %d", got)
	}
}

// 年度の境界: 1900〜2100 だけ受け付ける。
func TestList_年度の境界(t *testing.T) {
	for _, tt := range []struct {
		year string
		want int
	}{{"abc", 400}, {"1899", 400}, {"1900", 200}, {"2100", 200}, {"2101", 400}} {
		if got := code(call("/api/graduate-outcomes?year="+tt.year, 7, &fakeService{}, false, "")); got != tt.want {
			t.Errorf("year=%s: %d, want %d", tt.year, got, tt.want)
		}
	}
}

func TestCompany(t *testing.T) {
	s := &fakeService{}
	if err := call("/api/graduate-outcomes/companies/12", 7, s, true, "12"); err != nil || s.user != 7 || s.company != 12 {
		t.Fatalf("err=%v user=%d company=%d", err, s.user, s.company)
	}
	for _, id := range []string{"0", "-1", "abc", ""} {
		if got := code(call("/api/graduate-outcomes/companies/x", 7, &fakeService{}, true, id)); got != http.StatusBadRequest {
			t.Errorf("id=%q: %d", id, got)
		}
	}
	if got := code(call("/api/graduate-outcomes/companies/12", 0, s, true, "12")); got != http.StatusUnauthorized {
		t.Errorf("未ログイン: %d", got)
	}
}

// 学生が存在しなければ 401、企業が無ければ 404、それ以外の失敗は 500。
func TestErrorMapping(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want int
	}{{svc.ErrUserNotFound, 401}, {svc.ErrCompanyNotFound, 404}, {errors.New("db"), 500}} {
		if got := code(call("/api/graduate-outcomes/companies/12", 7, &fakeService{err: tt.err}, true, "12")); got != tt.want {
			t.Errorf("%v: %d, want %d", tt.err, got, tt.want)
		}
		if tt.want == 404 {
			continue
		}
		if got := code(call("/api/graduate-outcomes", 7, &fakeService{err: tt.err}, false, "")); got != tt.want {
			t.Errorf("list %v: %d, want %d", tt.err, got, tt.want)
		}
	}
}
