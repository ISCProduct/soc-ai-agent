package interview_test

// InterviewControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./internal/controllers/interview/... -run Interview -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	interviewcontrollers "Backend/internal/controllers/interview"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/interview"
	"Backend/internal/services/shared"
	"Backend/internal/services/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

func newInterviewController(svc *mocks.InterviewServiceMock) *interviewcontrollers.InterviewController {
	return interviewcontrollers.NewInterviewController(svc, nil, nil)
}

// ---- Create ----

func TestInterviewController_Create_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newInterviewController(nil).Create, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newInterviewController(nil).Create, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestInterviewController_Create_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"language": "ja", "interviewer_gender": "female"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateSession", uint(1), "ja", "female").Return(&interview.InterviewSessionResponse{ID: 10}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).Create, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Create_ServiceError(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"language": "ja"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateSession", uint(1), "ja", "").Return(nil, errors.New("guest limit exceeded"))
	testsupport.AssertStatus(t, newInterviewController(svc).Create, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

// ---- List ----

func TestInterviewController_List_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newInterviewController(nil).List, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_List_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews?page=1&limit=10", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessions", uint(1), false, 10, 0).Return([]interview.InterviewSessionResponse{{ID: 1}}, int64(1), nil)
	testsupport.AssertStatus(t, newInterviewController(svc).List, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_List_LimitCapped(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews?limit=999", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	// limit は100に切り捨てられる
	svc.On("ListSessions", uint(1), false, 100, 0).Return([]interview.InterviewSessionResponse{}, int64(0), nil)
	testsupport.AssertStatus(t, newInterviewController(svc).List, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_List_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessions", uint(1), false, 20, 0).Return([]interview.InterviewSessionResponse{}, int64(0), shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).List, testsupport.NewCtx(req, rec), http.StatusForbidden)
}

func TestInterviewController_HRList_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/interviews?company_id=10", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newInterviewController(nil).HRList, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_HRList_MissingCompanyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/interviews", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newInterviewController(nil).HRList, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestInterviewController_HRList_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/interviews?company_id=99", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessionsForOwner", uint(1), uint(99), 20, 0).Return([]interview.InterviewSessionResponse{}, int64(0), shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).HRList, testsupport.NewCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestInterviewController_HRList_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/interviews?company_id=10", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessionsForOwner", uint(1), uint(10), 20, 0).Return([]interview.InterviewSessionResponse{{ID: 3}}, int64(1), nil)
	testsupport.AssertStatus(t, newInterviewController(svc).HRList, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_HRList_InternalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/interviews?company_id=10", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessionsForOwner", uint(1), uint(10), 20, 0).Return([]interview.InterviewSessionResponse{}, int64(0), errors.New("db down"))
	testsupport.AssertStatus(t, newInterviewController(svc).HRList, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// ---- Get ----

func TestInterviewController_Get_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).Get, c, http.StatusUnauthorized)
}

func TestInterviewController_Get_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/abc", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("abc")
	testsupport.AssertStatus(t, newInterviewController(nil).Get, c, http.StatusBadRequest)
}

func TestInterviewController_Get_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/5", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetSessionDetailWithRole", uint(1), uint(5), "student").Return(&interview.InterviewDetailResponse{}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).Get, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Get_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/5", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetSessionDetailWithRole", uint(1), uint(5), "student").Return(nil, shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).Get, c, http.StatusForbidden)
}

// ---- Start ----

func TestInterviewController_Start_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/start", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).Start, c, http.StatusUnauthorized)
}

func TestInterviewController_Start_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/start", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("StartSession", uint(1), uint(3)).Return(&interview.InterviewSessionResponse{ID: 3}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).Start, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Start_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/start", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("StartSession", uint(1), uint(3)).Return(nil, shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).Start, c, http.StatusForbidden)
}

// ---- Finish ----

func TestInterviewController_Finish_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/finish", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).Finish, c, http.StatusUnauthorized)
}

func TestInterviewController_Finish_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/finish", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("FinishSession", uint(1), uint(3)).Return(&interview.InterviewSessionResponse{ID: 3}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).Finish, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Finish_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/finish", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("FinishSession", uint(1), uint(3)).Return(nil, shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).Finish, c, http.StatusForbidden)
}

