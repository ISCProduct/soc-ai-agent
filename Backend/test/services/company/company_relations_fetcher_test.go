package company_test

import (
	"context"
	"testing"
	"time"

	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/company"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type relationRepoMock struct {
	mock.Mock
}

func (m *relationRepoMock) UpsertBusinessRelation(fromID, toID uint, relationType, description string) error {
	args := m.Called(fromID, toID, relationType, description)
	return args.Error(0)
}

func (m *relationRepoMock) UpsertCapitalRelation(parentID, childID uint, relationType string, ratio *float64, description string) error {
	args := m.Called(parentID, childID, relationType, ratio, description)
	return args.Error(0)
}

func (m *relationRepoMock) GetRelationsByCompanyID(companyID uint) ([]models.CompanyRelation, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyRelation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *relationRepoMock) GetRelationsByCompanyIDs(companyIDs []uint) ([]models.CompanyRelation, error) {
	args := m.Called(companyIDs)
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyRelation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *relationRepoMock) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.(*models.CompanyMarketInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *relationRepoMock) GetMarketInfoByCompanyIDs(companyIDs []uint) (map[uint]*models.CompanyMarketInfo, error) {
	args := m.Called(companyIDs)
	if v := args.Get(0); v != nil {
		return v.(map[uint]*models.CompanyMarketInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *relationRepoMock) UpsertMarketInfo(info *models.CompanyMarketInfo) error {
	args := m.Called(info)
	return args.Error(0)
}

func validCompanyRelationsJSON() string {
	return `{"subsidiaries":[{"name":"子会社A","ratio":100,"description":"完全子会社"}],"affiliates":[{"name":"関連会社B","description":"資本業務提携"}],"business_partners":[{"name":"取引先C","description":"決済代行"}],"market_info":{"is_listed":true,"market_type":"prime","stock_code":"4755"}}`
}

func TestCompanyRelationsFetcher_FetchAndSave_TTLCache(t *testing.T) {
	now := time.Now()
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{
		ID:                 1,
		Name:               "テスト株式会社",
		RelationsFetchedAt: &now,
	}, nil)

	relRepo := &relationRepoMock{}
	childID := uint(2)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{
		{
			ParentID:     ptrUint(1),
			ChildID:      &childID,
			RelationType: "capital_subsidiary",
			Child:        &models.Company{ID: 2, Name: "子会社A"},
		},
	}, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(&models.CompanyMarketInfo{
		CompanyID:  1,
		IsListed:   true,
		MarketType: "prime",
		StockCode:  "4755",
	}, nil)

	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, nil)
	result, err := fetcher.FetchAndSave(context.Background(), 1, false)
	require.NoError(t, err)
	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "子会社A", result.Relations[0].Name)
	assert.Equal(t, "4755", result.MarketInfo.StockCode)
	assert.True(t, result.FromCache)
	assert.Equal(t, "ttl", result.SkipReason)
	repo.AssertNotCalled(t, "Update", mock.Anything)
}

func TestCompanyRelationsFetcher_FetchAndSave_AISearch(t *testing.T) {
	srv := makeChatCompletionsServer(t, validCompanyRelationsJSON())
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "テスト株式会社"}, nil)
	repo.On("FindByName", "子会社A").Return(nil, gorm.ErrRecordNotFound)
	repo.On("FindByName", "関連会社B").Return(nil, gorm.ErrRecordNotFound)
	repo.On("FindByName", "取引先C").Return(nil, gorm.ErrRecordNotFound)
	repo.On("Create", mock.AnythingOfType("*models.Company")).Return(nil).Times(3)
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		assert.NotNil(t, c.RelationsFetchedAt)
	})

	relRepo := &relationRepoMock{}
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{}, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)
	relRepo.On("UpsertCapitalRelation", uint(1), mock.Anything, "capital_subsidiary", mock.Anything, mock.Anything).Return(nil)
	relRepo.On("UpsertCapitalRelation", uint(1), mock.Anything, "capital_affiliate", mock.Anything, mock.Anything).Return(nil)
	relRepo.On("UpsertBusinessRelation", uint(1), mock.Anything, "business_partner", mock.Anything).Return(nil)
	relRepo.On("UpsertMarketInfo", mock.AnythingOfType("*models.CompanyMarketInfo")).Return(nil)

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, client)
	result, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.NoError(t, err)
	assert.Len(t, result.Relations, 3)
	assert.Equal(t, "prime", result.MarketInfo.MarketType)
	assert.Equal(t, "web_search", result.Source)
	repo.AssertNumberOfCalls(t, "Create", 3)
}

// 関連企業として新規作成された会社にも、infoFetcherを注入していれば詳細情報が
// 自動で埋まることを確認する(空データの企業が量産される問題への回帰テスト)。
func TestCompanyRelationsFetcher_FetchAndSave_FillsNewRelatedCompanyDetails(t *testing.T) {
	relationsJSON := `{"subsidiaries":[{"name":"子会社A","ratio":100,"description":"完全子会社"}],"affiliates":[],"business_partners":[],"market_info":{"is_listed":false,"market_type":"unlisted","stock_code":""}}`
	relationsSrv := makeChatCompletionsServer(t, relationsJSON)
	defer relationsSrv.Close()
	infoSrv := makeChatCompletionsServer(t, validCompanyInfoJSON())
	defer infoSrv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "テスト株式会社"}, nil)
	repo.On("FindByName", "子会社A").Return(nil, gorm.ErrRecordNotFound)
	repo.On("Create", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		c.ID = 2
	})
	repo.On("FindByID", uint(2)).Return(&models.Company{ID: 2, Name: "子会社A"}, nil)

	childUpdated := false
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		if c.ID == 2 {
			childUpdated = true
			assert.Equal(t, "テスト企業の概要", c.Description)
			assert.Equal(t, "IT・ソフトウェア", c.Industry)
		}
	})

	relRepo := &relationRepoMock{}
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{}, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)
	relRepo.On("UpsertCapitalRelation", uint(1), uint(2), "capital_subsidiary", mock.Anything, mock.Anything).Return(nil)
	relRepo.On("UpsertMarketInfo", mock.AnythingOfType("*models.CompanyMarketInfo")).Return(nil)

	relationsClient := openai.NewWithBaseURL(relationsSrv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, relationsClient)

	infoClient := openai.NewWithBaseURL(infoSrv.URL, "gpt-4o-mini")
	fetcher.SetInfoFetcher(company.NewCompanyInfoFetcher(repo, infoClient))

	_, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.NoError(t, err)
	assert.True(t, childUpdated, "新規作成された関連企業(id=2)の詳細情報が取得・保存されるべき")
}

