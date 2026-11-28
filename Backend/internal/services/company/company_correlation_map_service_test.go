package company_test

import (
	"context"
	"testing"

	"Backend/internal/controllers/mocks"
	"Backend/internal/models"
	"Backend/internal/services/company"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDeriveShortBusiness_PrefersMainBusinessFirstSentence(t *testing.T) {
	text, src, known := company.DeriveShortBusiness(
		"企業向けのクラウド環境を設計・構築する。長期運用も支援する。",
		"長い説明文",
	)
	assert.True(t, known)
	assert.Equal(t, "main_business", src)
	assert.Equal(t, "企業向けのクラウド環境を設計・構築する", text)
}

func TestDeriveShortBusiness_EmptyWhenMissing(t *testing.T) {
	text, src, known := company.DeriveShortBusiness("  ", "")
	assert.False(t, known)
	assert.Equal(t, "", text)
	assert.Equal(t, "", src)
}

func TestCompanyCorrelationMapService_Build_DistinguishesSubsidiaryAndAffiliate(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(&models.Company{
		ID: 1, IsActive: true, Name: "起点", NameReading: "きてん", MainBusiness: "業務システムを企画・開発する。",
	}, nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, IsActive: true, Name: "起点", NameReading: "きてん", MainBusiness: "業務システムを企画・開発する。"},
		{ID: 2, IsActive: true, Name: "子会社A", NameReading: "あ"},
		{ID: 3, IsActive: true, Name: "関連会社B", NameReading: "い"},
		{ID: 4, IsActive: true, Name: "親会社", NameReading: "おや"},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{
		1: {CompanyID: 1, MarketType: "prime", IsListed: true},
	}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(4, 1, "capital_subsidiary"),
		capitalRel(1, 2, "capital_subsidiary"),
		capitalRel(1, 3, "capital_affiliate"),
		businessRel(1, 2, "business_partner", "在庫管理システムを提供"),
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	got, err := svc.Build(context.Background(), 1)
	require.NoError(t, err)

	assert.Equal(t, uint(1), got.FocusCompanyID)
	require.Len(t, got.CapitalEdges, 3)

	types := map[string]int{}
	for _, e := range got.CapitalEdges {
		types[e.RelationType]++
		assert.NotEmpty(t, e.RelationLabel)
	}
	assert.Equal(t, 2, types["capital_subsidiary"])
	assert.Equal(t, 1, types["capital_affiliate"])

	require.Len(t, got.BusinessEdges, 1)
	assert.Equal(t, uint(1), got.BusinessEdges[0].FromID)
	assert.Equal(t, uint(2), got.BusinessEdges[0].ToID)
	assert.True(t, got.BusinessEdges[0].DescriptionKnown)
	assert.Contains(t, got.BusinessEdges[0].Description, "在庫管理")

	assert.Equal(t, []uint{4}, got.Meta.DirectParentIDs)
	assert.Equal(t, []uint{2, 3}, got.Meta.DirectChildIDs) // あ, い
	assert.Equal(t, 2, got.Meta.DirectChildTotal)

	var focus company.CorrelationMapNode
	for _, n := range got.Nodes {
		if n.IsFocus {
			focus = n
		}
	}
	assert.True(t, focus.ShortBusinessKnown)
	assert.True(t, focus.MarketTypeKnown)
	assert.Equal(t, "prime", focus.MarketType)
	assert.Equal(t, "unavailable", focus.BusinessTagsStatus)
	assert.Empty(t, focus.BusinessTags)
}

func TestCompanyCorrelationMapService_Build_InitialChildLimit(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, IsActive: true, Name: "親"}, nil)

	children := make([]models.Company, 0, 6)
	rels := make([]models.CompanyRelation, 0, 6)
	for i := 2; i <= 7; i++ {
		children = append(children, models.Company{
			ID: uint(i), IsActive: true, Name: string(rune('A' + i - 2)), NameReading: string(rune('a' + i - 2)),
		})
		rels = append(rels, capitalRel(1, uint(i), "capital_subsidiary"))
	}
	all := append([]models.Company{{ID: 1, IsActive: true, Name: "親"}}, children...)
	companyRepo.On("FindByIDs", mock.Anything).Return(all, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return(rels, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	got, err := svc.Build(context.Background(), 1)
	require.NoError(t, err)

	assert.Equal(t, 6, got.Meta.DirectChildTotal)
	assert.Equal(t, company.CorrelationMapInitialChildLimit, len(got.Meta.InitialChildIDs))
	assert.Equal(t, 2, got.Meta.RemainingChildCount)
	assert.Equal(t, got.Meta.DirectChildIDs[:4], got.Meta.InitialChildIDs)
}

func TestCompanyCorrelationMapService_CompareBusiness(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, IsActive: true, Name: "A", MainBusiness: "業務システム全体を企画・開発する。"},
		{ID: 2, IsActive: true, Name: "B", MainBusiness: "システムを動かすクラウド環境を構築する。"},
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, &relationRepoMock{})
	got, err := svc.CompareBusiness(1, 2)
	require.NoError(t, err)
	assert.Equal(t, false, got["compare_text_available"])
	focus := got["focus"].(map[string]any)
	assert.Contains(t, focus["short_business"], "業務システム")
}

