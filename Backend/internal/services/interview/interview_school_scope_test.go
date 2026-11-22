package interview_test

import (
	"Backend/internal/controllers/mocks"
	"Backend/internal/models"
	"Backend/internal/services/interview"
	"Backend/internal/services/school"
	"Backend/internal/services/shared"
	"errors"
	"testing"
)

// TestInterviewService_SchoolScopeOnStudentRoutes は学生用の面接ルートの所有者判定（isAllowed）が、
// 本人以外の管理者に対して admin ルートと同じ担当校スコープを課すことを固定する（SOCAIAGENT-687）。
// isAllowed を通る全ルート（詳細・レポート・開始・終了・ターン・動画など）はこの判定を共有するので、
// 代表として EnsureSessionOwnership と GetSessionDetail の両方で確かめる。
func TestInterviewService_SchoolScopeOnStudentRoutes(t *testing.T) {
	t.Parallel()

	const (
		ownerID      = uint(1)
		otherStudent = uint(2)
		teacherOwn   = uint(3) // 担当校 = 学生の学校
		teacherOther = uint(4) // 担当校 = 別の学校
		platform     = uint(5) // 担当校なしの is_admin
		staff        = uint(6) // is_admin でない職員
		teacherXOrg  = uint(7) // 担当校は一致するが、その学校が学生の学園のものではない
		sessionID    = uint(100)
		ownSchool    = uint(10)
		otherSchool  = uint(20)
		xOrgSchool   = uint(30)
	)
	sid := func(v uint) *uint { return &v }

	users := map[uint]*models.User{
		ownerID:      {ID: ownerID, Role: models.UserRoleStudent, OrganizationID: 1, SchoolID: sid(ownSchool)},
		otherStudent: {ID: otherStudent, Role: models.UserRoleStudent, OrganizationID: 1, SchoolID: sid(ownSchool)},
		teacherOwn:   {ID: teacherOwn, IsAdmin: true},
		teacherOther: {ID: teacherOther, IsAdmin: true},
		platform:     {ID: platform, IsAdmin: true},
		staff:        {ID: staff, Role: models.UserRoleStaff},
		teacherXOrg:  {ID: teacherXOrg, IsAdmin: true},
	}
	// 学生（学園1）の学校 xOrgSchool を指す自己申告を、学園2の学校として登録しておく
	xOrgOwner := &models.User{ID: 8, Role: models.UserRoleStudent, OrganizationID: 1, SchoolID: sid(xOrgSchool)}
	users[8] = xOrgOwner

	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", teacherOwn).Return([]models.School{{ID: ownSchool}}, nil)
	repo.On("ListSchoolsForAdmin", teacherOther).Return([]models.School{{ID: otherSchool}}, nil)
	repo.On("ListSchoolsForAdmin", teacherXOrg).Return([]models.School{{ID: xOrgSchool}}, nil)
	repo.On("ListSchoolsForAdmin", platform).Return([]models.School{}, nil)
	repo.On("FindByID", ownSchool).Return(&models.School{ID: ownSchool, OrganizationID: 1}, nil)
	repo.On("FindByID", xOrgSchool).Return(&models.School{ID: xOrgSchool, OrganizationID: 2}, nil)
	schools := school.NewSchoolService(repo, nil)

	tests := []struct {
		name    string
		owner   uint
		actor   uint
		noScope bool // 学校スコープ判定を注入しない（DI漏れ）
		wantErr error
	}{
		{name: "本人は許可", owner: ownerID, actor: ownerID},
		{name: "他人の学生は拒否", owner: ownerID, actor: otherStudent, wantErr: shared.ErrForbidden},
		{name: "担当校付き教員: 自校の学生は許可", owner: ownerID, actor: teacherOwn},
		{name: "担当校付き教員: 他校の学生は拒否", owner: ownerID, actor: teacherOther, wantErr: shared.ErrForbidden},
		{name: "担当校付き教員: 担当校が学生の学園のものでなければ拒否", owner: 8, actor: teacherXOrg, wantErr: shared.ErrForbidden},
		{name: "担当校なしの管理者は許可", owner: ownerID, actor: platform},
		{name: "is_admin でない職員は拒否", owner: ownerID, actor: staff, wantErr: shared.ErrForbidden},
		{name: "学校スコープ未注入なら管理者でも拒否(fail-close)", owner: ownerID, actor: platform, noScope: true, wantErr: shared.ErrForbidden},
		{name: "本人なら学校スコープ未注入でも許可", owner: ownerID, actor: ownerID, noScope: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sRepo := new(mockSessionRepo)
			sRepo.On("FindByID", sessionID).Return(&models.InterviewSession{ID: sessionID, UserID: tt.owner, Status: "finished"}, nil)
			uRepo := new(mockUserRepo)
			for id, u := range users {
				uRepo.On("GetUserByID", id).Return(u, nil).Maybe()
			}
			svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, nil, nil)
			if !tt.noScope {
				svc.SetSchoolAccess(schools)
			}

			if err := svc.EnsureSessionOwnership(tt.actor, sessionID); !errors.Is(err, tt.wantErr) {
				t.Fatalf("EnsureSessionOwnership err=%v want %v", err, tt.wantErr)
			}
		})
	}
}

// TestInterviewService_ListSessionsOwnOnly は学生用の一覧が管理者でも本人分だけを返すことを固定する。
// 全件一覧（all=1）は廃止した。管理者の一覧は学校スコープ付きの /api/admin/interviews を使う（SOCAIAGENT-687）。
func TestInterviewService_ListSessionsOwnOnly(t *testing.T) {
	t.Parallel()

	for _, actor := range []*models.User{
		{ID: 1, Role: models.UserRoleStudent},
		{ID: 2, IsAdmin: true},
	} {
		t.Run(actor.Role+map[bool]string{true: "admin", false: ""}[actor.IsAdmin], func(t *testing.T) {
			t.Parallel()
			sRepo := new(mockSessionRepo)
			sRepo.On("CountByUser", actor.ID).Return(int64(1), nil)
			sRepo.On("ListByUser", actor.ID, 20, 0).Return([]models.InterviewSession{{ID: 9, UserID: actor.ID}}, nil)
			uRepo := new(mockUserRepo)
			uRepo.On("GetUserByID", actor.ID).Return(actor, nil).Maybe()
			svc := interview.NewInterviewService(sRepo, nil, nil, uRepo, nil, nil, nil)

			got, total, err := svc.ListSessions(actor.ID, 20, 0)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if total != 1 || len(got) != 1 || got[0].UserID != actor.ID {
				t.Fatalf("本人分だけ返すべき: total=%d got=%+v", total, got)
			}
			sRepo.AssertNotCalled(t, "ListAll")
			sRepo.AssertNotCalled(t, "CountAll")
		})
	}
}
