package hr

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/shared"
	"errors"
	"testing"

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
		&stubUserRepo{user: &entity.User{ID: 5, AllowScoutVisibility: true}},
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