// 再取得で parent/child が逆転した鏡像行が残っていても、学生向けAPIは
// 同じ2社を「親かつ子」にしない。
func TestCompanyCorrelationMapService_Build_CollapsesMirroredCapitalEdges(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(18)).Return(&models.Company{ID: 18, IsActive: true, Name: "親会社", NameReading: "おや"}, nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 18, IsActive: true, Name: "親会社", NameReading: "おや"},
		{ID: 20, IsActive: true, Name: "子会社", NameReading: "こ"},
	}, nil)

	r100 := 100.0
	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{
		18: {CompanyID: 18, MarketType: "prime", IsListed: true},
	}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		{ID: 1, IsActive: true, ParentID: ptrUint(18), ChildID: ptrUint(20), RelationType: "capital_subsidiary", Ratio: &r100},
		{ID: 2, IsActive: true, ParentID: ptrUint(20), ChildID: ptrUint(18), RelationType: "capital_subsidiary", Ratio: &r100},
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	got, err := svc.Build(context.Background(), 18)
	require.NoError(t, err)

	require.Len(t, got.CapitalEdges, 1)
	assert.Equal(t, uint(18), got.CapitalEdges[0].ParentID)
	assert.Equal(t, uint(20), got.CapitalEdges[0].ChildID)
	assert.Equal(t, []uint{20}, got.Meta.DirectChildIDs)
	assert.Empty(t, got.Meta.DirectParentIDs)
}

func TestCompanyCorrelationMapService_Build_BusinessDirectionNotTrusted(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, IsActive: true, Name: "A"}, nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, IsActive: true, Name: "A"},
		{ID: 2, IsActive: true, Name: "B"},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(1, 2, "capital_subsidiary"),
		businessRel(1, 2, "business_partner", "在庫管理システムを提供"),
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	got, err := svc.Build(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got.BusinessEdges, 1)
	// from/to は取得起点→相手であり提供側→利用側ではない
	assert.False(t, got.BusinessEdges[0].DirectionKnown)
	var focus company.CorrelationMapNode
	for _, n := range got.Nodes {
		if n.IsFocus {
			focus = n
		}
	}
	assert.Equal(t, "/company/1", focus.DetailPath)
}

func TestCompanyCorrelationMapService_Build_HidesUnpublishedGuestEntry(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, IsActive: true, Name: "公開企業"}, nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, IsActive: true, Name: "公開企業"},
		{ID: 99, Name: "ゲスト投稿", DataStatus: "draft", IsGuestEntry: true, IsActive: true},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(1, 99, "capital_subsidiary"),
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	got, err := svc.Build(context.Background(), 1)
	require.NoError(t, err)
	for _, n := range got.Nodes {
		assert.NotEqual(t, uint(99), n.ID)
	}
	assert.Empty(t, got.CapitalEdges)
	assert.Empty(t, got.Meta.DirectChildIDs)
}

