package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
)

// newAdminUserController は school サービス付きの AdminUserController を組む。
func newAdminUserController(userRepo *mocks.UserRepositoryMock, schoolRepo *mocks.SchoolRepositoryMock) *admincontrollers.AdminUserController {
	audit := &mocks.AuditLogServiceMock{}
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	ctrl := admincontrollers.NewAdminUserController(userRepo, audit)
	ctrl.SetSchoolService(school.NewSchoolService(schoolRepo))
	return ctrl
}

func newUpdateCtx(t *testing.T, id string, body map[string]any, adminUserID uint) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodPut, "/api/admin/users/"+id, bytes.NewReader(raw)), adminUserID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(id)
	return ctx, rec
}

// TestUpdate_GrantStaff_Unrestricted は無制限管理者が職員ロールを付与でき、
// users.role が staff に更新されることを検証する(#1681 フォローUI)。
func TestUpdate_GrantStaff_Unrestricted(t *testing.T) {
	schoolID := uint(7)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(2)).Return(&entity.User{ID: 2, Email: "s@example.com", SchoolID: &schoolID, Role: entity.UserRoleStudent}, nil)
	var saved *entity.User
	userRepo.On("UpdateUser", mock.MatchedBy(func(u *entity.User) bool {
		saved = u
		return u.ID == 2 && u.Role == entity.UserRoleStaff
	})).Return(nil)

	schoolRepo := &mocks.SchoolRepositoryMock{}
	// 担当校0件 = 無制限管理者(platform)。
	schoolRepo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{}, nil)

	ctrl := newAdminUserController(userRepo, schoolRepo)
	ctx, rec := newUpdateCtx(t, "2", map[string]any{"is_staff": true}, 1)
	if err := ctrl.Update(ctx); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if saved == nil || saved.Role != entity.UserRoleStaff {
		t.Fatalf("role not updated to staff: %+v", saved)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["is_staff"] != true {
		t.Fatalf("response is_staff = %v, want true", resp["is_staff"])
	}
}

// TestUpdate_GrantStaff_RestrictedDenied は担当校を持つ制限adminが
// 職員ロールを付与できない(403)ことを検証する。自校の生徒を職員へ昇格させて
// 越権させる経路を塞ぐ。
func TestUpdate_GrantStaff_RestrictedDenied(t *testing.T) {
	schoolID := uint(5)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(2)).Return(&entity.User{ID: 2, SchoolID: &schoolID, Role: entity.UserRoleStudent}, nil)

	schoolRepo := &mocks.SchoolRepositoryMock{}
	// 担当校あり = 制限admin。対象の学校(5)は担当内なので ensureSchoolAccess は通り、
	// is_staff 付与の denyIfRestricted で 403 になることを確認する。
	schoolRepo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{{ID: 5, Name: "担当校"}}, nil)
	// 対象ユーザーの学園と一致する学校を返す（#1675 の学園一致検査を通す）。
	// 403 の理由は職員ロール付与の制限であり、学園不一致ではない。
	schoolRepo.On("FindByID", schoolID).Return(&models.School{ID: schoolID, Name: "担当校"}, nil)

	ctrl := newAdminUserController(userRepo, schoolRepo)
	ctx, _ := newUpdateCtx(t, "2", map[string]any{"is_staff": true}, 1)
	testsupport.AssertStatus(t, ctrl.Update, ctx, http.StatusForbidden)
	userRepo.AssertNotCalled(t, "UpdateUser", mock.Anything)
}
