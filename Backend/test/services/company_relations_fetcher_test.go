package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func (m *relationRepoMock) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.(*models.CompanyMarketInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *relationRepoMock) UpsertMarketInfo(info *models.CompanyMarketInfo) error {
	args := m.Called(info)
	return args.Error(0)
}

func validCompanyRelationsJSON() string {
	return `{"subsidiaries":[{"name":"子会社A","ratio":100}],"affiliates":[{"name":"関連会社B"}],"business_partners":[{"name":"取引先C"}],"market_info":{"is_listed":true,"market_type":"prime","stock_code":"4755"}}`
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

	fetcher := services.NewCompanyRelationsFetcher(repo, relRepo, nil)
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
	repo.On("FindByName", "子会社A").Return(nil, errors.New("record not found"))
	repo.On("FindByName", "関連会社B").Return(nil, errors.New("record not found"))
	repo.On("FindByName", "取引先C").Return(nil, errors.New("record not found"))
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
	fetcher := services.NewCompanyRelationsFetcher(repo, relRepo, client)
	result, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.NoError(t, err)
	assert.Len(t, result.Relations, 3)
	assert.Equal(t, "prime", result.MarketInfo.MarketType)
	assert.Equal(t, "web_search", result.Source)
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

	fetcher := services.NewCompanyRelationsFetcher(repo, relRepo, nil)
	result, err := fetcher.ConfirmAndSave(1, &services.CompanyRelationsResult{
		Relations: []services.RelationEntry{
			{Name: "子会社A", RelationType: "capital_subsidiary"},
		},
		MarketInfo: &services.CompanyMarketInfoResult{
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

func TestCompanyRelationsFetcher_ConfirmAndSave_NilResult(t *testing.T) {
	fetcher := services.NewCompanyRelationsFetcher(&mocks.CompanyRepositoryMock{}, &relationRepoMock{}, nil)
	_, err := fetcher.ConfirmAndSave(1, nil)
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
	fetcher := services.NewCompanyRelationsFetcher(repo, relRepo, client)
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