// ---- GetTrend ----

func TestInterviewController_GetTrend_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/trend", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newInterviewController(nil).GetTrend, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_GetTrend_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/trend?limit=5", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetTrend", uint(1), 5).Return([]interview.InterviewTrendPoint{{SessionID: 1}}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).GetTrend, testsupport.NewCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "points")
	svc.AssertExpectations(t)
}

// TestInterviewController_GetTrend_LimitCap は limit クエリの頭打ちを固定する(#1478)。
//
// 上限が無いと limit=1000000 がそのままサービスへ渡り、完了済みセッション全件が
// 読まれる。一覧系(List/HRList)と同じ 100 に揃える。
func TestInterviewController_GetTrend_LimitCap(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "未指定は既定の20", query: "", wantLimit: 20},
		{name: "上限内はそのまま", query: "?limit=5", wantLimit: 5},
		{name: "上限ちょうど", query: "?limit=100", wantLimit: 100},
		{name: "上限超過は100へ頭打ち", query: "?limit=101", wantLimit: 100},
		{name: "極端な値も100へ頭打ち", query: "?limit=1000000", wantLimit: 100},
		{name: "0は既定の20", query: "?limit=0", wantLimit: 20},
		{name: "負値は既定の20", query: "?limit=-1", wantLimit: 20},
		{name: "数値以外は既定の20", query: "?limit=abc", wantLimit: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/interviews/trend"+tt.query, nil)
			req = testsupport.WithUserID(req, 1)
			rec := httptest.NewRecorder()

			svc := &mocks.InterviewServiceMock{}
			svc.On("GetTrend", uint(1), tt.wantLimit).Return([]interview.InterviewTrendPoint{}, nil)
			testsupport.AssertStatus(t, newInterviewController(svc).GetTrend, testsupport.NewCtx(req, rec), http.StatusOK)
			svc.AssertExpectations(t)
		})
	}
}

// ---- GetReport ----

func TestInterviewController_GetReport_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1/report", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).GetReport, c, http.StatusUnauthorized)
}

func TestInterviewController_GetReport_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1/report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetReport", uint(1), uint(1)).Return(nil, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).GetReport, c, http.StatusNotFound)
}

func TestInterviewController_GetReport_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetReport", uint(1), uint(2)).Return(&models.InterviewReport{SessionID: 2}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).GetReport, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_GetReport_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetReport", uint(1), uint(2)).Return(nil, shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).GetReport, c, http.StatusForbidden)
}

// ---- SendReport ----

func TestInterviewController_SendReport_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/send-report", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).SendReport, c, http.StatusUnauthorized)
}

func TestInterviewController_SendReport_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(nil)
	testsupport.AssertStatus(t, newInterviewController(svc).SendReport, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_SendReport_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(errors.New("report not found"))
	testsupport.AssertStatus(t, newInterviewController(svc).SendReport, c, http.StatusNotFound)
}

func TestInterviewController_SendReport_GuestForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(errors.New("guest users cannot receive email reports"))
	testsupport.AssertStatus(t, newInterviewController(svc).SendReport, c, http.StatusForbidden)
}

// #939: 他ユーザーのセッションを指定した場合はサービス層が返す forbidden エラーを403にマッピングする。
func TestInterviewController_SendReport_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).SendReport, c, http.StatusForbidden)
}

// ---- AddUtterance ----

func TestInterviewController_AddUtterance_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/utterances", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).AddUtterance, c, http.StatusUnauthorized)
}

func TestInterviewController_AddUtterance_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"role": "user", "text": "自己紹介をします"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/utterances", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SaveUtterance", uint(1), uint(3), "user", "自己紹介をします").Return(nil)
	testsupport.AssertStatus(t, newInterviewController(svc).AddUtterance, c, http.StatusNoContent)
	svc.AssertExpectations(t)
}

func TestInterviewController_AddUtterance_Forbidden(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"role": "user", "text": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/utterances", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SaveUtterance", uint(1), uint(3), "user", "hello").Return(shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).AddUtterance, c, http.StatusForbidden)
}

// ---- GetPhraseSuggestions ----

