package controllers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
)

type interviewCompanyQuestionRepoStub struct {
	question   *models.InterviewCompanyQuestion
	updateCall bool
	deleteCall bool
}

func (r *interviewCompanyQuestionRepoStub) FindByCompanyID(uint) ([]models.InterviewCompanyQuestion, error) {
	return nil, nil
}

func (r *interviewCompanyQuestionRepoStub) FindByCompanyAndPosition(uint, string) ([]models.InterviewCompanyQuestion, error) {
	return nil, nil
}

func (r *interviewCompanyQuestionRepoStub) FindByID(uint) (*models.InterviewCompanyQuestion, error) {
	return r.question, nil
}

func (r *interviewCompanyQuestionRepoStub) Create(*models.InterviewCompanyQuestion) error {
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

func newAdminInterviewQuestionController(repo *interviewCompanyQuestionRepoStub) *controllers.AdminInterviewController {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	c.SetCompanyQuestionRepo(repo)
	return c
}

func TestAdminInterviewController_CreateCompanyQuestion_RejectsWhitespace(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/1/interview-questions", bytes.NewBufferString(`{"category":"   ","question_text":" question "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")

	assertStatus(t, newAdminInterviewQuestionController(repo).CreateCompanyQuestion, ctx, http.StatusBadRequest)
}

func TestAdminInterviewController_UpdateCompanyQuestion_RejectsDifferentCompany(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 2}}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1/interview-questions/10", bytes.NewBufferString(`{"question_text":"updated"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	assertStatus(t, newAdminInterviewQuestionController(repo).UpdateCompanyQuestion, ctx, http.StatusNotFound)
	if repo.updateCall {
		t.Fatal("question from another company was updated")
	}
}

func TestAdminInterviewController_UpdateCompanyQuestion_RejectsEmptyQuestion(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 1}}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1/interview-questions/10", bytes.NewBufferString(`{"question_text":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	assertStatus(t, newAdminInterviewQuestionController(repo).UpdateCompanyQuestion, ctx, http.StatusBadRequest)
	if repo.updateCall {
		t.Fatal("empty question was updated")
	}
}

func TestAdminInterviewController_DeleteCompanyQuestion_RejectsDifferentCompany(t *testing.T) {
	repo := &interviewCompanyQuestionRepoStub{question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 2}}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/companies/1/interview-questions/10", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")

	assertStatus(t, newAdminInterviewQuestionController(repo).DeleteCompanyQuestion, ctx, http.StatusNotFound)
	if repo.deleteCall {
		t.Fatal("question from another company was deleted")
	}
}
