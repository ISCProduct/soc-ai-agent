package hr

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/shared"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUserRepo struct {
	user *entity.User
	err  error
}

func (s *stubUserRepo) GetUserByID(uint) (*entity.User, error) {
	return s.user, s.err
}

type stubCrossFeature struct {
	profile *flywheel.UserIntegratedProfile
	err     error
}

func (s *stubCrossFeature) BuildIntegratedProfile(uint, string, int, bool) (*flywheel.UserIntegratedProfile, error) {
	return s.profile, s.err
}

type stubInterviewSessions struct{}

func (stubInterviewSessions) ListByUser(uint, int, int) ([]models.InterviewSession, error) {
	return nil, nil
}
func (stubInterviewSessions) CountByUser(uint) (int64, error) { return 0, nil }

type stubInterviewReports struct{}

func (stubInterviewReports) FindBySessionIDs([]uint) ([]models.InterviewReport, error) {
	return nil, nil
}

// stubAccess は shared.CompanyAccessReader の偽物。想定した (user, company) で
// 問い合わせられたことも確かめる(元の sqlmock の WithArgs と同じ役割)。
type stubAccess struct {
	t           *testing.T
	wantUser    uint
	wantCompany uint
	owns        bool
	err         error
}

func newStubAccess(t *testing.T, user, company uint, owns bool, err error) *stubAccess {
	return &stubAccess{t: t, wantUser: user, wantCompany: company, owns: owns, err: err}
}

func (a *stubAccess) IsAdmin(uint) (bool, error) { return false, nil }
func (a *stubAccess) OwnsCompany(user, company uint) (bool, error) {
	if a.t != nil && (user != a.wantUser || company != a.wantCompany) {
		a.t.Errorf("OwnsCompany(%d, %d), want (%d, %d)", user, company, a.wantUser, a.wantCompany)
	}
	return a.owns, a.err
}

func TestStudentAnalysisService_GetAnalysis_ForbiddenWithoutOwnership(t *testing.T) {
	access := newStubAccess(t, 2, 99, false, nil)

	svc := NewStudentAnalysisService(
		access, &stubUserRepo{}, &stubCrossFeature{}, nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 99, 10)
	assert.ErrorIs(t, err, shared.ErrForbidden)
}

func TestStudentAnalysisService_GetAnalysis_NotVisibleWithoutConsent(t *testing.T) {
	access := newStubAccess(t, 2, 10, true, nil)

	svc := NewStudentAnalysisService(
		access,
		&stubUserRepo{user: &entity.User{ID: 5, AllowScoutVisibility: false}},
		&stubCrossFeature{},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 10, 5)
	assert.ErrorIs(t, err, ErrStudentNotVisible)
}

type stubIdentity struct {
	revealed bool
	err      error
}

func (s stubIdentity) IdentityRevealed(uint, uint) (bool, error) {
	return s.revealed, s.err
}

func TestStudentAnalysisService_GetAnalysis_承諾前は氏名を返さない(t *testing.T) {
	access := newStubAccess(t, 2, 10, true, nil)

	svc := NewStudentAnalysisService(
		access,
		&stubUserRepo{user: &entity.User{ID: 5, Name: "山田太郎", AllowScoutVisibility: true, Role: "student"}},
		&stubCrossFeature{profile: &flywheel.UserIntegratedProfile{UserID: 5}},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	svc.SetIdentityRevealer(stubIdentity{revealed: false})

	resp, err := svc.GetAnalysis(2, 10, 5)
	require.NoError(t, err)
	assert.Empty(t, resp.Name)
}

func TestStudentAnalysisService_GetAnalysis_承諾後は氏名を返す(t *testing.T) {
	access := newStubAccess(t, 2, 10, true, nil)

	svc := NewStudentAnalysisService(
		access,
		&stubUserRepo{user: &entity.User{ID: 5, Name: "山田太郎", AllowScoutVisibility: true, Role: "student"}},
		&stubCrossFeature{profile: &flywheel.UserIntegratedProfile{UserID: 5}},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	svc.SetIdentityRevealer(stubIdentity{revealed: true})

	resp, err := svc.GetAnalysis(2, 10, 5)
	require.NoError(t, err)
	assert.Equal(t, "山田太郎", resp.Name)
}

func TestStudentAnalysisService_GetAnalysis_Success(t *testing.T) {
	access := newStubAccess(t, 2, 10, true, nil)

	svc := NewStudentAnalysisService(
		access,
		&stubUserRepo{user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "student"}},
		&stubCrossFeature{profile: &flywheel.UserIntegratedProfile{UserID: 5}},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	resp, err := svc.GetAnalysis(2, 10, 5)
	require.NoError(t, err)
	assert.Equal(t, uint(5), resp.UserID)
	assert.NotNil(t, resp.IntegratedProfile)
	assert.Empty(t, resp.InterviewReports)
}

func TestStudentAnalysisService_GetAnalysis_NotFoundUser(t *testing.T) {
	access := newStubAccess(t, 2, 10, true, nil)

	svc := NewStudentAnalysisService(
		access,
		&stubUserRepo{user: nil},
		&stubCrossFeature{},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 10, 999)
	assert.ErrorIs(t, err, ErrStudentNotVisible)
}

func TestStudentAnalysisService_GetAnalysis_DBError(t *testing.T) {
	access := newStubAccess(t, 2, 10, false, errors.New("db down"))

	svc := NewStudentAnalysisService(
		access, &stubUserRepo{}, &stubCrossFeature{}, nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 10, 5)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, shared.ErrForbidden))
}

