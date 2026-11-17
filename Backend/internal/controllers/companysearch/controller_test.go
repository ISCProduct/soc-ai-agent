package companysearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

// SOCAIAGENT-612: 学生向けの企業検索。

type fakeStore struct {
	got      repositories.CompanySearchFilter
	favUser  uint
	favIDs   []uint
	err      error
	searched bool
}

func (f *fakeStore) Search(filter repositories.CompanySearchFilter) ([]repositories.CompanySearchRow, int64, error) {
	f.got, f.searched = filter, true
	return nil, 0, f.err
}
func (f *fakeStore) SearchOptions() ([]string, []repositories.JobCategoryOption, error) {
	return nil, nil, f.err
}
func (f *fakeStore) FavoriteStates(userID uint, ids []uint) ([]repositories.FavoriteState, error) {
	f.favUser, f.favIDs = userID, ids
	return []repositories.FavoriteState{}, f.err
}

func call(h func(echo.Context) error, target string, userID uint) (int, string, error) {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if userID > 0 {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, userID))
	}
	rec := httptest.NewRecorder()
	err := h(echo.New().NewContext(req, rec))
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he.Code, "", err
	}
	return rec.Code, rec.Body.String(), err
}

func TestSearch_条件を渡す(t *testing.T) {
	s := &fakeStore{}
	code, body, err := call(NewController(s).Search,
		"/api/companies/search?name=%20A%E7%A4%BE%20&industry=IT&prefecture=%E6%9D%B1%E4%BA%AC%E9%83%BD&job_category_id=3&limit=500&offset=40", 0)
	if err != nil || code != http.StatusOK {
		t.Fatalf("code=%d err=%v", code, err)
	}
	want := repositories.CompanySearchFilter{Name: "A社", Industry: "IT", Prefecture: "東京都", JobCategoryID: 3, Limit: 100, Offset: 40}
	if s.got != want {
		t.Fatalf("filter = %+v, want %+v（limit は上限100に丸める）", s.got, want)
	}
	if !strings.Contains(body, `"companies":[]`) {
		t.Fatalf("0件は空の配列: %s", body)
	}
}

func TestSearch_不正な条件は400(t *testing.T) {
	for _, q := range []string{
		"prefecture=%E6%9D%B1%E4%BA%AC", // 「東京」（都道府県の一覧に無い）
		"prefecture=%25",                // LIKE の記号
		"job_category_id=0", "job_category_id=abc",
		"offset=-1", "offset=x",
		"name=" + strings.Repeat("a", maxNameRunes+1),
	} {
		s := &fakeStore{}
		if code, _, _ := call(NewController(s).Search, "/api/companies/search?"+q, 0); code != http.StatusBadRequest || s.searched {
			t.Errorf("%s: code=%d searched=%v", q, code, s.searched)
		}
	}
}

func TestOptions(t *testing.T) {
	code, body, err := call(NewController(&fakeStore{}).Options, "/api/companies/search-options", 0)
	if err != nil || code != http.StatusOK {
		t.Fatalf("code=%d err=%v", code, err)
	}
	for _, want := range []string{`"industries":[]`, `"job_categories":[]`, `"北海道"`, `"沖縄県"`} {
		if !strings.Contains(body, want) {
			t.Errorf("%s が無い: %s", want, body)
		}
	}
	if code, _, _ := call(NewController(&fakeStore{err: errors.New("db")}).Options, "/api/companies/search-options", 0); code != http.StatusInternalServerError {
		t.Fatalf("失敗: %d", code)
	}
}

func TestFavorites(t *testing.T) {
	s := &fakeStore{}
	code, _, err := call(NewController(s).Favorites, "/api/companies/search/favorites?company_ids=3,%201,3,,2", 7)
	if err != nil || code != http.StatusOK || s.favUser != 7 || len(s.favIDs) != 3 || s.favIDs[0] != 3 || s.favIDs[1] != 1 {
		t.Fatalf("code=%d err=%v user=%d ids=%v（重複と空は除く）", code, err, s.favUser, s.favIDs)
	}
	if code, _, _ := call(NewController(s).Favorites, "/api/companies/search/favorites?company_ids=1", 0); code != http.StatusUnauthorized {
		t.Fatalf("未ログイン: %d", code)
	}
	if code, _, _ := call(NewController(s).Favorites, "/api/companies/search/favorites?company_ids=1,x", 7); code != http.StatusBadRequest {
		t.Fatalf("不正な ID: %d", code)
	}
	// 上限ちょうどは通し、1件超えると 400
	for _, tt := range []struct {
		n    int
		want int
	}{{maxFavoriteIDs, http.StatusOK}, {maxFavoriteIDs + 1, http.StatusBadRequest}} {
		many := make([]string, tt.n)
		for i := range many {
			many[i] = strconv.Itoa(i + 1)
		}
		if code, _, _ := call(NewController(&fakeStore{}).Favorites, "/api/companies/search/favorites?company_ids="+strings.Join(many, ","), 7); code != tt.want {
			t.Errorf("%d件: %d, want %d", tt.n, code, tt.want)
		}
	}
}