func TestInterviewController_GetPhraseSuggestions_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1/phrase-suggestions", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	testsupport.AssertStatus(t, newInterviewController(nil).GetPhraseSuggestions, c, http.StatusUnauthorized)
}

func TestInterviewController_GetPhraseSuggestions_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/phrase-suggestions", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetPhraseSuggestions", req.Context(), uint(1), uint(2)).
		Return([]interview.PhraseSuggestion{{Original: "頑張ります", Suggestions: []string{"尽力します"}}}, nil)
	testsupport.AssertStatus(t, newInterviewController(svc).GetPhraseSuggestions, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_GetPhraseSuggestions_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/phrase-suggestions", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetPhraseSuggestions", req.Context(), uint(1), uint(2)).
		Return([]interview.PhraseSuggestion{}, shared.ErrForbidden)
	testsupport.AssertStatus(t, newInterviewController(svc).GetPhraseSuggestions, c, http.StatusForbidden)
}

// ---- UploadVideo (サービス未設定パス) ----

func TestInterviewController_UploadVideo_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/upload-video", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	// videoRepo/s3Service がnilのとき ServiceUnavailable
	ctrl := interviewcontrollers.NewInterviewController(nil, nil, nil)
	// まずInvalidID確認（ParseMultipartFormより前）
	c2 := testsupport.NewCtx(req, rec)
	c2.SetParamNames("id")
	c2.SetParamValues("abc")
	testsupport.AssertStatus(t, ctrl.UploadVideo, c2, http.StatusBadRequest)
}

func TestInterviewController_UploadVideo_ServiceUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/upload-video", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	// videoRepo/s3Service がnilのとき ServiceUnavailable を返す
	ctrl := interviewcontrollers.NewInterviewController(nil, nil, nil)
	testsupport.AssertStatus(t, ctrl.UploadVideo, c, http.StatusServiceUnavailable)
}

// TestInterviewController_UploadVideo_Forbidden は #941 の回帰テスト。
// 他人の面接セッションIDを指定した場合、ファイル解析やS3アップロードに進む前に
// 403を返し、動画レコードも作成されないことを検証する。
func TestInterviewController_UploadVideo_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/9/upload-video", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("9")

	svc := &mocks.InterviewServiceMock{}
	svc.On("EnsureSessionOwnership", uint(1), uint(9)).Return(shared.ErrForbidden)
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	// S3UploadServiceは未設定でもゼロ値で構築できる（メソッド呼び出しに到達しないことを検証するため）
	ctrl := interviewcontrollers.NewInterviewController(svc, videoRepo, &storage.S3UploadService{})

	testsupport.AssertStatus(t, ctrl.UploadVideo, c, http.StatusForbidden)
	svc.AssertExpectations(t)
	// 所有権チェックで弾かれるため、動画レコード作成は一切行われない
	videoRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestInterviewController_UploadVideo_SessionNotFound は未存在セッションを安全な404で返す。
func TestInterviewController_UploadVideo_SessionNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/9/upload-video", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("9")

	svc := &mocks.InterviewServiceMock{}
	svc.On("EnsureSessionOwnership", uint(1), uint(9)).Return(gorm.ErrRecordNotFound)
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	ctrl := interviewcontrollers.NewInterviewController(svc, videoRepo, &storage.S3UploadService{})

	testsupport.AssertStatus(t, ctrl.UploadVideo, c, http.StatusNotFound)
	svc.AssertExpectations(t)
	videoRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestInterviewController_UploadVideo_OwnershipCheckError は想定外の所有権確認エラーを500にする。
func TestInterviewController_UploadVideo_OwnershipCheckError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/9/upload-video", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("9")

	svc := &mocks.InterviewServiceMock{}
	svc.On("EnsureSessionOwnership", uint(1), uint(9)).Return(errors.New("db unavailable"))
	videoRepo := &mocks.InterviewVideoRepositoryMock{}
	ctrl := interviewcontrollers.NewInterviewController(svc, videoRepo, &storage.S3UploadService{})

	testsupport.AssertStatus(t, ctrl.UploadVideo, c, http.StatusInternalServerError)
	svc.AssertExpectations(t)
	videoRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}