// TestIsScoutVisible は企業へ公開してよい学生の条件を固定する。
// 一覧検索(SQL)と詳細取得(Go)で条件が食い違うと認可が破れるため、
// 退会済み・ゲスト・教員が公開同意していても公開されないことを明示的に検証する。
func TestIsScoutVisible(t *testing.T) {
	withdrawn := time.Now()
	tests := []struct {
		name string
		user *entity.User
		want bool
	}{
		{
			name: "同意済みの学生は公開",
			user: &entity.User{AllowScoutVisibility: true, Role: "student"},
			want: true,
		},
		{
			name: "未同意は非公開",
			user: &entity.User{AllowScoutVisibility: false, Role: "student"},
			want: false,
		},
		{
			name: "退会済みは同意済みでも非公開",
			user: &entity.User{AllowScoutVisibility: true, Role: "student", WithdrawnAt: &withdrawn},
			want: false,
		},
		{
			name: "ゲストは同意済みでも非公開",
			user: &entity.User{AllowScoutVisibility: true, Role: "student", IsGuest: true},
			want: false,
		},
		{
			name: "教員は同意済みでも非公開",
			user: &entity.User{AllowScoutVisibility: true, Role: "teacher"},
			want: false,
		},
		{
			name: "ロール未設定は非公開",
			user: &entity.User{AllowScoutVisibility: true, Role: ""},
			want: false,
		},
		{
			name: "nilは非公開",
			user: nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsScoutVisible(tt.user))
		})
	}
}

// TestStudentAnalysisService_GetAnalysisForVisibleStudent_AppliesFullGuard は、
// 企業ポータルの詳細取得が一覧検索と同じ公開条件を通ることを検証する。
func TestStudentAnalysisService_GetAnalysisForVisibleStudent_AppliesFullGuard(t *testing.T) {
	withdrawn := time.Now()
	tests := []struct {
		name    string
		user    *entity.User
		wantErr bool
	}{
		{name: "同意済みの学生は取得できる", user: &entity.User{ID: 5, Name: "山田太郎", AllowScoutVisibility: true, Role: "student"}},
		{name: "退会済みはID直指定でも取得できない", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "student", WithdrawnAt: &withdrawn}, wantErr: true},
		{name: "ゲストはID直指定でも取得できない", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "student", IsGuest: true}, wantErr: true},
		{name: "教員はID直指定でも取得できない", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "teacher"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewStudentAnalysisService(
				&stubAccess{},
				&stubUserRepo{user: tt.user},
				&stubCrossFeature{profile: &flywheel.UserIntegratedProfile{UserID: 5}},
				nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
			)
			svc.SetIdentityRevealer(stubIdentity{revealed: true})
			resp, err := svc.GetAnalysisForVisibleStudent(10, 5)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrStudentNotVisible)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, uint(5), resp.UserID)
			assert.Equal(t, "山田太郎", resp.Name, "承諾済みなら企業の学生詳細で氏名を出す")
		})
	}
}
