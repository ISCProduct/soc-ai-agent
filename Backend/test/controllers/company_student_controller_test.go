package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	hrsvc "Backend/internal/services/hr"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withCompanyContext は企業ポータルJWT認証済みの状態を模す。
func withCompanyContext(r *http.Request, companyUserID, companyID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.CompanyUserIDContextKey, companyUserID)
	ctx = context.WithValue(ctx, middleware.CompanyIDContextKey, companyID)
	return r.WithContext(ctx)
}

type companyStudentSearchStub struct {
	result       *hrsvc.StudentSearchResult
	err          error
	tags         []hrsvc.StudentTagView
	names        []string
	addErr       error
	lastCompany  uint
	lastFilters  repositories.StudentSearchFilters
	lastTagOwner uint
	lastQuery    string
}

func (s *companyStudentSearchStub) Search(companyID uint, f repositories.StudentSearchFilters) (*hrsvc.StudentSearchResult, error) {
	s.lastCompany = companyID
	s.lastFilters = f
	return s.result, s.err
}

func (s *companyStudentSearchStub) SemanticSearch(_ context.Context, companyID uint, query string, f repositories.StudentSearchFilters) (*hrsvc.StudentSearchResult, error) {
	s.lastCompany = companyID
	s.lastFilters = f
	s.lastQuery = query
	return s.result, s.err
}

func (s *companyStudentSearchStub) AddTag(companyID, companyUserID, _ uint, _ string) error {
	s.lastCompany = companyID
	s.lastTagOwner = companyUserID
	return s.addErr
}

func (s *companyStudentSearchStub) RemoveTag(companyID, _ uint) error {
	s.lastCompany = companyID
	return s.err
}

func (s *companyStudentSearchStub) ListTagNames(companyID uint) ([]string, error) {
	s.lastCompany = companyID
	return s.names, s.err
}

func (s *companyStudentSearchStub) ListTagsForUser(companyID, _ uint) ([]hrsvc.StudentTagView, error) {
	s.lastCompany = companyID
	return s.tags, nil
}

type companyStudentAnalysisStub struct {
	resp *hrsvc.StudentAnalysisResponse
	err  error
}

func (s *companyStudentAnalysisStub) GetAnalysisForVisibleStudent(uint) (*hrsvc.StudentAnalysisResponse, error) {
	return s.resp, s.err
}

type companyIndustryStub struct {
	items []repositories.IndustryOption
	err   error
}

func (s *companyIndustryStub) ListActive() ([]repositories.IndustryOption, error) {
	return s.items, s.err
}

func newCompanyStudentController(search *companyStudentSearchStub, analysis *companyStudentAnalysisStub) *controllers.CompanyStudentController {
	return controllers.NewCompanyStudentController(search, analysis, &companyIndustryStub{
		items: []repositories.IndustryOption{{ID: 1, Name: "IT・通信"}},
	})
}

func companyStudentCtx(req *http.Request, rec *httptest.ResponseRecorder, names, values []string) echo.Context {
	c := newCtx(req, rec)
	c.SetParamNames(names...)
	c.SetParamValues(values...)
	return c
}

// ── 認可（未認証は401） ─────────────────────────────────────────────

func TestCompanyStudentController_RequiresCompanyAuth(t *testing.T) {
	ctrl := newCompanyStudentController(&companyStudentSearchStub{}, &companyStudentAnalysisStub{})
	tests := []struct {
		name    string
		handler func(echo.Context) error
		method  string
	}{
		{name: "一覧", handler: ctrl.List, method: http.MethodGet},
		{name: "セマンティック検索", handler: ctrl.SemanticSearch, method: http.MethodPost},
		{name: "詳細", handler: ctrl.Detail, method: http.MethodGet},
		{name: "タグ付与", handler: ctrl.AddTag, method: http.MethodPost},
		{name: "タグ削除", handler: ctrl.RemoveTag, method: http.MethodDelete},
		{name: "タグ一覧", handler: ctrl.ListTags, method: http.MethodGet},
		{name: "業界一覧", handler: ctrl.Industries, method: http.MethodGet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/company-portal/students", nil)
			rec := httptest.NewRecorder()
			// 企業コンテキストを設定しない = 未認証
			assertStatus(t, tt.handler, companyStudentCtx(req, rec, []string{"userID", "tagID"}, []string{"5", "1"}), http.StatusUnauthorized)
		})
	}
}

