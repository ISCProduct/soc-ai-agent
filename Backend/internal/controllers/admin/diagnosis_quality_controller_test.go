package admin

import (
	"Backend/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeDiagnosisQualityRepo struct {
	limit int
}

func (f *fakeDiagnosisQualityRepo) ListRecent(limit int) ([]models.DiagnosisQualityReport, error) {
	f.limit = limit
	return []models.DiagnosisQualityReport{}, nil
}

func (f *fakeDiagnosisQualityRepo) FindByUserAndSession(uint, string) (*models.DiagnosisQualityReport, error) {
	return nil, nil
}

func TestDiagnosisQualityListCapsLimit(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "未指定は既定の50", query: "", wantLimit: 50},
		{name: "上限内はそのまま", query: "?limit=30", wantLimit: 30},
		{name: "上限ちょうど", query: "?limit=100", wantLimit: 100},
		{name: "上限超過は100へ頭打ち", query: "?limit=101", wantLimit: 100},
		{name: "極端な値も100へ頭打ち", query: "?limit=1000000", wantLimit: 100},
		{name: "0は既定の50", query: "?limit=0", wantLimit: 50},
		{name: "数値以外は既定の50", query: "?limit=abc", wantLimit: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDiagnosisQualityRepo{}
			req := httptest.NewRequest(http.MethodGet, "/api/admin/diagnosis-quality"+tt.query, nil)
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(req, rec)
			ctrl := &AdminDiagnosisQualityController{repo: repo}
			if err := ctrl.List(c); err != nil {
				t.Fatalf("List: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if repo.limit != tt.wantLimit {
				t.Fatalf("limit = %d, 期待 %d", repo.limit, tt.wantLimit)
			}
		})
	}
}
