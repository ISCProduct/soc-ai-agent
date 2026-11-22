package admin_test

// SOCAIAGENT-692 企業マスタ（PUT・confirm-info）の URL 検証と、面接質問 generate の AI 予算判定。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/costs"

	"github.com/stretchr/testify/mock"
)

func TestAdminCompanyController_Update_URLScheme(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
		want  int
	}{
		{"website https は可", "website_url", "https://example.com", http.StatusOK},
		{"website http は可", "website_url", "http://example.com/a", http.StatusOK},
		{"logo https は可", "logo_url", "https://example.com/logo.png", http.StatusOK},
		{"website javascript は不可", "website_url", "javascript:alert(1)", http.StatusBadRequest},
		{"website 大文字 JavaScript も不可", "website_url", "JavaScript:alert(1)", http.StatusBadRequest},
		{"logo data は不可", "logo_url", "data:image/png;base64,AAAA", http.StatusBadRequest},
		{"logo ftp は不可", "logo_url", "ftp://example.com/logo.png", http.StatusBadRequest},
		{"website ホストなしは不可", "website_url", "https://", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.CompanyRepositoryMock{}
			audit := &mocks.AuditLogServiceMock{}
			repo.On("FindByID", uint(1)).Return(&models.Company{Name: "Test Corp"}, nil)
			repo.On("Update", mock.Anything).Return(nil).Maybe()
			audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

			body, _ := json.Marshal(map[string]string{tt.field: tt.value})
			req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			ctx := testsupport.NewCtx(req, rec)
			ctx.SetParamNames("id")
			ctx.SetParamValues("1")
			testsupport.AssertStatus(t, admincontrollers.NewAdminCompanyController(repo, audit, nil).Update, ctx, tt.want)
			if tt.want != http.StatusOK {
				repo.AssertNotCalled(t, "Update", mock.Anything)
			}
		})
	}
}

// confirm-info は職員にも開けるため、PUT と同じく URL を http/https に限る。
func TestAdminCompanyController_ConfirmCompanyInfo_URLScheme(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
		want  int
	}{
		{"website https は可", "website_url", "https://example.com", http.StatusOK},
		{"source http は可", "source_url", "http://example.com/about", http.StatusOK},
		{"website javascript は不可", "website_url", "javascript:alert(1)", http.StatusBadRequest},
		{"source data は不可", "source_url", "data:text/html,x", http.StatusBadRequest},
		{"website ホストなしは不可", "website_url", "https://", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.CompanyRepositoryMock{}
			audit := &mocks.AuditLogServiceMock{}
			repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "Test Corp"}, nil).Maybe()
			repo.On("Update", mock.Anything).Return(nil).Maybe()
			audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

			body, _ := json.Marshal(map[string]string{tt.field: tt.value})
			req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/1/confirm-info", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			ctx := testsupport.NewCtx(req, rec)
			ctx.SetParamNames("id")
			ctx.SetParamValues("1")
			// AI は呼ばれない（確定は LLM 再実行なし）。fetcher を作るためだけにクライアントを渡す。
			c := admincontrollers.NewAdminCompanyController(repo, audit, nil, openai.NewWithBaseURL("http://127.0.0.1:1", "gpt-4o-mini"))
			testsupport.AssertStatus(t, c.ConfirmCompanyInfo, ctx, tt.want)
			if tt.want != http.StatusOK {
				repo.AssertNotCalled(t, "Update", mock.Anything)
			}
		})
	}
}

type budgetStub struct{ allowed bool }

func (b budgetStub) Status(context.Context, uint) (costs.AIBudgetStatus, error) {
	return costs.AIBudgetStatus{Allowed: b.allowed}, nil
}

func TestAdminInterviewController_GenerateCompanyQuestions_AIBudget(t *testing.T) {
	tests := []struct {
		name       string
		allowed    bool
		wantCode   int
		wantAICall bool
	}{
		{"予算超過は 429 で AI を呼ばない", false, http.StatusTooManyRequests, false},
		{"予算内なら AI を呼ぶ", true, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var aiCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				aiCalls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			repo := &mocks.CompanyRepositoryMock{}
			repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "Test Corp"}, nil)
			repo.On("FindJobPositionsByCompany", uint(1)).Return([]models.CompanyJobPosition(nil), nil)

			c := admincontrollers.NewAdminInterviewController(nil, nil, nil)
			c.SetCompanyRepo(repo)
			c.SetOpenAIClient(openai.NewWithBaseURL(server.URL, "gpt-4o-mini"))
			c.SetAIBudget(budgetStub{allowed: tt.allowed})

			req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/1/interview-questions/generate", nil)
			req = req.WithContext(context.WithValue(req.Context(), middleware.AdminUserIDContextKey, uint(5)))
			rec := httptest.NewRecorder()
			ctx := testsupport.NewCtx(req, rec)
			ctx.SetParamNames("id")
			ctx.SetParamValues("1")
			_ = c.GenerateCompanyQuestions(ctx)
			if tt.wantCode != 0 && rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := aiCalls.Load() > 0; got != tt.wantAICall {
				t.Fatalf("AI 呼び出し = %v, want %v", got, tt.wantAICall)
			}
		})
	}
}
