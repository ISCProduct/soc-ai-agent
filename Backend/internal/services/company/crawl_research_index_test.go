package company_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Backend/internal/controllers/mocks"
	"Backend/internal/models"
	"Backend/internal/ragclient"
	"Backend/internal/services/company"
)

type fakeIndexer struct {
	failFor map[uint]bool
	calls   map[uint][]uint // 企業ID → 送った文書ID
}

func (f *fakeIndexer) Enabled() bool { return true }
func (f *fakeIndexer) Index(_ context.Context, companyID uint, docs []ragclient.ResearchDocument) (ragclient.Usage, error) {
	for _, d := range docs {
		f.calls[companyID] = append(f.calls[companyID], d.DocumentID)
	}
	if f.failFor[companyID] {
		return ragclient.Usage{}, errors.New("rag down")
	}
	return ragclient.Usage{Model: "text-embedding-3-small", PromptTokens: 100, Calls: 1}, nil
}

// 未登録の文章を企業ごとにまとめて RAG へ送り、成功した分だけ登録済みにする（SOCAIAGENT-725）。
func TestCrawlService_ResearchIndex(t *testing.T) {
	fr := &fakeResearch{docs: []models.CompanyDocument{
		{ID: 1, CompanyID: 1, SourceType: "edinet", Body: "a"},
		{ID: 2, CompanyID: 1, SourceType: "edinet", Body: "b"},
		{ID: 3, CompanyID: 2, SourceType: "edinet", Body: "c"},
	}}
	ix := &fakeIndexer{failFor: map[uint]bool{2: true}, calls: map[uint][]uint{}}
	cr := &fakeCrawlRepo{src: models.CrawlSource{ID: 9, TargetType: "company_research_index", ScheduleType: "daily", ScheduleTime: "05:30"}}
	svc := company.NewCrawlService(cr, &mocks.CompanyRepositoryMock{}, nil, nil)
	svc.SetEDINETIngest(nil, fr)
	svc.SetResearchIndexer(ix)

	run, err := svc.RunSource(9)
	require.NoError(t, err)
	assert.Equal(t, "success", run.Status, "一部の失敗は次回に回す")
	assert.Equal(t, []uint{1, 2}, ix.calls[1], "同じ企業はまとめて送る")
	assert.Equal(t, []uint{3}, ix.calls[2])
	assert.NotNil(t, fr.docs[0].IndexedAt)
	assert.NotNil(t, fr.docs[1].IndexedAt)
	assert.Nil(t, fr.docs[2].IndexedAt, "失敗した企業は未登録のまま残し、次回拾い直す")
}
