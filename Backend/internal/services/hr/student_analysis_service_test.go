package hr

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/shared"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
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

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	return db, mock
}

func TestStudentAnalysisService_GetAnalysis_ForbiddenWithoutOwnership(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships`").
		WithArgs(uint(2), uint(99)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	svc := NewStudentAnalysisService(
		db, &stubUserRepo{}, &stubCrossFeature{}, nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 99, 10)
	assert.ErrorIs(t, err, shared.ErrForbidden)
}

func TestStudentAnalysisService_GetAnalysis_NotVisibleWithoutConsent(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships`").
		WithArgs(uint(2), uint(10)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	svc := NewStudentAnalysisService(
		db,
		&stubUserRepo{user: &entity.User{ID: 5, AllowScoutVisibility: false}},
		&stubCrossFeature{},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 10, 5)
	assert.ErrorIs(t, err, ErrStudentNotVisible)
}

func TestStudentAnalysisService_GetAnalysis_Success(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships`").
		WithArgs(uint(2), uint(10)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	svc := NewStudentAnalysisService(
		db,
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
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships`").
		WithArgs(uint(2), uint(10)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	svc := NewStudentAnalysisService(
		db,
		&stubUserRepo{user: nil},
		&stubCrossFeature{},
		nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
	)
	_, err := svc.GetAnalysis(2, 10, 999)
	assert.ErrorIs(t, err, ErrStudentNotVisible)
}

func TestStudentAnalysisService_GetAnalysis_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships`").
		WithArgs(uint(2), uint(10)).
		WillReturnError(errors.New("db down"))

	svc := NewStudentAnalysisService(
		db, &stubUserRepo{}, &stubCrossFeature{}, nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
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
		{name: "同意済みの学生は取得できる", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "student"}},
		{name: "退会済みはID直指定でも取得できない", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "student", WithdrawnAt: &withdrawn}, wantErr: true},
		{name: "ゲストはID直指定でも取得できない", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "student", IsGuest: true}, wantErr: true},
		{name: "教員はID直指定でも取得できない", user: &entity.User{ID: 5, AllowScoutVisibility: true, Role: "teacher"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _ := newMockDB(t)
			svc := NewStudentAnalysisService(
				db,
				&stubUserRepo{user: tt.user},
				&stubCrossFeature{profile: &flywheel.UserIntegratedProfile{UserID: 5}},
				nil, nil, stubInterviewSessions{}, stubInterviewReports{}, nil,
			)
			resp, err := svc.GetAnalysisForVisibleStudent(5)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrStudentNotVisible)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, uint(5), resp.UserID)
		})
	}
}
