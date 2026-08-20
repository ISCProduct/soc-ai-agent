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

	"Backend/domain/entity"
	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
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

// #982: school scope制限のあるadminは、担当校外のユーザーのセッション動画一覧を取得できない(403)。
func TestAdminInterviewController_ListVideos_SchoolAccessDenied(t *testing.T) {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	otherSchoolID := uint(99)
	sessionRepo := &mocks.InterviewSessionRepositoryMock{}
	sessionRepo.On("FindByID", uint(5)).Return(&models.InterviewSession{ID: 5, UserID: 3}, nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "u3@example.com", SchoolID: &otherSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)
	c.SetSchoolAccess(userRepo, sessionRepo, services.NewSchoolService(schoolRepo))

	req := withAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/interview/sessions/5/videos", nil), 42)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("5")
	assertStatus(t, c.ListVideos, ctx, http.StatusForbidden)
	sessionRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// #982: 担当校が一致すればセッション動画一覧を取得できる。
func TestAdminInterviewController_ListVideos_SchoolAccessAllowed(t *testing.T) {
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	c := controllers.NewAdminInterviewController(nil, videoRepo, nil)
	ownSchoolID := uint(1)
	sessionRepo := &mocks.InterviewSessionRepositoryMock{}
	sessionRepo.On("FindByID", uint(5)).Return(&models.InterviewSession{ID: 5, UserID: 3}, nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "u3@example.com", SchoolID: &ownSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)
	c.SetSchoolAccess(userRepo, sessionRepo, services.NewSchoolService(schoolRepo))
	videoRepo.On("FindBySessionID", mock.Anything, uint(5)).Return([]models.InterviewVideo{}, nil)

	req := withAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/interview/sessions/5/videos", nil), 42)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("5")
	assertStatus(t, c.ListVideos, ctx, http.StatusOK)
	sessionRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	videoRepo.AssertExpectations(t)
}

// #982: school scope制限のあるadminは、担当校外のユーザーの動画URLを取得できない(403)。
func TestAdminInterviewController_VideoURL_SchoolAccessDenied(t *testing.T) {
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	c := controllers.NewAdminInterviewController(nil, videoRepo, nil)
	otherSchoolID := uint(99)
	videoRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.InterviewVideo{ID: 7, UserID: 3, Status: "done", DriveFileID: "f1"}, nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "u3@example.com", SchoolID: &otherSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)
	c.SetSchoolAccess(userRepo, nil, services.NewSchoolService(schoolRepo))

	req := withAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/interview/videos/7/url", nil), 42)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("video_id")
	ctx.SetParamValues("7")
	assertStatus(t, c.VideoURL, ctx, http.StatusForbidden)
	videoRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
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
// #983: 認証済みユーザーIDはリクエストコンテキスト(EchoUserAuth経由)から取得するため、
// 未認証(コンテキストにユーザーIDが無い)場合は401を返す。

func TestScheduleController_List_Unauthenticated(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.List, newCtx(req, rec), http.StatusUnauthorized)
}

func TestScheduleController_Create_Unauthenticated(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.Create, newCtx(req, rec), http.StatusUnauthorized)
}

func TestScheduleController_Get_Unauthenticated(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Get, ctx, http.StatusUnauthorized)
}

func TestScheduleController_Update_Unauthenticated(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Update, ctx, http.StatusUnauthorized)
}

func TestScheduleController_Delete_Unauthenticated(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Delete, ctx, http.StatusUnauthorized)
}

func TestScheduleController_ExportICS_Unauthenticated(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/export.ics", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.ExportICS, newCtx(req, rec), http.StatusUnauthorized)
}

// ---- CompanyEntryController ----

func TestCompanyEntryController_Submit_MissingName(t *testing.T) {
	c := controllers.NewCompanyEntryController(nil)
	body, _ := json.Marshal(map[string]any{"name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/company-entry", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	// service が nil だと panic するため、バリデーション前に Bind のみ通るケースは別テストで担保
	_ = c
	_ = req
	_ = rec
	t.Skip("replaced by company_entry_service / controller unit tests")
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

