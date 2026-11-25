package company_test

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"Backend/internal/controllers/mocks"
	"Backend/internal/models"
	"Backend/internal/services/company"
	"Backend/internal/services/edinet"
)

// fakeEDINET は日付ごとの書類一覧と、書類ごとの ZIP を返す。値は合成データ。
type fakeEDINET struct {
	docsByDate map[string][]edinet.Document
	rateAt     string // この日の一覧で 429 を返す
	listed     []string
	fetched    []string
}

func (f *fakeEDINET) Enabled() bool { return true }
func (f *fakeEDINET) ListDocuments(_ context.Context, d time.Time) ([]edinet.Document, error) {
	k := d.Format("2006-01-02")
	f.listed = append(f.listed, k)
	if k == f.rateAt {
		return nil, edinet.ErrRateLimited
	}
	return f.docsByDate[k], nil
}
func (f *fakeEDINET) FetchCSVZip(_ context.Context, docID string) ([]byte, error) {
	f.fetched = append(f.fetched, docID)
	return annualReportZip(), nil
}

func annualReportZip() []byte {
	text := "要素ID\t項目名\tコンテキストID\t相対年度\t連結・個別\t期間・時点\tユニットID\t単位\t値\r\n" +
		"jpcrp_cor:NumberOfEmployees\t\tCurrentYearInstant\t\t\t\t\t\t110\r\n" +
		"jpcrp_cor:DescriptionOfBusinessTextBlock\t\tFilingDateInstant\t\t\t\t\t\t事業の内容（合成データ）\r\n"
	u := utf16.Encode([]rune(text))
	raw := []byte{0xFF, 0xFE}
	for _, c := range u {
		raw = append(raw, byte(c), byte(c>>8))
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("XBRL_TO_CSV/jpcrp030000-asr-001_E99999-000_2025-03-31_01_2025-06-25.csv")
	_, _ = w.Write(raw)
	_ = zw.Close()
	return buf.Bytes()
}

// fakeResearch は保存された文章と数字を覚える。
type fakeResearch struct {
	docs  []models.CompanyDocument
	facts []models.CompanyFact
}

func (f *fakeResearch) UpsertDocument(_ context.Context, d *models.CompanyDocument) (bool, error) {
	for i := range f.docs {
		if f.docs[i].CompanyID == d.CompanyID && f.docs[i].SourceType == d.SourceType && f.docs[i].SourceRef == d.SourceRef {
			f.docs[i] = *d
			return true, nil
		}
	}
	f.docs = append(f.docs, *d)
	return true, nil
}
func (f *fakeResearch) ListDocuments(_ context.Context, id uint) ([]models.CompanyDocument, error) {
	var out []models.CompanyDocument
	for _, d := range f.docs {
		if d.CompanyID == id {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeResearch) UpsertFacts(_ context.Context, fs []models.CompanyFact) error {
	f.facts = append(f.facts, fs...)
	return nil
}
func (f *fakeResearch) ListFacts(context.Context, uint) ([]models.CompanyFact, error) {
	return f.facts, nil
}

func annual(docID, jcn, periodEnd string) edinet.Document {
	return edinet.Document{DocID: docID, JCN: jcn, DocTypeCode: "120", OrdinanceCode: "010", WithdrawalStatus: "0", CSVFlag: "1", PeriodEnd: periodEnd}
}

func day(offset int) string {
	jst := time.FixedZone("JST", 9*60*60)
	n := time.Now().In(jst)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, jst).AddDate(0, 0, offset).Format("2006-01-02")
}

func newEDINETService(t *testing.T, cursor string, fe *fakeEDINET, fr *fakeResearch) (*company.CrawlService, *fakeCrawlRepo) {
	t.Helper()
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByCorporateNumber", "1000000000001").Return(&models.Company{ID: 1, IsActive: true}, nil)
	repo.On("FindByCorporateNumber", "1000000000002").Return(&models.Company{ID: 2, IsActive: false}, nil)
	repo.On("FindByCorporateNumber", mock.Anything).Return(nil, gorm.ErrRecordNotFound)
	cr := &fakeCrawlRepo{src: models.CrawlSource{ID: 9, TargetType: "edinet_daily", ScheduleType: "daily", ScheduleTime: "05:00", ProgressCursor: cursor}}
	svc := company.NewCrawlService(cr, repo, nil, nil)
	svc.SetEDINETIngest(fe, fr)
	return svc, cr
}

// 前回の翌日から昨日まで処理し、自社の企業の有報だけを取り込んで位置を進める。
func TestCrawlService_EDINETDaily_IngestsAndAdvances(t *testing.T) {
	fe := &fakeEDINET{docsByDate: map[string][]edinet.Document{
		day(-2): {
			annual("S1", "1000000000001", "2025-03-31"),
			annual("S2", "9999999999999", "2025-03-31"), // 自社に無い会社
			annual("S3", "1000000000002", "2025-03-31"), // 停止中の会社
			{DocID: "S4", JCN: "1000000000001", DocTypeCode: "180", OrdinanceCode: "010", WithdrawalStatus: "0", CSVFlag: "1"}, // 臨時報告書
		},
	}}
	fr := &fakeResearch{}
	svc, cr := newEDINETService(t, day(-3), fe, fr)

	run, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, "success", run.Status)
	assert.Equal(t, []string{day(-2), day(-1)}, fe.listed, "当日は処理しない")
	assert.Equal(t, []string{"S1"}, fe.fetched)
	assert.Equal(t, day(-1), cr.src.ProgressCursor)
	require.Len(t, fr.docs, 1)
	assert.Equal(t, "annual_report:business", fr.docs[0].SourceRef)
	assert.Contains(t, fr.docs[0].Title, "2025年3月期")
	assert.Contains(t, fr.docs[0].URL, "S1")
	require.Len(t, fr.facts, 1)
	assert.Equal(t, "employees_consolidated", fr.facts[0].Field)
}

// 429 を受けた日は進めず、次回その日から再開する。
func TestCrawlService_EDINETDaily_RateLimitKeepsCursor(t *testing.T) {
	fe := &fakeEDINET{rateAt: day(-2)}
	svc, cr := newEDINETService(t, day(-4), fe, &fakeResearch{})
	_, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, day(-3), cr.src.ProgressCursor)
}

// 1回で処理する日数に上限がある（初回のさかのぼりも少しずつ進む）。
func TestCrawlService_EDINETDaily_DaysPerRun(t *testing.T) {
	t.Setenv("EDINET_DAYS_PER_RUN", "3")
	fe := &fakeEDINET{}
	svc, cr := newEDINETService(t, "", fe, &fakeResearch{})
	_, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Len(t, fe.listed, 3)
	assert.Equal(t, day(-456+2), cr.src.ProgressCursor, "初回は約15か月前から")
}

// 新しい期の有報を既に持っていれば、古い期で上書きしない。
func TestCrawlService_EDINETDaily_SkipsOlderPeriod(t *testing.T) {
	newer := time.Date(2025, 3, 31, 0, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	fr := &fakeResearch{docs: []models.CompanyDocument{{CompanyID: 1, SourceType: "edinet", SourceRef: "annual_report:business", AsOf: &newer, Body: "新しい期"}}}
	fe := &fakeEDINET{docsByDate: map[string][]edinet.Document{day(-1): {annual("OLD", "1000000000001", "2024-03-31")}}}
	svc, _ := newEDINETService(t, day(-2), fe, fr)
	_, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Empty(t, fe.fetched)
	assert.True(t, strings.Contains(fr.docs[0].Body, "新しい期"))
}
