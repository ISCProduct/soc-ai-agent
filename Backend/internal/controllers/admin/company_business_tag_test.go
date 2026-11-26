package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// businessTagRepoFake はメモリ上で事業タグを持つ。
type businessTagRepoFake struct {
	byID     map[uint][]string
	replaced bool
}

func (f *businessTagRepoFake) ListAll(context.Context) ([]models.BusinessTag, error) {
	return []models.BusinessTag{{ID: 1, Name: "クラウド"}}, nil
}
func (f *businessTagRepoFake) NamesByCompanyIDs(_ context.Context, ids []uint) (map[uint][]string, error) {
	out := map[uint][]string{}
	for _, id := range ids {
		if tags, ok := f.byID[id]; ok {
			out[id] = tags
		}
	}
	return out, nil
}
func (f *businessTagRepoFake) ReplaceForCompany(_ context.Context, id uint, names []string) error {
	f.replaced = true
	if len(names) == 0 {
		delete(f.byID, id)
		return nil
	}
	f.byID[id] = names
	return nil
}

func TestAdminCompanyController_UpdateCompanyBusinessTags(t *testing.T) {
	// 重複はまとめられるので、別々の名前で上限+1件にする
	names := make([]string, models.MaxBusinessTagsPerCompany+1)
	for i := range names {
		names[i] = strconv.Quote(strings.Repeat("あ", i+1))
	}
	tooMany := `{"business_tags":[` + strings.Join(names, ",") + `]}`
	tests := []struct {
		name         string
		companyID    string
		body         string
		want         int
		wantReplaced bool
		wantTags     []string
	}{
		{name: "正規化して置き換える", companyID: "1", body: `{"business_tags":["ｸﾗｳﾄﾞ"," 業務システム ","クラウド"]}`,
			want: http.StatusOK, wantReplaced: true, wantTags: []string{"クラウド", "業務システム"}},
		{name: "空配列で全解除", companyID: "1", body: `{"business_tags":[]}`, want: http.StatusOK, wantReplaced: true},
		{name: "business_tags が無い本文は 400", companyID: "1", body: `{}`, want: http.StatusBadRequest},
		{name: "上限超過は 400", companyID: "1", body: tooMany, want: http.StatusBadRequest},
		{name: "存在しない企業は 404", companyID: "2", body: `{"business_tags":["クラウド"]}`, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.CompanyRepositoryMock{}
			repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1}, nil)
			repo.On("FindByID", uint(2)).Return(nil, gorm.ErrRecordNotFound)
			audit := &mocks.AuditLogServiceMock{}
			audit.On("Record", mock.Anything, "company.business_tags.update", "company", uint(1), mock.Anything).Return()
			tags := &businessTagRepoFake{byID: map[uint][]string{1: {"旧タグ"}}}

			ctrl := admincontrollers.NewAdminCompanyController(repo, audit, nil)
			ctrl.SetBusinessTagRepository(tags)

			req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/"+tt.companyID+"/business-tags", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := testsupport.NewCtx(req, rec)
			c.SetParamNames("id")
			c.SetParamValues(tt.companyID)

			testsupport.AssertStatus(t, ctrl.UpdateCompanyBusinessTags, c, tt.want)
			assert.Equal(t, tt.wantReplaced, tags.replaced)
			if tt.wantReplaced {
				assert.Equal(t, tt.wantTags, tags.byID[1])
				audit.AssertExpectations(t)
			} else {
				audit.AssertNotCalled(t, "Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestAdminCompanyController_GetCompanyBusinessTags_EmptyIsArray(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1}, nil)
	ctrl := admincontrollers.NewAdminCompanyController(repo, &mocks.AuditLogServiceMock{}, nil)
	ctrl.SetBusinessTagRepository(&businessTagRepoFake{byID: map[uint][]string{}})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1/business-tags", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	testsupport.AssertStatus(t, ctrl.GetCompanyBusinessTags, c, http.StatusOK)
	assert.JSONEq(t, `{"company_id":1,"business_tags":[]}`, rec.Body.String())
}
