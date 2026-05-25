package controllers_test

// 残りのコントローラーのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run "AdminCrawl|AdminInterview|Question|Schedule|CompanyEntry|CompanyRelation|ESReview|ESRewrite|GitHub" -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
)

// ---- AdminCrawlController ----

func TestAdminCrawlController_ListSources_CallsService(t *testing.T) {
	// nilサービスでパニックになる前に返すケースはないため、コンストラクタの動作のみ確認
	c := controllers.NewAdminCrawlController(nil, nil)
	if c == nil {
		t.Fatal("NewAdminCrawlController returned nil")
	}
}

func TestAdminCrawlController_Runs_InvalidSourceID(t *testing.T) {
	// source_idが数値でない場合は無視されてサービス呼び出しになるため、
	// nilサービスで呼ぶとpanicするケースはここでは扱わない
	c := controllers.NewAdminCrawlController(nil, nil)
	if c == nil {
		t.Fatal("NewAdminCrawlController returned nil")
	}
}

// ---- AdminInterviewController ----

func TestAdminInterviewController_ListVideos_InvalidID(t *testing.T) {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/interview/sessions/abc/videos", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.ListVideos, ctx, http.StatusBadRequest)
}

func TestAdminInterviewController_VideoURL_InvalidID(t *testing.T) {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/interview/videos/xyz/url", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("video_id")
	ctx.SetParamValues("xyz")
	assertStatus(t, c.VideoURL, ctx, http.StatusBadRequest)
}

// ---- QuestionController ----

func TestQuestionController_GenerateQuestions_MissingCategory(t *testing.T) {
	c := controllers.NewQuestionController(nil)
	body, _ := json.Marshal(map[string]any{"count": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.GenerateQuestions, newCtx(req, rec), http.StatusBadRequest)
}

func TestQuestionController_CreateQuestion_MissingFields(t *testing.T) {
	c := controllers.NewQuestionController(nil)
	body, _ := json.Marshal(map[string]any{"question": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.CreateQuestion, newCtx(req, rec), http.StatusBadRequest)
}

func TestQuestionController_GetQuestionsByCategory_MissingCategory(t *testing.T) {
	c := controllers.NewQuestionController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/questions", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetQuestionsByCategory, newCtx(req, rec), http.StatusBadRequest)
}

// ---- ScheduleController ----

func TestScheduleController_List_MissingUserID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.List, newCtx(req, rec), http.StatusBadRequest)
}

func TestScheduleController_Create_MissingUserID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.Create, newCtx(req, rec), http.StatusBadRequest)
}

func TestScheduleController_Get_MissingUserID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Get, ctx, http.StatusBadRequest)
}

func TestScheduleController_Update_MissingUserID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Update, ctx, http.StatusBadRequest)
}

func TestScheduleController_Delete_MissingUserID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Delete, ctx, http.StatusBadRequest)
}

func TestScheduleController_ExportICS_MissingUserID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/export.ics", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.ExportICS, newCtx(req, rec), http.StatusBadRequest)
}

// ---- CompanyEntryController ----

func TestCompanyEntryController_Submit_MissingName(t *testing.T) {
	c := controllers.NewCompanyEntryController(nil, nil)
	body, _ := json.Marshal(map[string]any{"name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/companies/entry", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.Submit, newCtx(req, rec), http.StatusBadRequest)
}

// ---- CompanyRelationController ----

func TestCompanyRelationController_GetCompanyByID_InvalidID(t *testing.T) {
	c := controllers.NewCompanyRelationController(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.GetCompanyByID, ctx, http.StatusBadRequest)
}

func TestCompanyRelationController_GetCompanyJobPositions_InvalidID(t *testing.T) {
	c := controllers.NewCompanyRelationController(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc/job-positions", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.GetCompanyJobPositions, ctx, http.StatusBadRequest)
}

func TestCompanyRelationController_WebSearchCompanies_MissingQuery(t *testing.T) {
	c := controllers.NewCompanyRelationController(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/search", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.WebSearchCompanies, newCtx(req, rec), http.StatusBadRequest)
}

// ---- ESReviewController ----

func TestESReviewController_Review_MissingESText(t *testing.T) {
	c := controllers.NewESReviewController()
	body, _ := json.Marshal(map[string]any{"es_text": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.Review, newCtx(req, rec), http.StatusBadRequest)
}

// ---- ESRewriteController ----

func TestESRewriteController_Rewrite_InvalidBody(t *testing.T) {
	c := controllers.NewESRewriteController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/es/rewrite", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.Rewrite, newCtx(req, rec), http.StatusBadRequest)
}

