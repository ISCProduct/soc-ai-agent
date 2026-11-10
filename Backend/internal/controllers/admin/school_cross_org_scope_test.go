package admin_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// 1件ごとの認可判定は、対象ユーザーの school_id をそのまま信用してはいけない（#1675）。
// school_id はプロフィールの学校名（自己申告）から解決されるため、学校が本人の学園に
// 属していなければ未所属として扱う。担当校ありの管理者・職員には見せない。
func TestEnsureAdminSchoolAccessForUser_RejectsSchoolOutsideTargetOrganization(t *testing.T) {
	schoolID := uint(5)
	tests := []struct {
		name         string
		targetOrgID  uint
		schoolOrgID  uint
		wantHTTPCode int
	}{
		// 担当校(5)に属していても、学校の学園と本人の学園が違えば通してはいけない。
		{"学園が一致しない", 2, 1, http.StatusForbidden},
		{"学園が一致する", 1, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.SchoolRepositoryMock{}
			// 担当校ありの職員（restricted）
			repo.On("ListSchoolsForAdmin", uint(42)).
				Return([]models.School{{ID: schoolID, Name: "担当校"}}, nil)
			repo.On("FindByID", schoolID).
				Return(&models.School{ID: schoolID, OrganizationID: tt.schoolOrgID}, nil)

			req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/", nil), 42)
			ctx := testsupport.NewCtx(req, httptest.NewRecorder())

			err := httpapi.EnsureAdminSchoolAccessForUser(ctx, school.NewSchoolService(repo, nil), tt.targetOrgID, &schoolID)
			if tt.wantHTTPCode == 0 {
				if err != nil {
					t.Fatalf("同一学園なら通すべき: %v", err)
				}
				return
			}
			he, ok := err.(*echo.HTTPError)
			if !ok || he.Code != tt.wantHTTPCode {
				t.Fatalf("学園不一致を拒否していない: err=%v", err)
			}
		})
	}
}

// 無制限の管理者は学校で絞らないため、学園一致の判定に入らず学校も引き直さない。
// 引き直すと余分な問い合わせが増えるうえ、FindByID のスタブが無い既存テストが壊れる。
func TestEnsureAdminSchoolAccessForUser_UnrestrictedAdminSkipsLookup(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{}, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/", nil), 42)
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())

	schoolID := uint(5)
	if err := httpapi.EnsureAdminSchoolAccessForUser(ctx, school.NewSchoolService(repo, nil), 2, &schoolID); err != nil {
		t.Fatalf("無制限管理者は通すべき: %v", err)
	}
	repo.AssertNotCalled(t, "FindByID", schoolID)
}

// 担当校は学園をまたげない（#1675）。またげると、他学園の職員に担当校を割り当てるだけで
// その学校の学生データへ到達させられる。
func TestSchoolService_AddMemberRejectsCrossOrganization(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)

	svc := school.NewSchoolService(repo, stubUserReader{user: &entity.User{ID: 2, OrganizationID: 9}})
	if err := svc.AddMember(2, 5); !errors.Is(err, school.ErrMemberOrganizationMismatch) {
		t.Fatalf("学園をまたいだ割当を許している: %v", err)
	}
	repo.AssertNotCalled(t, "AddMember")
}

// ユーザー参照を渡さない構成では割当を通さない（fail-close）。
// 検査を飛ばして割当が通る方が危険なので、未注入はエラーにする。
func TestSchoolService_AddMemberFailsClosedWithoutUserReader(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)

	// 「別の理由のエラー」で緑にならないよう、センチネルで固定する。
	if err := school.NewSchoolService(repo, nil).AddMember(2, 5); !errors.Is(err, school.ErrUserReaderNotConfigured) {
		t.Fatalf("未注入の fail-close になっていない: %v", err)
	}
	repo.AssertNotCalled(t, "AddMember")
}

// 学園をまたいだ割当は 400 で、原因の分かるメッセージを返す。
// 内部エラー(500)に落とすと、設定ミスと見分けがつかない（#1675）。
func TestAdminSchoolController_AddMember_CrossOrganizationIsBadRequest(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{}, nil) // 無制限管理者
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)

	body, _ := json.Marshal(map[string]any{"user_id": 2})
	req := testsupport.WithAdminUserID(
		httptest.NewRequest(http.MethodPost, "/api/admin/schools/5/members", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())
	ctx.SetParamNames("id")
	ctx.SetParamValues("5")

	ctrl := admincontrollers.NewAdminSchoolController(
		school.NewSchoolService(repo, stubUserReader{user: &entity.User{ID: 2, OrganizationID: 9}}))
	testsupport.AssertStatus(t, ctrl.AddMember, ctx, http.StatusBadRequest)
	repo.AssertNotCalled(t, "AddMember")
}

