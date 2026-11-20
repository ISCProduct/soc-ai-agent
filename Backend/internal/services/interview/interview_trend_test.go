package interview_test

// GetTrend のレポート取得がバッチ1回で済むこと、および出力が従来と同じことを固定する。
//
// 実行: cd Backend && go test ./internal/services/interview/ -run GetTrend -v

import (
	"Backend/internal/models"
	"Backend/internal/services/interview"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trendSessionRepo struct {
	mockInterviewSessionRepo
	finished []models.InterviewSession
}

func (r *trendSessionRepo) ListFinishedByUser(uint, int) ([]models.InterviewSession, error) {
	// 実装と同じく新しい順で返す
	return append([]models.InterviewSession(nil), r.finished...), nil
}

type trendReportRepo struct {
	mockInterviewReportRepo
	reports    map[uint]models.InterviewReport
	singleCall int
	batchCall  int
	batchErr   error
}

func (r *trendReportRepo) FindBySessionID(id uint) (*models.InterviewReport, error) {
	r.singleCall++
	rep, ok := r.reports[id]
	if !ok {
		return nil, assert.AnError
	}
	return &rep, nil
}

func (r *trendReportRepo) FindBySessionIDs(ids []uint) ([]models.InterviewReport, error) {
	r.batchCall++
	if r.batchErr != nil {
		return nil, r.batchErr
	}
	var out []models.InterviewReport
	for _, id := range ids {
		if rep, ok := r.reports[id]; ok {
			out = append(out, rep)
		}
	}
	return out, nil
}

func TestGetTrend_BatchesReportLookup(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 新しい順: 3(レポート未生成), 2(不正JSON), 1(正常)
	sessions := []models.InterviewSession{
		{ID: 3, CreatedAt: base.Add(2 * time.Hour)},
		{ID: 2, CreatedAt: base.Add(time.Hour)},
		{ID: 1, CreatedAt: base},
	}
	reports := &trendReportRepo{reports: map[uint]models.InterviewReport{
		1: {SessionID: 1, ScoresJSON: `{"logic":4,"enthusiasm":3}`},
		2: {SessionID: 2, ScoresJSON: `not json`},
	}}
	svc := interview.NewInterviewService(&trendSessionRepo{finished: sessions}, &mockInterviewUtterRepo{}, reports, &mockUserRepo2{}, nil, nil, nil)

	points, err := svc.GetTrend(1, 20)
	require.NoError(t, err)

	// レポート未生成・不正JSONはスキップ
	require.Len(t, points, 1)
	assert.Equal(t, uint(1), points[0].SessionID)
	assert.Equal(t, base, points[0].CreatedAt)
	require.NotNil(t, points[0].Logic)
	assert.Equal(t, 4.0, *points[0].Logic)
	require.NotNil(t, points[0].Enthusiasm)
	assert.Equal(t, 3.0, *points[0].Enthusiasm)
	assert.Nil(t, points[0].Ownership)

	assert.Equal(t, 1, reports.batchCall)
	assert.Equal(t, 0, reports.singleCall)
}

// レポートの一括取得に失敗しても、従来どおりエラーにせず空の推移を返す（ログには残る）。
func TestGetTrend_BatchFetchError_ReturnsEmpty(t *testing.T) {
	sessions := []models.InterviewSession{{ID: 1, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
	reports := &trendReportRepo{batchErr: assert.AnError}
	svc := interview.NewInterviewService(&trendSessionRepo{finished: sessions}, &mockInterviewUtterRepo{}, reports, &mockUserRepo2{}, nil, nil, nil)

	points, err := svc.GetTrend(1, 20)
	require.NoError(t, err)
	assert.Empty(t, points)
	assert.Equal(t, 1, reports.batchCall)
}

func TestGetTrend_OldestFirst(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := []models.InterviewSession{{ID: 2, CreatedAt: base.Add(time.Hour)}, {ID: 1, CreatedAt: base}}
	reports := &trendReportRepo{reports: map[uint]models.InterviewReport{
		1: {SessionID: 1, ScoresJSON: `{}`}, 2: {SessionID: 2, ScoresJSON: `{}`},
	}}
	svc := interview.NewInterviewService(&trendSessionRepo{finished: sessions}, &mockInterviewUtterRepo{}, reports, &mockUserRepo2{}, nil, nil, nil)

	points, err := svc.GetTrend(1, 20)
	require.NoError(t, err)
	require.Len(t, points, 2)
	assert.Equal(t, []uint{1, 2}, []uint{points[0].SessionID, points[1].SessionID})
}
