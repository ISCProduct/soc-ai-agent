package admin_test

// 管理画面ユーザー一覧/CSV出力のセッションID取得がバッチ1回で済むことを固定するテスト。
//
// 実行: cd Backend && go test ./internal/controllers/admin/... -run "UsersBatch" -v

import (
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func scoreReport(sessionID uint, score int) models.InterviewReport {
	b, _ := json.Marshal(map[string]int{"a": score})
	return models.InterviewReport{SessionID: sessionID, ScoresJSON: string(b)}
}

// ユーザー1: セッション10,11(スコア4,2 → 平均3)、ユーザー2: セッション20(スコア5)、ユーザー3: セッションなし
func setupUsersBatch(t *testing.T) (*mocks.UserRepositoryMock, *mocks.DashboardSessionRepoMock, *mocks.DashboardReportRepoMock) {
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}
	users := []models.User{{ID: 1, Name: "u1"}, {ID: 2, Name: "u2"}, {ID: 3, Name: "u3"}}
	userRepo.On("ListUsersPaged", mock.Anything, 0, "", mock.Anything).Return(users, int64(3), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1, 2, 3}).Return(map[uint]repositories.UserSessionStat{}, nil)
	sessRepo.On("ListFinishedSessionIDsByUsers", []uint{1, 2, 3}).Return(map[uint][]uint{1: {10, 11}, 2: {20}}, nil)
	repRepo.On("FindBySessionIDs", mock.Anything).Return([]models.InterviewReport{
		scoreReport(10, 4), scoreReport(11, 2), scoreReport(20, 5),
	}, nil)
	return userRepo, sessRepo, repRepo
}

func TestAdminDashboardController_UsersBatch_ListUsers(t *testing.T) {
	userRepo, sessRepo, repRepo := setupUsersBatch(t)
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil), nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ListUsers, testsupport.NewCtx(req, rec), http.StatusOK)

	var body struct {
		Users []struct {
			UserID   uint     `json:"user_id"`
			AvgScore *float64 `json:"avg_score"`
		} `json:"users"`
	}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	got := map[uint]*float64{}
	for _, u := range body.Users {
		got[u.UserID] = u.AvgScore
	}
	if assert.Len(t, got, 3) {
		assert.InDelta(t, 3.0, *got[1], 0.001)
		assert.InDelta(t, 5.0, *got[2], 0.001)
		assert.Nil(t, got[3])
	}
	sessRepo.AssertNumberOfCalls(t, "ListFinishedSessionIDsByUsers", 1)
	sessRepo.AssertNumberOfCalls(t, "ListFinishedSessionIDsByUser", 0)
}

func TestAdminDashboardController_UsersBatch_ExportCSV(t *testing.T) {
	userRepo, sessRepo, repRepo := setupUsersBatch(t)
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), nil)
	rec := httptest.NewRecorder()
	ctrl := newAdminDashboardController(userRepo, sessRepo, repRepo)
	// プラン判定は未設定時のDEFAULT_PLAN(pro)で通る
	testsupport.AssertStatus(t, ctrl.ExportCSV, testsupport.NewCtx(req, rec), http.StatusOK)

	out := rec.Body.String()
	assert.Contains(t, out, "1,u1,,")
	assert.Contains(t, out, ",3.00\n")
	assert.Contains(t, out, ",5.00\n")
	sessRepo.AssertNumberOfCalls(t, "ListFinishedSessionIDsByUsers", 1)
	sessRepo.AssertNumberOfCalls(t, "ListFinishedSessionIDsByUser", 0)
}

// セッションID取得に失敗しても従来どおりスコア無し(null)で200を返す
func TestAdminDashboardController_UsersBatch_SessionIDsError(t *testing.T) {
	userRepo, sessRepo, repRepo := setupUsersBatch(t)
	sessRepo.ExpectedCalls = nil
	sessRepo.On("GetUserStatsBatch", []uint{1, 2, 3}).Return(map[uint]repositories.UserSessionStat{}, nil)
	sessRepo.On("ListFinishedSessionIDsByUsers", []uint{1, 2, 3}).Return(nil, errors.New("db error"))
	repRepo.ExpectedCalls = nil
	repRepo.On("FindBySessionIDs", []uint(nil)).Return([]models.InterviewReport{}, nil)

	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil), nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ListUsers, testsupport.NewCtx(req, rec), http.StatusOK)
}

// ユーザーが1000件を超えても分割して取得し、失敗した範囲だけ空欄にして成功した範囲の平均は出す
func TestAdminDashboardController_UsersBatch_ExportCSV_ChunkedPartialFailure(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}
	ids := make([]uint, 2500)
	users := make([]models.User, 2500)
	for i := range users {
		ids[i] = uint(i + 1)
		users[i] = models.User{ID: ids[i], Name: "u"}
	}
	userRepo.On("ListUsersPaged", mock.Anything, 0, "", mock.Anything).Return(users, int64(2500), nil)
	sessRepo.On("GetUserStatsBatch", ids).Return(map[uint]repositories.UserSessionStat{}, nil)
	// 1〜1000: 成功(ユーザー1のみセッション10) / 1001〜2000: 失敗 / 2001〜2500: 成功(ユーザー2001のみセッション20)
	sessRepo.On("ListFinishedSessionIDsByUsers", ids[:1000]).Return(map[uint][]uint{1: {10}}, nil)
	sessRepo.On("ListFinishedSessionIDsByUsers", ids[1000:2000]).Return(nil, errors.New("db error"))
	sessRepo.On("ListFinishedSessionIDsByUsers", ids[2000:]).Return(map[uint][]uint{2001: {20}}, nil)
	repRepo.On("FindBySessionIDs", mock.Anything).Return([]models.InterviewReport{scoreReport(10, 4), scoreReport(20, 2)}, nil)

	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), nil)
	rec := httptest.NewRecorder()
	ctrl := newAdminDashboardController(userRepo, sessRepo, repRepo)
	testsupport.AssertStatus(t, ctrl.ExportCSV, testsupport.NewCtx(req, rec), http.StatusOK)

	out := rec.Body.String()
	assert.Contains(t, out, ",4.00\n")
	assert.Contains(t, out, ",2.00\n")
	sessRepo.AssertNumberOfCalls(t, "ListFinishedSessionIDsByUsers", 3)
}