// 公開API。審査前のゲスト投稿企業は、事業の比較でも返さない。
func TestCompanyCorrelationMapService_CompareBusiness_HidesUnpublishedGuestEntry(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, IsActive: true, Name: "公開企業", MainBusiness: "事業A"},
		{ID: 99, Name: "ゲスト投稿", MainBusiness: "事業B", DataStatus: "draft", IsGuestEntry: true, IsActive: true},
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, &relationRepoMock{})
	_, err := svc.CompareBusiness(1, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// tagRepoStub は事業タグの読み出しを固定値で返す。
type tagRepoStub struct {
	byID     map[uint][]string
	askedIDs []uint
}

func (s *tagRepoStub) ListAll(context.Context) ([]models.BusinessTag, error) { return nil, nil }
func (s *tagRepoStub) NamesByCompanyIDs(_ context.Context, ids []uint) (map[uint][]string, error) {
	s.askedIDs = ids
	return s.byID, nil
}
func (s *tagRepoStub) ReplaceForCompany(context.Context, uint, []string) error { return nil }

func TestCompanyCorrelationMapService_Build_BusinessTags(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "親", IsActive: true}, nil)
	all := []models.Company{{ID: 1, Name: "親", Industry: "情報通信業", IsActive: true}}
	rels := []models.CompanyRelation{}
	for i := 2; i <= 7; i++ {
		all = append(all, models.Company{ID: uint(i), Name: string(rune('A' + i - 2)), NameReading: string(rune('a' + i - 2)), IsActive: true})
		rels = append(rels, capitalRel(1, uint(i), "capital_subsidiary"))
	}
	companyRepo.On("FindByIDs", mock.Anything).Return(all, nil)
	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return(rels, nil)

	// 7 は初期表示の4社に入らない（読み仮名順で最後）
	tags := &tagRepoStub{byID: map[uint][]string{7: {"クラウド", "業務システム"}}}
	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	svc.SetBusinessTagRepository(tags)
	got, err := svc.Build(context.Background(), 1)
	require.NoError(t, err)
	require.NotContains(t, got.Meta.InitialChildIDs, uint(7))
	assert.Len(t, tags.askedIDs, 7, "返すノード全件をまとめて引く")

	for _, n := range got.Nodes {
		switch n.ID {
		case 7:
			assert.Equal(t, []string{"クラウド", "業務システム"}, n.BusinessTags)
			assert.Equal(t, "available", n.BusinessTagsStatus)
		default:
			// 業種があってもタグとして補わない
			assert.Equal(t, []string{}, n.BusinessTags)
			assert.Equal(t, "unavailable", n.BusinessTagsStatus)
		}
	}
}

// SOCAIAGENT-694: 停止中(is_active=false)の企業は、ゲスト投稿でなくても相関図に出さない。
func TestCompanyCorrelationMapService_Build_HidesInactiveCompany(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "公開企業", IsActive: true}, nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, Name: "公開企業", IsActive: true},
		{ID: 98, Name: "停止中企業", DataStatus: "published", IsActive: false},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(1, 98, "capital_subsidiary"),
	}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	got, err := svc.Build(context.Background(), 1)
	require.NoError(t, err)
	for _, n := range got.Nodes {
		assert.NotEqual(t, uint(98), n.ID)
	}
	assert.Empty(t, got.CapitalEdges)
}

// 停止中の企業そのものの相関図は返さない。
func TestCompanyCorrelationMapService_Build_InactiveFocusNotFound(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(98)).Return(&models.Company{ID: 98, Name: "停止中企業", IsActive: false}, nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{{ID: 98, Name: "停止中企業", IsActive: false}}, nil)
	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{}, nil)

	svc := company.NewCompanyCorrelationMapService(companyRepo, relRepo)
	_, err := svc.Build(context.Background(), 98)
	require.Error(t, err)
}
