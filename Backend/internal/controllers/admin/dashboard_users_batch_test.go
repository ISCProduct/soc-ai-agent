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
	"strings"
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

// セッションIDが1000件を超えてもレポートを1000件ずつ取得する。
// 2つ目の範囲の取得に失敗したとき、CSVは失敗した範囲のユーザーだけ空欄にし、一覧は500を返す。
func TestAdminDashboardController_UsersBatch_ReportsChunkedPartialFailure(t *testing.T) {
	// ユーザー1: セッション1〜1000(全てスコア4) / ユーザー2: セッション1001(取得失敗する範囲)
	sessIDs1 := make([]uint, 1000)
	for i := range sessIDs1 {
		sessIDs1[i] = uint(i + 1)
	}
	reports1 := make([]models.InterviewReport, len(sessIDs1))
	for i, id := range sessIDs1 {
		reports1[i] = scoreReport(id, 4)
	}
	setup := func() (*mocks.UserRepositoryMock, *mocks.DashboardSessionRepoMock, *mocks.DashboardReportRepoMock) {
		userRepo := &mocks.UserRepositoryMock{}
		sessRepo := &mocks.DashboardSessionRepoMock{}
		repRepo := &mocks.DashboardReportRepoMock{}
		userRepo.On("ListUsersPaged", mock.Anything, 0, "", mock.Anything).
			Return([]models.User{{ID: 1, Name: "u1"}, {ID: 2, Name: "u2"}}, int64(2), nil)
		sessRepo.On("GetUserStatsBatch", []uint{1, 2}).Return(map[uint]repositories.UserSessionStat{}, nil)
		sessRepo.On("ListFinishedSessionIDsByUsers", []uint{1, 2}).Return(map[uint][]uint{1: sessIDs1, 2: {1001}}, nil)
		repRepo.On("FindBySessionIDs", sessIDs1).Return(reports1, nil)
		repRepo.On("FindBySessionIDs", []uint{1001}).Return([]models.InterviewReport(nil), errors.New("db error"))
		return userRepo, sessRepo, repRepo
	}

	t.Run("CSV", func(t *testing.T) {
		userRepo, sessRepo, repRepo := setup()
		req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), nil)
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ExportCSV, testsupport.NewCtx(req, rec), http.StatusOK)
		lines := strings.Split(rec.Body.String(), "\n")
		if assert.GreaterOrEqual(t, len(lines), 3) {
			assert.True(t, strings.HasPrefix(lines[1], "1,u1,") && strings.HasSuffix(lines[1], ",4.00"), lines[1])
			assert.True(t, strings.HasPrefix(lines[2], "2,u2,") && strings.HasSuffix(lines[2], ","), lines[2])
		}
		repRepo.AssertNumberOfCalls(t, "FindBySessionIDs", 2)
	})
	t.Run("一覧", func(t *testing.T) {
		userRepo, sessRepo, repRepo := setup()
		req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil), nil)
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ListUsers, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	})
}