// ── 一覧・フィルタ ─────────────────────────────────────────────────

func TestCompanyStudentController_List_UsesJWTCompanyIDAndFilters(t *testing.T) {
	search := &companyStudentSearchStub{result: &hrsvc.StudentSearchResult{Items: []hrsvc.StudentListItem{}}}
	req := httptest.NewRequest(http.MethodGet,
		"/api/company-portal/students?industry_id=3&location=東京&skill=基本情報&tag=候補A&limit=10&offset=20&company_id=999", nil)
	req = withCompanyContext(req, 42, 7)
	rec := httptest.NewRecorder()

	assertStatus(t, newCompanyStudentController(search, nil).List, newCtx(req, rec), http.StatusOK)

	// company_id クエリを付けても JWT の company_id が使われる（他社越境の防止）。
	assert.Equal(t, uint(7), search.lastCompany)
	assert.Equal(t, uint(3), search.lastFilters.IndustryID)
	assert.Equal(t, "東京", search.lastFilters.Location)
	assert.Equal(t, "基本情報", search.lastFilters.Skill)
	assert.Equal(t, "候補A", search.lastFilters.Tag)
	assert.Equal(t, 10, search.lastFilters.Limit)
	assert.Equal(t, 20, search.lastFilters.Offset)
}

// ── セマンティック検索 ─────────────────────────────────────────────

func TestCompanyStudentController_SemanticSearch(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
	}{
		{name: "成功", body: `{"query":"リーダーシップ経験があってReactができる学生"}`, wantStatus: http.StatusOK},
		{name: "不正なJSONは400", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "空クエリは400", body: `{"query":"  "}`, svcErr: hrsvc.ErrEmptyQuery, wantStatus: http.StatusBadRequest},
		{name: "RAG不通は503", body: `{"query":"React"}`, svcErr: hrsvc.ErrSemanticSearchUnavailable, wantStatus: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			search := &companyStudentSearchStub{
				result: &hrsvc.StudentSearchResult{Items: []hrsvc.StudentListItem{}},
				err:    tt.svcErr,
			}
			req := httptest.NewRequest(http.MethodPost, "/api/company-portal/students/semantic-search",
				bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withCompanyContext(req, 42, 7)
			rec := httptest.NewRecorder()

			assertStatus(t, newCompanyStudentController(search, nil).SemanticSearch, newCtx(req, rec), tt.wantStatus)
		})
	}
}

// ── 詳細 ───────────────────────────────────────────────────────────

func TestCompanyStudentController_Detail(t *testing.T) {
	tests := []struct {
		name        string
		analysisErr error
		wantStatus  int
	}{
		{name: "成功", wantStatus: http.StatusOK},
		{name: "非公開の学生は404", analysisErr: hrsvc.ErrStudentNotVisible, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			search := &companyStudentSearchStub{tags: []hrsvc.StudentTagView{{ID: 1, TagName: "即戦力"}}}
			analysis := &companyStudentAnalysisStub{
				resp: &hrsvc.StudentAnalysisResponse{UserID: 5},
				err:  tt.analysisErr,
			}
			req := httptest.NewRequest(http.MethodGet, "/api/company-portal/students/5", nil)
			req = withCompanyContext(req, 42, 7)
			rec := httptest.NewRecorder()

			assertStatus(t, newCompanyStudentController(search, analysis).Detail,
				companyStudentCtx(req, rec, []string{"userID"}, []string{"5"}), tt.wantStatus)
		})
	}
}