// 対象ユーザーを読んでから認可する経路は、学園が一致しない学校を担当校として通してはいけない（#1675）。
//
// ヘルパー単体のテストだけでは、呼び出し元が素の EnsureAdminSchoolAccess に戻る回帰を検出できない
// （シグネチャが合うのでコンパイルも通る）。経路ごとに「学園不一致なら403」を固定する。
// 通過側の確認はヘルパー単体のテストが両方向を押さえているので、ここでは重ねない
// （後段の処理まで動かすとモックが際限なく増え、認可の検証がぼやける）。
//
// 条件は実際に報告された再現そのもの: 対象は担当校(5)に在籍しているように見えるが、
// 学校の学園(1)と対象ユーザーの学園(9)が違う。包含判定だけでは弾けない。
func TestAdminControllers_RejectCrossOrganizationTarget(t *testing.T) {
	const (
		adminID    = uint(42)
		targetID   = uint(3)
		schoolID   = uint(5)
		schoolOrg  = uint(1)
		foreignOrg = uint(9)
	)

	newSchools := func(repo *mocks.SchoolRepositoryMock) *school.SchoolService {
		// 担当校ありの職員。担当校には対象の school_id が含まれている。
		repo.On("ListSchoolsForAdmin", adminID).
			Return([]models.School{{ID: schoolID, OrganizationID: schoolOrg}}, nil)
		repo.On("FindByID", schoolID).
			Return(&models.School{ID: schoolID, OrganizationID: schoolOrg}, nil)
		return school.NewSchoolService(repo, nil)
	}
	foreignTarget := func() *entity.User {
		sid := schoolID
		return &entity.User{ID: targetID, Email: "t@example.com", Role: entity.UserRoleStudent,
			OrganizationID: foreignOrg, SchoolID: &sid}
	}
	newReq := func(method string, body []byte) echo.Context {
		var r *http.Request
		if body == nil {
			r = httptest.NewRequest(method, "/", nil)
		} else {
			r = httptest.NewRequest(method, "/", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		}
		ctx := testsupport.NewCtx(testsupport.WithAdminUserID(r, adminID), httptest.NewRecorder())
		ctx.SetParamNames("id")
		ctx.SetParamValues("3")
		return ctx
	}
	assertForbidden := func(t *testing.T, err error) {
		t.Helper()
		he, ok := err.(*echo.HTTPError)
		if !ok || he.Code != http.StatusForbidden {
			t.Fatalf("学園不一致を拒否していない: err=%v", err)
		}
	}

	t.Run("学生詳細", func(t *testing.T) {
		userRepo := &mocks.UserRepositoryMock{}
		userRepo.On("GetUserByID", targetID).Return(foreignTarget(), nil)
		ctrl := admincontrollers.NewAdminDashboardController(userRepo, nil, nil)
		ctrl.SetSchoolService(newSchools(&mocks.SchoolRepositoryMock{}))
		assertForbidden(t, ctrl.UserSessions(newReq(http.MethodGet, nil)))
	})

	t.Run("ユーザー編集", func(t *testing.T) {
		userRepo := &mocks.UserRepositoryMock{}
		userRepo.On("GetUserByID", targetID).Return(foreignTarget(), nil)
		audit := &mocks.AuditLogServiceMock{}
		ctrl := admincontrollers.NewAdminUserController(userRepo, audit)
		ctrl.SetSchoolService(newSchools(&mocks.SchoolRepositoryMock{}))
		body, _ := json.Marshal(map[string]any{"name": "x"})
		assertForbidden(t, ctrl.Update(newReq(http.MethodPut, body)))
	})

	// 削除（Delete）は Update と同じ ensureSchoolAccess を通るので重ねない。
	// 単体で呼ぶには削除サービスの注入が要り、認可の検証がぼやける。

	t.Run("面接動画一覧", func(t *testing.T) {
		userRepo := &mocks.UserRepositoryMock{}
		userRepo.On("GetUserByID", targetID).Return(foreignTarget(), nil)
		sessionRepo := &mocks.InterviewSessionRepositoryMock{}
		sessionRepo.On("FindByID", uint(3)).Return(&models.InterviewSession{ID: 3, UserID: targetID}, nil)

		ctrl := admincontrollers.NewAdminInterviewController(nil, nil, nil)
		ctrl.SetSchoolAccess(userRepo, sessionRepo, newSchools(&mocks.SchoolRepositoryMock{}))
		assertForbidden(t, ctrl.ListVideos(newReq(http.MethodGet, nil)))
	})
}
