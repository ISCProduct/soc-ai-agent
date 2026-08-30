package controllers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	hrsvc "Backend/internal/services/hr"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

type hrStudentAnalysisServiceStub struct {
	resp *hrsvc.StudentAnalysisResponse
	err  error
}

func (s *hrStudentAnalysisServiceStub) GetAnalysis(ownerUserID, companyID, targetUserID uint) (*hrsvc.StudentAnalysisResponse, error) {
	return s.resp, s.err
}

func newHRStudentAnalysisController(svc *hrStudentAnalysisServiceStub) *controllers.HRStudentAnalysisController {
	return controllers.NewHRStudentAnalysisController(svc)
}

func hrAnalysisCtx(req *http.Request, rec *httptest.ResponseRecorder, userID string) echo.Context {
	c := newCtx(req, rec)
	c.SetParamNames("userID")
	c.SetParamValues(userID)
	return c
}

func TestHRStudentAnalysisController_GetAnalysis_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/students/5/analysis?company_id=10", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newHRStudentAnalysisController(nil).GetAnalysis, newCtx(req, rec), http.StatusUnauthorized)
}

func TestHRStudentAnalysisController_GetAnalysis_MissingCompanyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/students/5/analysis", nil)
	req = withUserID(req, 2)
	rec := httptest.NewRecorder()
	assertStatus(t, newHRStudentAnalysisController(nil).GetAnalysis, newCtx(req, rec), http.StatusBadRequest)
}

func TestHRStudentAnalysisController_GetAnalysis_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/students/5/analysis?company_id=99", nil)
	req = withUserID(req, 2)
	rec := httptest.NewRecorder()
	svc := &hrStudentAnalysisServiceStub{err: shared.ErrForbidden}
	assertStatus(t, newHRStudentAnalysisController(svc).GetAnalysis, hrAnalysisCtx(req, rec, "5"), http.StatusForbidden)
}

func TestHRStudentAnalysisController_GetAnalysis_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/students/5/analysis?company_id=10", nil)
	req = withUserID(req, 2)
	rec := httptest.NewRecorder()
	svc := &hrStudentAnalysisServiceStub{err: hrsvc.ErrStudentNotVisible}
	assertStatus(t, newHRStudentAnalysisController(svc).GetAnalysis, hrAnalysisCtx(req, rec, "5"), http.StatusNotFound)
}

func TestHRStudentAnalysisController_GetAnalysis_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/students/5/analysis?company_id=10", nil)
	req = withUserID(req, 2)
	rec := httptest.NewRecorder()
	svc := &hrStudentAnalysisServiceStub{
		resp: &hrsvc.StudentAnalysisResponse{
			UserID:            5,
			IntegratedProfile: &flywheel.UserIntegratedProfile{UserID: 5},
			InterviewReports:  []hrsvc.InterviewReportView{},
		},
	}
	assertStatus(t, newHRStudentAnalysisController(svc).GetAnalysis, hrAnalysisCtx(req, rec, "5"), http.StatusOK)
}

func TestHRStudentAnalysisController_GetAnalysis_InternalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/students/5/analysis?company_id=10", nil)
	req = withUserID(req, 2)
	rec := httptest.NewRecorder()
	svc := &hrStudentAnalysisServiceStub{err: errors.New("boom")}
	assertStatus(t, newHRStudentAnalysisController(svc).GetAnalysis, hrAnalysisCtx(req, rec, "5"), http.StatusInternalServerError)
}
