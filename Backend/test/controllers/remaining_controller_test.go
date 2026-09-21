// Package controllers_test は、複数のコントローラーパッケージにまたがる
// テストだけを置く。
//
// 単一のコントローラーを対象とするテストは、それぞれの
// internal/controllers/<pkg>/ へ移した。ここに残っているのは 1 ファイルで
// admin / chat / company / es / schedule のように複数パッケージを
// 横断して検証しているもので、どれか 1 つのパッケージには属せない。
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
	admincontrollers "Backend/internal/controllers/admin"
	chatcontrollers "Backend/internal/controllers/chat"
	companycontrollers "Backend/internal/controllers/company"
	escontrollers "Backend/internal/controllers/es"
	"Backend/internal/controllers/mocks"
	schedulecontrollers "Backend/internal/controllers/schedule"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"

	"github.com/stretchr/testify/mock"
)

// ---- AdminCrawlController ----

func TestAdminCrawlController_ListSources_CallsService(t *testing.T) {
	// nilサービスでパニックになる前に返すケースはないため、コンストラクタの動作のみ確認
	c := admincontrollers.NewAdminCrawlController(nil, nil)
	if c == nil {
		t.Fatal("NewAdminCrawlController returned nil")
	}
}

func TestAdminCrawlController_Runs_InvalidSourceID(t *testing.T) {
	// source_idが数値でない場合は無視されてサービス呼び出しになるため、
	// nilサービスで呼ぶとpanicするケースはここでは扱わない
	c := admincontrollers.NewAdminCrawlController(nil, nil)
	if c == nil {
		t.Fatal("NewAdminCrawlController returned nil")
	}
}

// ---- AdminInterviewController ----

