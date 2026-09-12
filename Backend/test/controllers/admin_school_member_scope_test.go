package controllers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
)

// newSchoolMemberCtx は :id / :user_id を持つ echo.Context を作る。
func newSchoolMemberCtx(req *http.Request, schoolID, userID string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	if userID == "" {
		ctx.SetParamNames("id")
		ctx.SetParamValues(schoolID)
	} else {
		ctx.SetParamNames("id", "user_id")
		ctx.SetParamValues(schoolID, userID)
	}
	return ctx, rec
}

// TestAdminSchoolController_AddMember_OtherSchoolDenied は、担当校を持つ管理者(先生)が
// 他校へメンバーを追加できないことを検証する（#1157）。
//
// ここが無検証だと、自分を他校の担当に追加して他校の生徒データを
// 「正規の担当」として閲覧できてしまう（学校スコープの修正が2リクエストで無効化される）。
func TestAdminSchoolController_AddMember_OtherSchoolDenied(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).
		Return([]models.School{{ID: 5, Name: "担当校"}}, nil)

	body, _ := json.Marshal(map[string]any{"user_id": 1})
	req := withAdminUserID(httptest.NewRequest(http.MethodPost, "/api/admin/schools/9/members", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := newSchoolMemberCtx(req, "9", "")

	ctrl := controllers.NewAdminSchoolController(services.NewSchoolService(repo))
	assertStatus(t, ctrl.AddMember, ctx, http.StatusForbidden)
	repo.AssertNotCalled(t, "AddMember")
}

// TestAdminSchoolController_AddMember_OwnSchoolAllowed は担当校への追加は通ることを検証する。
func TestAdminSchoolController_AddMember_OwnSchoolAllowed(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).
		Return([]models.School{{ID: 5, Name: "担当校"}}, nil)
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, Name: "担当校"}, nil)
	repo.On("AddMember", mock.MatchedBy(func(m *models.AdminSchoolMembership) bool {
		return m.UserID == 2 && m.SchoolID == 5
	})).Return(nil)

	body, _ := json.Marshal(map[string]any{"user_id": 2})
	req := withAdminUserID(httptest.NewRequest(http.MethodPost, "/api/admin/schools/5/members", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := newSchoolMemberCtx(req, "5", "")

	ctrl := controllers.NewAdminSchoolController(services.NewSchoolService(repo))
	assertStatus(t, ctrl.AddMember, ctx, http.StatusCreated)
	repo.AssertExpectations(t)
}

// TestAdminSchoolController_RemoveMember_LastSchoolDenied は、対象の担当校が0件になる削除を
// 拒否することを検証する（#1157）。
//
// ResolveAdminAccess は担当校0件を「無制限のプラットフォーム管理者」として扱うため、
// 最後の担当校を外すと権限剥奪のつもりが全校アクセスへの昇格になる。
func TestAdminSchoolController_RemoveMember_LastSchoolDenied(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	// 呼び出し元(無制限管理者)と対象(A校のみ担当)の2回解決される
	repo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{}, nil)
	repo.On("ListSchoolsForAdmin", uint(7)).Return([]models.School{{ID: 5, Name: "担当校"}}, nil)

	req := withAdminUserID(httptest.NewRequest(http.MethodDelete, "/api/admin/schools/5/members/7", nil), 1)
	ctx, _ := newSchoolMemberCtx(req, "5", "7")

	ctrl := controllers.NewAdminSchoolController(services.NewSchoolService(repo))
	assertStatus(t, ctrl.RemoveMember, ctx, http.StatusForbidden)
	repo.AssertNotCalled(t, "RemoveMember")
}

// TestAdminSchoolController_RemoveMember_KeepsOtherSchools は、担当校が複数ある管理者からの
// 1校の解除は通ることを検証する（0件にならないため昇格しない）。
func TestAdminSchoolController_RemoveMember_KeepsOtherSchools(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{}, nil)
	repo.On("ListSchoolsForAdmin", uint(7)).
		Return([]models.School{{ID: 5, Name: "A校"}, {ID: 6, Name: "B校"}}, nil)
	repo.On("RemoveMember", uint(7), uint(5)).Return(nil)

	req := withAdminUserID(httptest.NewRequest(http.MethodDelete, "/api/admin/schools/5/members/7", nil), 1)
	ctx, _ := newSchoolMemberCtx(req, "5", "7")

	ctrl := controllers.NewAdminSchoolController(services.NewSchoolService(repo))
	assertStatus(t, ctrl.RemoveMember, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// TestAdminUserController_Update_RestrictedCannotGrantAdmin は、担当校を持つ管理者(先生)が
// is_admin を付与できないことを検証する（#1157）。
//
// 自校の生徒に is_admin を付けると、その生徒は担当校0件 = 無制限管理者として扱われ、
// そのアカウントでログインすれば全校のデータへ到達できてしまう。
func TestAdminUserController_Update_RestrictedCannotGrantAdmin(t *testing.T) {
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(1)).
		Return([]models.School{{ID: 5, Name: "担当校"}}, nil)

	userRepo := &mocks.UserRepositoryMock{}
	school := uint(5)
	userRepo.On("GetUserByID", uint(20)).
		Return(&entity.User{ID: 20, SchoolID: &school}, nil)

	body, _ := json.Marshal(map[string]any{"is_admin": true})
	req := withAdminUserID(httptest.NewRequest(http.MethodPut, "/api/admin/users/20", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("20")

	ctrl := controllers.NewAdminUserController(userRepo, nil)
	ctrl.SetSchoolService(services.NewSchoolService(schoolRepo))
	assertStatus(t, ctrl.Update, ctx, http.StatusForbidden)
	userRepo.AssertNotCalled(t, "UpdateUser")
}
