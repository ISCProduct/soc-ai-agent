package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/middleware"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

// #2039: 機能横断のコンバージョンファネルの API。

type fakeFunnelSource struct {
	gotSchool *uint
	gotPeriod repositories.FunnelPeriod
}

func (f *fakeFunnelSource) Students(schoolID *uint) ([]uint, error) {
	f.gotSchool = schoolID
	return []uint{1, 2}, nil
}

func (f *fakeFunnelSource) StageUserIDs(schoolID *uint, p repositories.FunnelPeriod) (map[string][]uint, error) {
	f.gotPeriod = p
	return map[string][]uint{"diagnosis": {1, 2}, "interview": {1}}, nil
}

func callFunnel(t *testing.T, src FunnelSource, query string, schoolID *uint, scoped bool) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/funnel?"+query, nil)
	if scoped {
		req = req.WithContext(context.WithValue(req.Context(), middleware.AdminSchoolFilterContextKey, schoolID))
	}
	rec := httptest.NewRecorder()
	c := &AdminDashboardController{}
	c.SetFunnelSource(src)
	return rec, c.Funnel(e.NewContext(req, rec))
}

func TestFunnel_担当校と期間で集計する(t *testing.T) {
	src := &fakeFunnelSource{}
	school := uint(7)
	rec, err := callFunnel(t, src, "school_id=7&from=2026-04-01&to=2026-04-30", &school, true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if src.gotSchool == nil || *src.gotSchool != 7 {
		t.Fatalf("担当校で絞っていない: %v", src.gotSchool)
	}
	jst := funnelLocation
	if !src.gotPeriod.From.Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, jst)) || !src.gotPeriod.To.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, jst)) {
		t.Fatalf("期間 = %v 〜 %v（to はその日の終わりまで含めるため翌日0時）", src.gotPeriod.From, src.gotPeriod.To)
	}
	var body struct {
		Funnel struct {
			Students int `json:"students"`
			Stages   []struct {
				Stage   string `json:"stage"`
				Reached int    `json:"reached"`
			} `json:"stages"`
		} `json:"funnel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body.Funnel.Students != 2 || len(body.Funnel.Stages) != 5 || body.Funnel.Stages[1].Reached != 1 {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestFunnel_不正な期間は400(t *testing.T) {
	for _, q := range []string{"from=2026/04/01", "to=abc", "from=2026-05-01&to=2026-04-01"} {
		_, err := callFunnel(t, &fakeFunnelSource{}, q, nil, true)
		var he *echo.HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusBadRequest {
			t.Fatalf("%s: err = %v, want 400", q, err)
		}
	}
	// 同じ日だけの期間は正しい（from 以上、翌日0時未満）
	if _, err := callFunnel(t, &fakeFunnelSource{}, "from=2026-04-01&to=2026-04-01", nil, true); err != nil {
		t.Fatalf("1日だけの期間: %v", err)
	}
}

// 担当校スコープのミドルウェアが外れていたら、全体を返さずに拒否する（fail-closed）。
func TestFunnel_スコープが無ければ拒否する(t *testing.T) {
	_, err := callFunnel(t, &fakeFunnelSource{}, "", nil, false)
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusForbidden {
		t.Fatalf("err = %v, want 403", err)
	}
}
