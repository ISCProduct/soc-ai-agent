package admin_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
)

type interviewCompanyQuestionRepoStub struct {
	question   *models.InterviewCompanyQuestion
	existing   []models.InterviewCompanyQuestion
	createCall bool
	updateCall bool
	deleteCall bool
}

func (r *interviewCompanyQuestionRepoStub) FindByCompanyID(uint) ([]models.InterviewCompanyQuestion, error) {
	return r.existing, nil
}

func (r *interviewCompanyQuestionRepoStub) FindByCompanyAndPosition(uint, string) ([]models.InterviewCompanyQuestion, error) {
	return nil, nil
}

func (r *interviewCompanyQuestionRepoStub) FindByID(uint) (*models.InterviewCompanyQuestion, error) {
	return r.question, nil
}

func (r *interviewCompanyQuestionRepoStub) FindByIDForCompany(uint, uint) (*models.InterviewCompanyQuestion, error) {
	return r.question, nil
}

func (r *interviewCompanyQuestionRepoStub) Create(*models.InterviewCompanyQuestion) error {
	r.createCall = true
	return nil
}

func (r *interviewCompanyQuestionRepoStub) Update(*models.InterviewCompanyQuestion) error {
	r.updateCall = true
	return nil
}

func (r *interviewCompanyQuestionRepoStub) Delete(uint) error {
	r.deleteCall = true
	return nil
}

func newAdminInterviewQuestionController(repo *interviewCompanyQuestionRepoStub) *admincontrollers.AdminInterviewController {
	c := admincontrollers.NewAdminInterviewController(nil, nil, nil)
	c.SetCompanyQuestionRepo(repo)
	return c
}

func TestAdminInterviewController_CreateCompanyQuestion_RejectsWhitespace(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/1/interview-questions", bytes.NewBufferString(`{"category":"   ","question_text":" question "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")

	testsupport.AssertStatus(t, newAdminInterviewQuestionController(repo).CreateCompanyQuestion, ctx, http.StatusBadRequest)
}

// 管理者経路も企業ポータルと同じ制約を守る。同じテーブルを操作するので、片方だけ緩いと制約が破れる。
func TestAdminInterviewController_CreateCompanyQuestion_EnforcesPortalConstraints(t *testing.T) {
	tooLong := strings.Repeat("あ", 2001)
	tests := []struct {
		name       string
		body       string
		existing   int
		wantStatus int
	}{
		{name: "質問文が2000字を超える", body: `{"category":"技術","question_text":"` + tooLong + `"}`, wantStatus: http.StatusBadRequest},
		{name: "優先度が範囲外", body: `{"category":"技術","question_text":"質問","priority":1000}`, wantStatus: http.StatusBadRequest},
		{name: "件数が上限に達している", body: `{"category":"技術","question_text":"質問"}`, existing: 50, wantStatus: http.StatusBadRequest},
		{name: "制約内なら作成できる", body: `{"category":"技術","question_text":"質問","priority":999}`, existing: 49, wantStatus: http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &interviewCompanyQuestionRepoStub{existing: make([]models.InterviewCompanyQuestion, tt.existing)}
			req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/1/interview-questions", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			ctx := testsupport.NewCtx(req, rec)
			ctx.SetParamNames("id")
			ctx.SetParamValues("1")

			testsupport.AssertStatus(t, newAdminInterviewQuestionController(repo).CreateCompanyQuestion, ctx, tt.wantStatus)
			if repo.createCall != (tt.wantStatus == http.StatusCreated) {
				t.Fatalf("createCall = %v, wantStatus %d", repo.createCall, tt.wantStatus)
			}
		})
	}
}

func TestAdminInterviewController_UpdateCompanyQuestion_EnforcesPortalConstraints(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 1, Category: "技術", QuestionText: "質問"}}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1/interview-questions/10", bytes.NewBufferString(`{"priority":-1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	testsupport.AssertStatus(t, newAdminInterviewQuestionController(repo).UpdateCompanyQuestion, ctx, http.StatusBadRequest)
	if repo.updateCall {
		t.Fatal("範囲外の優先度で更新している")
	}
}

func TestAdminInterviewController_UpdateCompanyQuestion_RejectsDifferentCompany(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 2}}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1/interview-questions/10", bytes.NewBufferString(`{"question_text":"updated"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	testsupport.AssertStatus(t, newAdminInterviewQuestionController(repo).UpdateCompanyQuestion, ctx, http.StatusNotFound)
	if repo.updateCall {
		t.Fatal("question from another company was updated")
	}
}

func TestAdminInterviewController_UpdateCompanyQuestion_RejectsEmptyQuestion(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 1}}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1/interview-questions/10", bytes.NewBufferString(`{"question_text":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	testsupport.AssertStatus(t, newAdminInterviewQuestionController(repo).UpdateCompanyQuestion, ctx, http.StatusBadRequest)
	if repo.updateCall {
		t.Fatal("empty question was updated")
	}
}

func TestAdminInterviewController_DeleteCompanyQuestion_RejectsDifferentCompany(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 2}}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/companies/1/interview-questions/10", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	testsupport.AssertStatus(t, newAdminInterviewQuestionController(repo).DeleteCompanyQuestion, ctx, http.StatusNotFound)
	if repo.deleteCall {
		t.Fatal("question from another company was deleted")
	}
}
