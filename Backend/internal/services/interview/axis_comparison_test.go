package interview_test

import (
	"Backend/internal/models"
	"Backend/internal/services/interview"
	"Backend/internal/services/shared"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAxisDeltas_ImprovedDeclinedAndMissing(t *testing.T) {
	base := map[string]float64{
		"logic": 2, "specificity": 4, "ownership": 3, "communication": 1,
	}
	target := map[string]float64{
		"logic": 4, "specificity": 4, "ownership": 1, "enthusiasm": 5,
	}

	got := interview.AxisDeltas(base, target)
	require.Len(t, got, 5)

	assert.Equal(t, "logic", got[0].Axis)
	assert.Equal(t, "伸びた", got[0].ChangeLabel)
	assert.Equal(t, "improved", got[0].Direction)
	require.NotNil(t, got[0].Delta)
	assert.InDelta(t, 2, *got[0].Delta, 0.001)

	assert.Equal(t, "変化なし", got[1].ChangeLabel)
	assert.Equal(t, "unchanged", got[1].Direction)
	require.NotNil(t, got[1].Delta)
	assert.InDelta(t, 0, *got[1].Delta, 0.001)

	assert.Equal(t, "落ちた", got[2].ChangeLabel)
	assert.Equal(t, "declined", got[2].Direction)
	require.NotNil(t, got[2].Delta)
	assert.InDelta(t, -2, *got[2].Delta, 0.001)

	assert.Equal(t, "比較できない", got[3].ChangeLabel)
	assert.Nil(t, got[3].Delta)
	assert.NotNil(t, got[3].Base)
	assert.Nil(t, got[3].Target)

	assert.Equal(t, "比較できない", got[4].ChangeLabel)
	assert.Nil(t, got[4].Base)
	assert.NotNil(t, got[4].Target)
}

func TestCompareAxes_OwnSessions(t *testing.T) {
	sRepo := new(mockSessionRepo)
	rRepo := new(mockReportRepo)
	sRepo.On("FindByID", uint(10)).Return(&models.InterviewSession{ID: 10, UserID: 1}, nil)
	sRepo.On("FindByID", uint(11)).Return(&models.InterviewSession{ID: 11, UserID: 1}, nil)
	rRepo.On("FindBySessionID", uint(10)).Return(&models.InterviewReport{SessionID: 10, ScoresJSON: `{"logic":1,"specificity":2,"ownership":3,"communication":4,"enthusiasm":5}`}, nil)
	rRepo.On("FindBySessionID", uint(11)).Return(&models.InterviewReport{SessionID: 11, ScoresJSON: `{"logic":3,"specificity":2,"ownership":1,"communication":4,"enthusiasm":5}`}, nil)

	svc := interview.NewInterviewService(sRepo, nil, rRepo, nil, nil, nil, nil)
	got, err := svc.CompareAxes(1, 10, 11)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint(10), got.BaseSessionID)
	assert.Equal(t, uint(11), got.TargetSessionID)
	assert.Equal(t, "伸びた", got.Axes[0].ChangeLabel)
	assert.Equal(t, "落ちた", got.Axes[2].ChangeLabel)
}

func TestCompareAxes_OtherUsersSessionIsForbidden(t *testing.T) {
	sRepo := new(mockSessionRepo)
	uRepo := new(mockUserRepo)
	sRepo.On("FindByID", uint(10)).Return(&models.InterviewSession{ID: 10, UserID: 2}, nil)
	uRepo.On("GetUserByID", uint(1)).Return(&models.User{ID: 1, IsAdmin: false}, nil)

	svc := interview.NewInterviewService(sRepo, nil, new(mockReportRepo), uRepo, nil, nil, nil)
	_, err := svc.CompareAxes(1, 10, 11)
	assert.ErrorIs(t, err, shared.ErrForbidden)
}

func TestCompareAxes_SameSessionAndMissingReport(t *testing.T) {
	svc := interview.NewInterviewService(new(mockSessionRepo), nil, new(mockReportRepo), nil, nil, nil, nil)
	_, err := svc.CompareAxes(1, 10, 10)
	assert.ErrorIs(t, err, interview.ErrInvalidComparison)

	sRepo := new(mockSessionRepo)
	rRepo := new(mockReportRepo)
	sRepo.On("FindByID", uint(10)).Return(&models.InterviewSession{ID: 10, UserID: 1}, nil)
	sRepo.On("FindByID", uint(11)).Return(&models.InterviewSession{ID: 11, UserID: 1}, nil)
	rRepo.On("FindBySessionID", uint(10)).Return(nil, gorm.ErrRecordNotFound)

	svc = interview.NewInterviewService(sRepo, nil, rRepo, nil, nil, nil, nil)
	_, err = svc.CompareAxes(1, 10, 11)
	assert.ErrorIs(t, err, interview.ErrReportNotReady)
}
