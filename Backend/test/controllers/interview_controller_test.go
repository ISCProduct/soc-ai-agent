package controllers_test

// InterviewControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run Interview -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
)

func newInterviewController(svc *mocks.InterviewServiceMock) *controllers.InterviewController {
	return controllers.NewInterviewController(svc, nil, nil)
}

// ---- Create ----

func TestInterviewController_Create_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newInterviewController(nil).Create, newCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newInterviewController(nil).Create, newCtx(req, rec), http.StatusBadRequest)
}

func TestInterviewController_Create_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"language": "ja", "interviewer_gender": "female"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateSession", uint(1), "ja", "female").Return(&services.InterviewSessionResponse{ID: 10}, nil)
	assertStatus(t, newInterviewController(svc).Create, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Create_ServiceError(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"language": "ja"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateSession", uint(1), "ja", "").Return(nil, errors.New("guest limit exceeded"))
	assertStatus(t, newInterviewController(svc).Create, newCtx(req, rec), http.StatusBadRequest)
}

// ---- List ----

func TestInterviewController_List_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newInterviewController(nil).List, newCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_List_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews?page=1&limit=10", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessions", uint(1), false, 10, 0).Return([]services.InterviewSessionResponse{{ID: 1}}, int64(1), nil)
	assertStatus(t, newInterviewController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_List_LimitCapped(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews?limit=999", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	// limit は100に切り捨てられる
	svc.On("ListSessions", uint(1), false, 100, 0).Return([]services.InterviewSessionResponse{}, int64(0), nil)
	assertStatus(t, newInterviewController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_List_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("ListSessions", uint(1), false, 20, 0).Return([]services.InterviewSessionResponse{}, int64(0), errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).List, newCtx(req, rec), http.StatusForbidden)
}

// ---- Get ----

func TestInterviewController_Get_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).Get, c, http.StatusUnauthorized)
}

func TestInterviewController_Get_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/abc", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("abc")
	assertStatus(t, newInterviewController(nil).Get, c, http.StatusBadRequest)
}

func TestInterviewController_Get_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/5", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetSessionDetailWithRole", uint(1), uint(5), "student").Return(&services.InterviewDetailResponse{}, nil)
	assertStatus(t, newInterviewController(svc).Get, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Get_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/5", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetSessionDetailWithRole", uint(1), uint(5), "student").Return(nil, errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).Get, c, http.StatusForbidden)
}

// ---- Start ----

func TestInterviewController_Start_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/start", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).Start, c, http.StatusUnauthorized)
}

func TestInterviewController_Start_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/start", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("StartSession", uint(1), uint(3)).Return(&services.InterviewSessionResponse{ID: 3}, nil)
	assertStatus(t, newInterviewController(svc).Start, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Start_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/start", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("StartSession", uint(1), uint(3)).Return(nil, errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).Start, c, http.StatusForbidden)
}

// ---- Finish ----

func TestInterviewController_Finish_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/finish", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).Finish, c, http.StatusUnauthorized)
}

func TestInterviewController_Finish_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/finish", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("FinishSession", uint(1), uint(3)).Return(&services.InterviewSessionResponse{ID: 3}, nil)
	assertStatus(t, newInterviewController(svc).Finish, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_Finish_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/finish", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("FinishSession", uint(1), uint(3)).Return(nil, errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).Finish, c, http.StatusForbidden)
}

// ---- GetTrend ----

func TestInterviewController_GetTrend_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/trend", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newInterviewController(nil).GetTrend, newCtx(req, rec), http.StatusUnauthorized)
}

func TestInterviewController_GetTrend_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/trend?limit=5", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetTrend", uint(1), 5).Return([]services.InterviewTrendPoint{{SessionID: 1}}, nil)
	assertStatus(t, newInterviewController(svc).GetTrend, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "points")
	svc.AssertExpectations(t)
}

// ---- GetReport ----

func TestInterviewController_GetReport_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1/report", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).GetReport, c, http.StatusUnauthorized)
}