func TestCompanyRelationsFetcher_ConfirmAndSave(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "テスト株式会社"}, nil)
	repo.On("FindByName", "子会社A").Return(&models.Company{ID: 2, Name: "子会社A"}, nil)
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		assert.NotNil(t, c.RelationsFetchedAt)
	})

	relRepo := &relationRepoMock{}
	relRepo.On("UpsertCapitalRelation", uint(1), uint(2), "capital_subsidiary", mock.Anything, mock.Anything).Return(nil)
	relRepo.On("UpsertMarketInfo", mock.AnythingOfType("*models.CompanyMarketInfo")).Return(nil)

	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, nil)
	result, err := fetcher.ConfirmAndSave(context.Background(), 1, &company.CompanyRelationsResult{
		Relations: []company.RelationEntry{
			{Name: "子会社A", RelationType: "capital_subsidiary"},
		},
		MarketInfo: &company.CompanyMarketInfoResult{
			IsListed:   true,
			MarketType: "prime",
			StockCode:  "4755",
		},
		Source:     "web_search",
		ModelUsed:  "gpt-4o-mini",
		Confidence: "medium",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.SavedCount)
}

func TestCompanyRelationsFetcher_FetchAndSave_EmptyUnlistedStamps(t *testing.T) {
	emptyJSON := `{"subsidiaries":[],"affiliates":[],"business_partners":[],"market_info":{"is_listed":false,"market_type":"unlisted","stock_code":""}}`
	srv := makeChatCompletionsServer(t, emptyJSON)
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "テスト株式会社"}, nil)
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		assert.NotNil(t, c.RelationsFetchedAt)
	})

	relRepo := &relationRepoMock{}
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{}, nil)
	// HasStoredData / acquire 双方から呼ばれるため、非上場情報を一貫して返す
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(&models.CompanyMarketInfo{
		CompanyID:  1,
		IsListed:   false,
		MarketType: "unlisted",
	}, nil)
	relRepo.On("UpsertMarketInfo", mock.MatchedBy(func(info *models.CompanyMarketInfo) bool {
		return info != nil && info.CompanyID == 1 && info.MarketType == "unlisted" && !info.IsListed
	})).Return(nil)

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, client)
	result, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SavedCount)
	assert.Empty(t, result.Relations)
	assert.True(t, fetcher.HasStoredData(1))
	repo.AssertCalled(t, "Update", mock.AnythingOfType("*models.Company"))
}

func TestCompanyRelationsFetcher_TTLSkipsWhenConfirmedUnlisted(t *testing.T) {
	now := time.Now()
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{
		ID:                 1,
		Name:               "テスト株式会社",
		RelationsFetchedAt: &now,
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{}, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(&models.CompanyMarketInfo{
		CompanyID:  1,
		IsListed:   false,
		MarketType: "unlisted",
	}, nil)

	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, nil)
	result, err := fetcher.FetchAndSave(context.Background(), 1, false)
	require.NoError(t, err)
	assert.True(t, result.FromCache)
	assert.Equal(t, "ttl", result.SkipReason)
	repo.AssertNotCalled(t, "Update", mock.Anything)
}

func TestCompanyRelationsFetcher_ConfirmAndSave_NilResult(t *testing.T) {
	fetcher := company.NewCompanyRelationsFetcher(&mocks.CompanyRepositoryMock{}, &relationRepoMock{}, nil)
	_, err := fetcher.ConfirmAndSave(context.Background(), 1, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "result is required")
}

type denyRelationsBudget struct{}

func (denyRelationsBudget) AllowSearch() error {
	return companyfetch.ErrSearchBudgetExceeded
}

func TestCompanyRelationsFetcher_FetchAndSave_BudgetExceededUsesCache(t *testing.T) {
	srv := makeChatCompletionsServer(t, validCompanyRelationsJSON())
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "テスト株式会社"}, nil)

	relRepo := &relationRepoMock{}
	childID := uint(2)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{
		{
			ParentID:     ptrUint(1),
			ChildID:      &childID,
			RelationType: "capital_subsidiary",
			Child:        &models.Company{ID: 2, Name: "キャッシュ子会社"},
		},
	}, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(&models.CompanyMarketInfo{
		CompanyID:  1,
		MarketType: "unlisted",
	}, nil)

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyRelationsFetcher(repo, relRepo, client)
	fetcher.SetSearchBudget(denyRelationsBudget{})

	result, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.NoError(t, err)
	assert.True(t, result.FromCache)
	assert.True(t, result.BudgetExceeded)
	assert.Equal(t, "budget", result.SkipReason)
	assert.Equal(t, "キャッシュ子会社", result.Relations[0].Name)
	repo.AssertNotCalled(t, "Update", mock.Anything)
}

func ptrUint(v uint) *uint { return &v }
