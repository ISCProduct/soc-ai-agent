package controllers_test

// 編集APIからの掲載状態変更が、システム管理者以外に通らないことを固定する。
//
// PATCH /companies/:id/publish を platform ミドルウェアで限定しても、
// PUT /companies/:id に data_status を載せれば担当校つき管理者が公開できた。
// ハンドラ内のガードはミドルウェアと違ってルート定義から見えないため、
// リファクタで落ちても気づけない。ここで 403 を固定する。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

func TestAdminCompanyController_Update_PublicationGuard(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]any
		restricted bool
		hasProfile bool
		want       int
	}{
		{
			name:       "担当校つき管理者は data_status を変えられない",
			body:       map[string]any{"data_status": "published"},
			restricted: true,
			want:       http.StatusForbidden,
		},
		{
			name:       "担当校つき管理者は is_provisional も変えられない",
			body:       map[string]any{"is_provisional": true},
			restricted: true,
			want:       http.StatusForbidden,
		},
		{
			name:       "掲載状態に触らない編集は担当校つき管理者でも通る",
			body:       map[string]any{"name": "テスト株式会社"},
			restricted: true,
			want:       http.StatusOK,
		},
		{
			name:       "システム管理者は data_status を変えられる",
			body:       map[string]any{"data_status": "published"},
			restricted: false,
			hasProfile: true,
			want:       http.StatusOK,
		},
		{
			// 重み付けプロファイルが無いまま公開すると、マッチングが既定値で計算される
			name:       "プロファイルが無い企業は編集からも公開できない",
			body:       map[string]any{"data_status": "published"},
			restricted: false,
			hasProfile: false,
			want:       http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.CompanyRepositoryMock{}
			repo.On("FindByID", uint(1)).Return(&models.Company{Name: "旧名", DataStatus: "draft"}, nil)
			repo.On("Update", mock.Anything).Return(nil)
			if tt.hasProfile {
				repo.On("GetWeightProfile", uint(1), (*uint)(nil)).Return(&models.CompanyWeightProfile{}, nil)
			} else {
				repo.On("GetWeightProfile", uint(1), (*uint)(nil)).Return(nil, gorm.ErrRecordNotFound)
			}
			audit := &mocks.AuditLogServiceMock{}
			audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

			ctrl := admincontrollers.NewAdminCompanyController(repo, audit, nil)
			ctrl.SetSchoolRestrictionChecker(func(uint) (bool, error) { return tt.restricted, nil })

			raw, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), middleware.AdminUserIDContextKey, uint(7)))
			rec := httptest.NewRecorder()
			c := newCtx(req, rec)
			c.SetParamNames("id")
			c.SetParamValues("1")

			assertStatus(t, ctrl.Update, c, tt.want)
		})
	}
}
