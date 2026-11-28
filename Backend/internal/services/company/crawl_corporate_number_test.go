package company_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"Backend/internal/controllers/mocks"
	"Backend/internal/models"
	"Backend/internal/services/company"
	"Backend/internal/services/houjinbangou"
)

// fakeCrawlRepo は法人番号補完ジョブ1件だけを持つ。
type fakeCrawlRepo struct {
	src     models.CrawlSource
	created []string // CreateSource された target_type
}

func (f *fakeCrawlRepo) ListSources() ([]models.CrawlSource, error) {
	return []models.CrawlSource{f.src}, nil
}
func (f *fakeCrawlRepo) GetSource(uint) (*models.CrawlSource, error) { return &f.src, nil }
func (f *fakeCrawlRepo) CreateSource(s *models.CrawlSource) error {
	f.created = append(f.created, s.TargetType)
	return nil
}
func (f *fakeCrawlRepo) UpdateSource(*models.CrawlSource) error                 { return nil }
func (f *fakeCrawlRepo) ListDueSources(time.Time) ([]models.CrawlSource, error) { return nil, nil }
func (f *fakeCrawlRepo) CreateRun(*models.CrawlRun) error                       { return nil }
func (f *fakeCrawlRepo) UpdateRun(*models.CrawlRun) error                       { return nil }
func (f *fakeCrawlRepo) ListRuns(uint, int) ([]models.CrawlRun, error)          { return nil, nil }
func (f *fakeCrawlRepo) WithRunLock(_ context.Context, fn func()) (bool, error) {
	fn()
	return true, nil
}

// fakeFinder は企業名ごとの特定結果を返す。
type fakeFinder struct {
	byName map[string]*houjinbangou.Corporation
	calls  []string
}

func (f *fakeFinder) Enabled() bool { return true }
func (f *fakeFinder) ResolveCorporateNumber(_ context.Context, name, _ string) (*houjinbangou.Corporation, error) {
	f.calls = append(f.calls, name)
	return f.byName[name], nil
}

// 法人番号の日次補完: 一意に特定できた空欄だけを、法人番号の列だけ埋める（SOCAIAGENT-726）。
func TestCrawlService_FillCorporateNumbers(t *testing.T) {
	companies := []models.Company{ // FindAllActive と同じく ID の降順
		{ID: 6, Name: "桁がおかしい", IsActive: true},
		{ID: 5, Name: "ゲスト投稿", IsActive: true, IsGuestEntry: true},
		{ID: 4, Name: "番号あり", IsActive: true, CorporateNumber: "1111111111111"},
		{ID: 3, Name: "閉鎖済み", IsActive: true},
		{ID: 2, Name: "特定できない", IsActive: true},
		{ID: 1, Name: "特定できる", IsActive: true},
	}
	finder := &fakeFinder{byName: map[string]*houjinbangou.Corporation{
		"特定できる":  {CorporateNumber: "2222222222222"},
		"閉鎖済み":   {CorporateNumber: "3333333333333", CloseDate: "2020-01-01"},
		"桁がおかしい": {CorporateNumber: "12345"},
	}}
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActive", 200, 0).Return(companies, nil)
	repo.On("FillCorporateNumberIfEmpty", mock.Anything, uint(1), "2222222222222").Return(true, nil).Once()

	cr := &fakeCrawlRepo{src: models.CrawlSource{ID: 9, TargetType: "fill_corporate_numbers", ScheduleType: "daily", ScheduleTime: "04:00"}}
	svc := company.NewCrawlService(cr, repo, nil, nil)
	svc.SetCorporateNumberFinder(finder)
	run, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, "success", run.Status)
	repo.AssertExpectations(t) // 特定できた1社だけを、列だけ書く（Update は呼ばない）
	repo.AssertNotCalled(t, "Update", mock.Anything)
	assert.ElementsMatch(t, []string{"特定できる", "特定できない", "閉鎖済み", "桁がおかしい"}, finder.calls,
		"番号のある企業とゲスト投稿には問い合わせない")
	assert.Equal(t, "", cr.src.ProgressCursor, "最後まで見たら次回は先頭から")
}

// 上限で止まったら、次回は続き（前回より小さい ID）から再開する。特定できない企業が先頭にあっても後ろが埋まる。
func TestCrawlService_FillCorporateNumbers_ResumesFromCursor(t *testing.T) {
	t.Setenv("CORPORATE_NUMBER_FILL_DAILY_LIMIT", "2")
	companies := []models.Company{{ID: 30, Name: "a", IsActive: true}, {ID: 20, Name: "b", IsActive: true}, {ID: 10, Name: "c", IsActive: true}}
	finder := &fakeFinder{byName: map[string]*houjinbangou.Corporation{}}
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActive", 200, 0).Return(companies, nil)
	cr := &fakeCrawlRepo{src: models.CrawlSource{ID: 9, TargetType: "fill_corporate_numbers", ScheduleType: "daily", ScheduleTime: "04:00"}}
	svc := company.NewCrawlService(cr, repo, nil, nil)
	svc.SetCorporateNumberFinder(finder)

	_, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, finder.calls)
	assert.Equal(t, "20", cr.src.ProgressCursor)

	finder.calls = nil
	_, err = svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, []string{"c"}, finder.calls, "続きから再開する")
	assert.Equal(t, "", cr.src.ProgressCursor, "最後まで見たので先頭に戻る")
}