func TestInterviewController_GetReport_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1/report", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetReport", uint(1), uint(1)).Return(nil, nil)
	assertStatus(t, newInterviewController(svc).GetReport, c, http.StatusNotFound)
}

func TestInterviewController_GetReport_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/report", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetReport", uint(1), uint(2)).Return(&models.InterviewReport{SessionID: 2}, nil)
	assertStatus(t, newInterviewController(svc).GetReport, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_GetReport_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/report", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetReport", uint(1), uint(2)).Return(nil, errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).GetReport, c, http.StatusForbidden)
}

// ---- SendReport ----

func TestInterviewController_SendReport_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/send-report", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).SendReport, c, http.StatusUnauthorized)
}

func TestInterviewController_SendReport_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(nil)
	assertStatus(t, newInterviewController(svc).SendReport, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_SendReport_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(errors.New("report not found"))
	assertStatus(t, newInterviewController(svc).SendReport, c, http.StatusNotFound)
}

func TestInterviewController_SendReport_GuestForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/2/send-report", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SendReportEmail", uint(1), uint(2)).Return(errors.New("guest users cannot receive email reports"))
	assertStatus(t, newInterviewController(svc).SendReport, c, http.StatusForbidden)
}

// ---- AddUtterance ----

func TestInterviewController_AddUtterance_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/utterances", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).AddUtterance, c, http.StatusUnauthorized)
}

func TestInterviewController_AddUtterance_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"role": "user", "text": "自己紹介をします"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/utterances", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SaveUtterance", uint(1), uint(3), "user", "自己紹介をします").Return(nil)
	assertStatus(t, newInterviewController(svc).AddUtterance, c, http.StatusNoContent)
	svc.AssertExpectations(t)
}

func TestInterviewController_AddUtterance_Forbidden(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"role": "user", "text": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/3/utterances", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	svc := &mocks.InterviewServiceMock{}
	svc.On("SaveUtterance", uint(1), uint(3), "user", "hello").Return(errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).AddUtterance, c, http.StatusForbidden)
}

// ---- GetPhraseSuggestions ----

func TestInterviewController_GetPhraseSuggestions_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/1/phrase-suggestions", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newInterviewController(nil).GetPhraseSuggestions, c, http.StatusUnauthorized)
}

func TestInterviewController_GetPhraseSuggestions_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/phrase-suggestions", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetPhraseSuggestions", req.Context(), uint(1), uint(2)).
		Return([]services.PhraseSuggestion{{Original: "頑張ります", Suggestions: []string{"尽力します"}}}, nil)
	assertStatus(t, newInterviewController(svc).GetPhraseSuggestions, c, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestInterviewController_GetPhraseSuggestions_Forbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/interviews/2/phrase-suggestions", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	svc := &mocks.InterviewServiceMock{}
	svc.On("GetPhraseSuggestions", req.Context(), uint(1), uint(2)).
		Return([]services.PhraseSuggestion{}, errors.New("forbidden"))
	assertStatus(t, newInterviewController(svc).GetPhraseSuggestions, c, http.StatusForbidden)
}

// ---- UploadVideo (サービス未設定パス) ----

func TestInterviewController_UploadVideo_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/upload-video", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	// videoRepo/s3Service がnilのとき ServiceUnavailable
	ctrl := controllers.NewInterviewController(nil, nil, nil)
	// まずInvalidID確認（ParseMultipartFormより前）
	c2 := newCtx(req, rec)
	c2.SetParamNames("id")
	c2.SetParamValues("abc")
	assertStatus(t, ctrl.UploadVideo, c2, http.StatusBadRequest)
}

func TestInterviewController_UploadVideo_ServiceUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/1/upload-video", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	// videoRepo/s3Service がnilのとき ServiceUnavailable を返す
	ctrl := controllers.NewInterviewController(nil, nil, nil)
	assertStatus(t, ctrl.UploadVideo, c, http.StatusServiceUnavailable)
}
