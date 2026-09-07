package controllers_test

// #1030 GET /api/resume/status のHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run ResumeStatus -v

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/services/resume"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
)

func TestResumeStatus_Unauthorized(t *testing.T) {
	// 認証コンテキストが無い場合は401。サービスは呼ばれない。
	svc := &mocks.ResumeServiceMock{}
	req := httptest.NewRequest(http.MethodGet, "/api/resume/status", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(svc).Status, newCtx(req, rec), http.StatusUnauthorized)
	svc.AssertNotCalled(t, "GetResumeStatus")
}

// TestResumeStatus_UsesAuthenticatedUserID は他人のIDを参照できないことを検証する。
// クエリで別ユーザーを指定しても、トークンのIDだけが使われること。
func TestResumeStatus_UsesAuthenticatedUserID(t *testing.T) {
	score := 45
	svc := &mocks.ResumeServiceMock{}
	svc.On("GetResumeStatus", uint(7)).
		Return(&resume.ResumeStatus{HasDocument: true, LatestScore: &score, NeedsAttention: true}, nil)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/resume/status?user_id=999", nil), 7)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(svc).Status, newCtx(req, rec), http.StatusOK)

	// クエリの999ではなく、認証済みの7で呼ばれていること。
	svc.AssertCalled(t, "GetResumeStatus", uint(7))
	svc.AssertNotCalled(t, "GetResumeStatus", uint(999))

	var got resume.ResumeStatus
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.HasDocument)
	assert.True(t, got.NeedsAttention)
	if assert.NotNil(t, got.LatestScore) {
		assert.Equal(t, 45, *got.LatestScore)
	}
}

func TestResumeStatus_NoDocumentSerializesNullScore(t *testing.T) {
	// 未提出時に latest_score が JSON の null になること（フロントが null を期待する）。
	svc := &mocks.ResumeServiceMock{}
	svc.On("GetResumeStatus", uint(3)).
		Return(&resume.ResumeStatus{HasDocument: false, LatestScore: nil, NeedsAttention: true}, nil)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/resume/status", nil), 3)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(svc).Status, newCtx(req, rec), http.StatusOK)

	assert.JSONEq(t, `{"has_document":false,"latest_score":null,"needs_attention":true}`, rec.Body.String())
}

func TestResumeStatus_ServiceError(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("GetResumeStatus", uint(7)).Return(nil, errors.New("db down"))

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/resume/status", nil), 7)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(svc).Status, newCtx(req, rec), http.StatusInternalServerError)
}
