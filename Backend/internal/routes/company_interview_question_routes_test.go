package routes

// #1762 企業ポータルの面接質問ルートの認可テスト。
// 本物のルート登録と認証ミドルウェアを通し、グループ外への登録やオーナー判定の削除で落ちるようにする。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	companycontrollers "Backend/internal/controllers/company"
	"Backend/internal/models"
	"Backend/internal/services/companyportal"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// portalQuestionServiceFake はハンドラまで到達したかを記録する。
type portalQuestionServiceFake struct {
	calls     int
	companyID uint
}

func (f *portalQuestionServiceFake) List(companyID uint) ([]models.InterviewCompanyQuestion, error) {
	f.calls++
	f.companyID = companyID
	return nil, nil
}

func (f *portalQuestionServiceFake) Create(companyID uint, _ companyportal.InterviewQuestionInput) (*models.InterviewCompanyQuestion, error) {
	f.calls++
	f.companyID = companyID
	return &models.InterviewCompanyQuestion{}, nil
}

func (f *portalQuestionServiceFake) Update(_, companyID uint, _ companyportal.InterviewQuestionInput) (*models.InterviewCompanyQuestion, error) {
	f.calls++
	f.companyID = companyID
	return &models.InterviewCompanyQuestion{}, nil
}

func (f *portalQuestionServiceFake) Delete(_, companyID uint) error {
	f.calls++
	f.companyID = companyID
	return nil
}

func TestCompanyPortalInterviewQuestionRoutesAuthorization(t *testing.T) {
	const (
		listPath = "/api/company-portal/interview-questions"
		itemPath = "/api/company-portal/interview-questions/8"
		body     = `{"company_id":99,"category":"技術","question_text":"質問"}`
	)
	const owner, member, anonymous = models.CompanyUserRoleOwner, "member", ""

	tests := []struct {
		name       string
		method     string
		path       string
		role       string
		wantStatus int
	}{
		{name: "未認証は一覧できない", method: http.MethodGet, path: listPath, role: anonymous, wantStatus: http.StatusUnauthorized},
		{name: "未認証は作成できない", method: http.MethodPost, path: listPath, role: anonymous, wantStatus: http.StatusUnauthorized},
		{name: "未認証は更新できない", method: http.MethodPatch, path: itemPath, role: anonymous, wantStatus: http.StatusUnauthorized},
		{name: "未認証は削除できない", method: http.MethodDelete, path: itemPath, role: anonymous, wantStatus: http.StatusUnauthorized},
		{name: "メンバーは一覧できる", method: http.MethodGet, path: listPath, role: member, wantStatus: http.StatusOK},
		{name: "メンバーは作成できない", method: http.MethodPost, path: listPath, role: member, wantStatus: http.StatusForbidden},
		{name: "メンバーは更新できない", method: http.MethodPatch, path: itemPath, role: member, wantStatus: http.StatusForbidden},
		{name: "メンバーは削除できない", method: http.MethodDelete, path: itemPath, role: member, wantStatus: http.StatusForbidden},
		{name: "オーナーは作成できる", method: http.MethodPost, path: listPath, role: owner, wantStatus: http.StatusCreated},
		{name: "オーナーは更新できる", method: http.MethodPatch, path: itemPath, role: owner, wantStatus: http.StatusOK},
		{name: "オーナーは削除できる", method: http.MethodDelete, path: itemPath, role: owner, wantStatus: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, userMock := newCompanyUserRepoMock(t)
			companies, _ := newCompanyRepoMock(t)
			service := &portalQuestionServiceFake{}
			e := echo.New()
			SetupCompanyAuthRoutes(
				e.Group("/api"),
				&companycontrollers.CompanyAuthController{},
				&companycontrollers.CompanyPortalController{},
				companycontrollers.NewCompanyPortalInterviewQuestionController(service),
				&companycontrollers.CompanyStudentController{},
				nil, nil, nil, nil,
				&companycontrollers.CompanyPortalScoutController{},
				testCompanySecret,
				users,
				companies,
			)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.role != anonymous {
				req.Header.Set("X-Company-User-Token", validCompanyToken(t))
				userMock.ExpectQuery("SELECT \\* FROM `company_users`").
					WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "role"}).
						AddRow(1, 10, "hr@example.com", "hashed", tt.role))
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus >= http.StatusBadRequest {
				assert.Zero(t, service.calls, "権限のない呼び出しがサービスへ到達している")
				return
			}
			assert.Equal(t, 1, service.calls)
			// リクエストの company_id ではなく、認証した企業ユーザーの企業を使う
			assert.Equal(t, uint(10), service.companyID)
		})
	}
}
