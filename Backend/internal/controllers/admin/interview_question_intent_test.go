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

// intentCapturingRepo は作成された質問を記録する。
type intentCapturingRepo struct {
	interviewCompanyQuestionRepoStub
	created *models.InterviewCompanyQuestion
}

func (r *intentCapturingRepo) Create(q *models.InterviewCompanyQuestion) error {
	r.created = q
	return nil
}

func intentController(repo *intentCapturingRepo) *admincontrollers.AdminInterviewController {
	c := admincontrollers.NewAdminInterviewController(nil, nil, nil)
	c.SetCompanyQuestionRepo(repo)
	return c
}

func postQuestion(t *testing.T, repo *intentCapturingRepo, body string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/1/interview-questions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, intentController(repo).CreateCompanyQuestion, ctx, want)
}

func putQuestion(t *testing.T, repo *intentCapturingRepo, body string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1/interview-questions/10", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())
	ctx.SetParamNames("id", "qid")
	ctx.SetParamValues("1", "10")
	testsupport.AssertStatus(t, intentController(repo).UpdateCompanyQuestion, ctx, want)
}

// #1092: 作成時の評価したい観点は前後の空白を落として保存し、空なら未設定（nil）にする。
func TestCreateCompanyQuestion_EvaluationIntent(t *testing.T) {
	repo := &intentCapturingRepo{}
	postQuestion(t, repo, `{"category":"志望動機","question_text":"志望理由は","evaluation_intent":"  困難に直面したときの粘り強さ  "}`, http.StatusCreated)
	if repo.created == nil || repo.created.EvaluationIntent == nil || *repo.created.EvaluationIntent != "困難に直面したときの粘り強さ" {
		t.Fatalf("観点が保存されていない: %+v", repo.created)
	}

	repo = &intentCapturingRepo{}
	postQuestion(t, repo, `{"category":"志望動機","question_text":"志望理由は","evaluation_intent":"   "}`, http.StatusCreated)
	if repo.created == nil || repo.created.EvaluationIntent != nil {
		t.Fatalf("空白だけの観点は未設定（nil）にする: %+v", repo.created)
	}
}

// 上限は300字。300字は通り、301字は 400。
func TestCreateCompanyQuestion_EvaluationIntentLimit(t *testing.T) {
	postQuestion(t, &intentCapturingRepo{}, `{"category":"志望動機","question_text":"q","evaluation_intent":"`+strings.Repeat("あ", 300)+`"}`, http.StatusCreated)
	postQuestion(t, &intentCapturingRepo{}, `{"category":"志望動機","question_text":"q","evaluation_intent":"`+strings.Repeat("あ", 301)+`"}`, http.StatusBadRequest)
}

// 更新は指定したときだけ変える。空文字で消え、指定しなければそのまま。
func TestUpdateCompanyQuestion_EvaluationIntent(t *testing.T) {
	old := "旧い観点"
	repo := &intentCapturingRepo{interviewCompanyQuestionRepoStub: interviewCompanyQuestionRepoStub{
		question: &models.InterviewCompanyQuestion{ID: 10, CompanyID: 1, Category: "志望動機", QuestionText: "q", EvaluationIntent: &old},
	}}

	putQuestion(t, repo, `{"question_text":"更新後"}`, http.StatusOK)
	if repo.question.EvaluationIntent == nil || *repo.question.EvaluationIntent != "旧い観点" {
		t.Fatalf("指定していないのに観点が変わった: %v", repo.question.EvaluationIntent)
	}

	putQuestion(t, repo, `{"evaluation_intent":" 新しい観点 "}`, http.StatusOK)
	if repo.question.EvaluationIntent == nil || *repo.question.EvaluationIntent != "新しい観点" {
		t.Fatalf("観点が更新されていない: %v", repo.question.EvaluationIntent)
	}

	putQuestion(t, repo, `{"evaluation_intent":""}`, http.StatusOK)
	if repo.question.EvaluationIntent != nil {
		t.Fatalf("空文字で観点が消えていない: %v", *repo.question.EvaluationIntent)
	}

	putQuestion(t, repo, `{"evaluation_intent":"`+strings.Repeat("あ", 301)+`"}`, http.StatusBadRequest)
}
