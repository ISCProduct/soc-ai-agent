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
type fakeCrawlRepo struct{ src models.CrawlSource }

func (f *fakeCrawlRepo) ListSources() ([]models.CrawlSource, error) {
	return []models.CrawlSource{f.src}, nil
}
func (f *fakeCrawlRepo) GetSource(uint) (*models.CrawlSource, error)            { return &f.src, nil }
func (f *fakeCrawlRepo) CreateSource(*models.CrawlSource) error                 { return nil }
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

// 法人番号の日次補完: 一意に特定できた空欄だけを埋める（SOCAIAGENT-726 の前提）。
func TestCrawlService_FillCorporateNumbers(t *testing.T) {
	companies := []models.Company{
		{ID: 1, Name: "特定できる", IsActive: true},
		{ID: 2, Name: "特定できない", IsActive: true},
		{ID: 3, Name: "閉鎖済み", IsActive: true},
		{ID: 4, Name: "番号あり", IsActive: true, CorporateNumber: "1111111111111"},
		{ID: 5, Name: "ゲスト投稿", IsActive: true, IsGuestEntry: true},
		{ID: 6, Name: "桁がおかしい", IsActive: true},
	}
	finder := &fakeFinder{byName: map[string]*houjinbangou.Corporation{
		"特定できる":  {CorporateNumber: "2222222222222"},
		"閉鎖済み":   {CorporateNumber: "3333333333333", CloseDate: "2020-01-01"},
		"桁がおかしい": {CorporateNumber: "12345"},
	}}
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActive", 200, 0).Return(companies, nil)
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "特定できる"}, nil)
	var updated []models.Company
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(a mock.Arguments) {
		updated = append(updated, *a.Get(0).(*models.Company))
	})

	svc := company.NewCrawlService(&fakeCrawlRepo{src: models.CrawlSource{ID: 9, TargetType: "fill_corporate_numbers", ScheduleType: "daily", ScheduleTime: "04:00"}}, repo, nil, nil)
	svc.SetCorporateNumberFinder(finder)
	run, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, "success", run.Status)

	require.Len(t, updated, 1, "特定できた1社だけを書く")
	assert.Equal(t, uint(1), updated[0].ID)
	assert.Equal(t, "2222222222222", updated[0].CorporateNumber)
	assert.ElementsMatch(t, []string{"特定できる", "特定できない", "閉鎖済み", "桁がおかしい"}, finder.calls,
		"番号のある企業とゲスト投稿には問い合わせない")
}

// 1回の上限を超えて問い合わせない。
func TestCrawlService_FillCorporateNumbers_Limit(t *testing.T) {
	t.Setenv("CORPORATE_NUMBER_FILL_DAILY_LIMIT", "2")
	companies := []models.Company{{ID: 1, Name: "a", IsActive: true}, {ID: 2, Name: "b", IsActive: true}, {ID: 3, Name: "c", IsActive: true}}
	finder := &fakeFinder{byName: map[string]*houjinbangou.Corporation{}}
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActive", 200, 0).Return(companies, nil)

	svc := company.NewCrawlService(&fakeCrawlRepo{src: models.CrawlSource{ID: 9, TargetType: "fill_corporate_numbers", ScheduleType: "daily", ScheduleTime: "04:00"}}, repo, nil, nil)
	svc.SetCorporateNumberFinder(finder)
	_, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Len(t, finder.calls, 2)
}