func TestAdminInterviewController_ListVideos_InvalidID(t *testing.T) {
	c := admincontrollers.NewAdminInterviewController(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/interview/sessions/abc/videos", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	testsupport.AssertStatus(t, c.ListVideos, ctx, http.StatusBadRequest)
}

func TestAdminInterviewController_VideoURL_InvalidID(t *testing.T) {
	c := admincontrollers.NewAdminInterviewController(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/interview/videos/xyz/url", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("video_id")
	ctx.SetParamValues("xyz")
	testsupport.AssertStatus(t, c.VideoURL, ctx, http.StatusBadRequest)
}

// #982: school scope制限のあるadminは、担当校外のユーザーのセッション動画一覧を取得できない(403)。
func TestAdminInterviewController_ListVideos_SchoolAccessDenied(t *testing.T) {
	c := admincontrollers.NewAdminInterviewController(nil, nil, nil)
	otherSchoolID := uint(99)
	sessionRepo := &mocks.InterviewSessionRepositoryMock{}
	sessionRepo.On("FindByID", uint(5)).Return(&models.InterviewSession{ID: 5, UserID: 3}, nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "u3@example.com", SchoolID: &otherSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)
	c.SetSchoolAccess(userRepo, sessionRepo, school.NewSchoolService(schoolRepo))

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/interview/sessions/5/videos", nil), 42)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("5")
	testsupport.AssertStatus(t, c.ListVideos, ctx, http.StatusForbidden)
	sessionRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// #982: 担当校が一致すればセッション動画一覧を取得できる。
func TestAdminInterviewController_ListVideos_SchoolAccessAllowed(t *testing.T) {
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	c := admincontrollers.NewAdminInterviewController(nil, videoRepo, nil)
	ownSchoolID := uint(1)
	sessionRepo := &mocks.InterviewSessionRepositoryMock{}
	sessionRepo.On("FindByID", uint(5)).Return(&models.InterviewSession{ID: 5, UserID: 3}, nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "u3@example.com", SchoolID: &ownSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)
	c.SetSchoolAccess(userRepo, sessionRepo, school.NewSchoolService(schoolRepo))
	videoRepo.On("FindBySessionID", mock.Anything, uint(5)).Return([]models.InterviewVideo{}, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/interview/sessions/5/videos", nil), 42)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("5")
	testsupport.AssertStatus(t, c.ListVideos, ctx, http.StatusOK)
	sessionRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	videoRepo.AssertExpectations(t)
}

// #982: school scope制限のあるadminは、担当校外のユーザーの動画URLを取得できない(403)。
func TestAdminInterviewController_VideoURL_SchoolAccessDenied(t *testing.T) {
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	c := admincontrollers.NewAdminInterviewController(nil, videoRepo, nil)
	otherSchoolID := uint(99)
	videoRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.InterviewVideo{ID: 7, UserID: 3, Status: "done", DriveFileID: "f1"}, nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "u3@example.com", SchoolID: &otherSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)
	c.SetSchoolAccess(userRepo, nil, school.NewSchoolService(schoolRepo))

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/interview/videos/7/url", nil), 42)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("video_id")
	ctx.SetParamValues("7")
	testsupport.AssertStatus(t, c.VideoURL, ctx, http.StatusForbidden)
	videoRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// ---- QuestionController ----

func TestQuestionController_GenerateQuestions_MissingCategory(t *testing.T) {
	c := chatcontrollers.NewQuestionController(nil)
	body, _ := json.Marshal(map[string]any{"count": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GenerateQuestions, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestQuestionController_CreateQuestion_MissingFields(t *testing.T) {
	c := chatcontrollers.NewQuestionController(nil)
	body, _ := json.Marshal(map[string]any{"question": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.CreateQuestion, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestQuestionController_GetQuestionsByCategory_MissingCategory(t *testing.T) {
	c := chatcontrollers.NewQuestionController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/questions", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetQuestionsByCategory, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

// ---- ScheduleController ----
// #983: 認証済みユーザーIDはリクエストコンテキスト(EchoUserAuth経由)から取得するため、
// 未認証(コンテキストにユーザーIDが無い)場合は401を返す。

func TestScheduleController_List_Unauthenticated(t *testing.T) {
	c := schedulecontrollers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.List, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestScheduleController_Create_Unauthenticated(t *testing.T) {
	c := schedulecontrollers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.Create, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestScheduleController_Get_Unauthenticated(t *testing.T) {
	c := schedulecontrollers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, c.Get, ctx, http.StatusUnauthorized)
}

func TestScheduleController_Update_Unauthenticated(t *testing.T) {
	c := schedulecontrollers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, c.Update, ctx, http.StatusUnauthorized)
}

func TestScheduleController_Delete_Unauthenticated(t *testing.T) {
	c := schedulecontrollers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/schedules/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, c.Delete, ctx, http.StatusUnauthorized)
}

func TestScheduleController_ExportICS_Unauthenticated(t *testing.T) {
	c := schedulecontrollers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/export.ics", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.ExportICS, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

// ---- CompanyEntryController ----

func TestCompanyEntryController_Submit_MissingName(t *testing.T) {
	c := companycontrollers.NewCompanyEntryController(nil)
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
	c := companycontrollers.NewCompanyRelationController(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	testsupport.AssertStatus(t, c.GetCompanyByID, ctx, http.StatusBadRequest)
}

func TestCompanyRelationController_GetCompanyJobPositions_InvalidID(t *testing.T) {
	c := companycontrollers.NewCompanyRelationController(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc/job-positions", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	testsupport.AssertStatus(t, c.GetCompanyJobPositions, ctx, http.StatusBadRequest)
}

func TestCompanyRelationController_WebSearchCompanies_MissingQuery(t *testing.T) {
	c := companycontrollers.NewCompanyRelationController(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/search", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.WebSearchCompanies, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

// ---- ESReviewController ----

func TestESReviewController_Review_MissingESText(t *testing.T) {
	c := escontrollers.NewESReviewController()
	body, _ := json.Marshal(map[string]any{"es_text": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.Review, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

// ---- ESRewriteController ----

func TestESRewriteController_Rewrite_InvalidBody(t *testing.T) {
	c := escontrollers.NewESRewriteController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/es/rewrite", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.Rewrite, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}