func TestCompanyStudentController_Detail_ReturnsOwnCompanyTags(t *testing.T) {
	search := &companyStudentSearchStub{tags: []hrsvc.StudentTagView{{ID: 1, TagName: "即戦力"}}}
	analysis := &companyStudentAnalysisStub{resp: &hrsvc.StudentAnalysisResponse{UserID: 5}}
	req := httptest.NewRequest(http.MethodGet, "/api/company-portal/students/5", nil)
	req = withCompanyContext(req, 42, 7)
	rec := httptest.NewRecorder()

	assertStatus(t, newCompanyStudentController(search, analysis).Detail,
		companyStudentCtx(req, rec, []string{"userID"}, []string{"5"}), http.StatusOK)

	var body struct {
		Tags []hrsvc.StudentTagView `json:"tags"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Tags, 1)
	assert.Equal(t, "即戦力", body.Tags[0].TagName)
	assert.Equal(t, uint(7), search.lastCompany, "タグは自社IDでのみ取得される")
}

// ── タグ ───────────────────────────────────────────────────────────

func TestCompanyStudentController_AddTag(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		addErr     error
		wantStatus int
	}{
		{name: "成功", body: `{"tag_name":"即戦力"}`, wantStatus: http.StatusCreated},
		{name: "不正なJSONは400", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "空タグは400", body: `{"tag_name":""}`, addErr: hrsvc.ErrInvalidTagName, wantStatus: http.StatusBadRequest},
		{name: "非公開の学生は404", body: `{"tag_name":"即戦力"}`, addErr: hrsvc.ErrStudentNotVisible, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			search := &companyStudentSearchStub{addErr: tt.addErr}
			req := httptest.NewRequest(http.MethodPost, "/api/company-portal/students/5/tags",
				bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withCompanyContext(req, 42, 7)
			rec := httptest.NewRecorder()

			assertStatus(t, newCompanyStudentController(search, nil).AddTag,
				companyStudentCtx(req, rec, []string{"userID"}, []string{"5"}), tt.wantStatus)
		})
	}
}

func TestCompanyStudentController_AddTag_RecordsCreator(t *testing.T) {
	search := &companyStudentSearchStub{}
	req := httptest.NewRequest(http.MethodPost, "/api/company-portal/students/5/tags",
		bytes.NewBufferString(`{"tag_name":"即戦力"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withCompanyContext(req, 42, 7)
	rec := httptest.NewRecorder()

	assertStatus(t, newCompanyStudentController(search, nil).AddTag,
		companyStudentCtx(req, rec, []string{"userID"}, []string{"5"}), http.StatusCreated)

	assert.Equal(t, uint(7), search.lastCompany)
	assert.Equal(t, uint(42), search.lastTagOwner, "作成者は企業ユーザーIDで記録される")
}

func TestCompanyStudentController_RemoveTag_ScopedToOwnCompany(t *testing.T) {
	search := &companyStudentSearchStub{}
	req := httptest.NewRequest(http.MethodDelete, "/api/company-portal/students/5/tags/9", nil)
	req = withCompanyContext(req, 42, 7)
	rec := httptest.NewRecorder()

	assertStatus(t, newCompanyStudentController(search, nil).RemoveTag,
		companyStudentCtx(req, rec, []string{"userID", "tagID"}, []string{"5", "9"}), http.StatusNoContent)

	assert.Equal(t, uint(7), search.lastCompany)
}

func TestCompanyStudentController_Industries(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/company-portal/industries", nil)
	req = withCompanyContext(req, 42, 7)
	rec := httptest.NewRecorder()

	assertStatus(t, newCompanyStudentController(&companyStudentSearchStub{}, nil).Industries,
		newCtx(req, rec), http.StatusOK)
	assert.Contains(t, rec.Body.String(), "IT・通信")
}

func TestCompanyStudentController_ListTags(t *testing.T) {
	search := &companyStudentSearchStub{names: []string{"候補A", "即戦力"}}
	req := httptest.NewRequest(http.MethodGet, "/api/company-portal/tags", nil)
	req = withCompanyContext(req, 42, 7)
	rec := httptest.NewRecorder()

	assertStatus(t, newCompanyStudentController(search, nil).ListTags, newCtx(req, rec), http.StatusOK)
	assert.Equal(t, uint(7), search.lastCompany)
	assert.Contains(t, rec.Body.String(), "即戦力")
}
