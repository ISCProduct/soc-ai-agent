package company_test

import (
	"testing"
	"time"

	"Backend/domain/repository"
	"Backend/internal/controllers/mocks"
	"Backend/internal/models"
	"Backend/internal/services/company"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// 合成データ。起点(1)に、子 2..n がぶら下がる図を組み立てる。
func buildExclusionSvc(t *testing.T, all []models.Company, markets map[uint]*models.CompanyMarketInfo, rels []models.CompanyRelation) *company.CompanyCorrelationMapService {
	t.Helper()
	companyRepo := &mocks.CompanyRepositoryMock{}
	for i := range all {
		// 相関図の対象は有効な企業。停止中の扱いは除外規則とは別に確かめる（SOCAIAGENT-694）。
		all[i].IsActive = true
		c := all[i]
		companyRepo.On("FindByID", c.ID).Return(&c, nil).Maybe()
	}
	companyRepo.On("FindByIDs", mock.Anything).Return(all, nil)
	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(markets, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return(rels, nil)
	return company.NewCompanyCorrelationMapService(companyRepo, relRepo)
}

func nodeByID(m *company.CompanyCorrelationMap, id uint) (company.CorrelationMapNode, bool) {
	for _, n := range m.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return company.CorrelationMapNode{}, false
}

// checked は国税庁で確認済みの印。除外は確認済みの企業にだけ効く。
func checked() *time.Time {
	d := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return &d
}

func closedOn() *time.Time {
	d := time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC)
	return &d
}

func TestCorrelationMap_ExcludesPublicBodiesAndKeepsPrivateAndUnchecked(t *testing.T) {
	all := []models.Company{
		{ID: 1, Name: "起点", NameReading: "きてん", CorporateKind: "301", RegistryCheckedAt: checked()},
		{ID: 2, Name: "国の機関", NameReading: "あ", CorporateKind: "101", RegistryCheckedAt: checked()},
		{ID: 3, Name: "地方公共団体", NameReading: "い", CorporateKind: "201", RegistryCheckedAt: checked()},
		{ID: 4, Name: "民間企業", NameReading: "う", CorporateKind: "301", RegistryCheckedAt: checked()},
		{ID: 5, Name: "未確認", NameReading: "え", CorporateKind: ""},
	}
	rels := []models.CompanyRelation{
		capitalRel(1, 2, "capital_subsidiary"),
		capitalRel(1, 3, "capital_subsidiary"),
		capitalRel(1, 4, "capital_subsidiary"),
		capitalRel(1, 5, "capital_affiliate"),
		businessRel(1, 2, "business_partner", "取引"),
	}
	got, err := buildExclusionSvc(t, all, map[uint]*models.CompanyMarketInfo{}, rels).Build(1)
	require.NoError(t, err)

	_, ok2 := nodeByID(got, 2)
	_, ok3 := nodeByID(got, 3)
	_, ok4 := nodeByID(got, 4)
	_, ok5 := nodeByID(got, 5)
	assert.False(t, ok2)
	assert.False(t, ok3)
	assert.True(t, ok4)
	assert.True(t, ok5)
	assert.Len(t, got.CapitalEdges, 2)
	assert.Empty(t, got.BusinessEdges)
	for _, e := range got.CapitalEdges {
		assert.Contains(t, []uint{4, 5}, e.ChildID)
	}
}

func TestCorrelationMap_CountsAreComputedAfterExclusion(t *testing.T) {
	all := []models.Company{{ID: 1, Name: "起点", CorporateKind: "301", RegistryCheckedAt: checked()}}
	var rels []models.CompanyRelation
	// 国の機関 3 社 + 民間 6 社。除外後の子は 6 社だけ。
	for i := uint(2); i <= 10; i++ {
		kind := "301"
		if i <= 4 {
			kind = "101"
		}
		all = append(all, models.Company{ID: i, Name: "子", NameReading: string(rune('a' + i)), CorporateKind: kind, RegistryCheckedAt: checked()})
		rels = append(rels, capitalRel(1, i, "capital_subsidiary"))
	}
	got, err := buildExclusionSvc(t, all, map[uint]*models.CompanyMarketInfo{}, rels).Build(1)
	require.NoError(t, err)

	assert.Equal(t, 6, got.Meta.DirectChildTotal)
	assert.Len(t, got.Meta.DirectChildIDs, 6)
	assert.Len(t, got.Meta.InitialChildIDs, company.CorrelationMapInitialChildLimit)
	assert.Equal(t, 2, got.Meta.RemainingChildCount)
	for _, id := range got.Meta.DirectChildIDs {
		assert.Greater(t, id, uint(4))
	}
}

func TestCorrelationMap_FocusExcludedIsNotFound(t *testing.T) {
	cases := map[string]models.Company{
		"国の機関":    {ID: 1, Name: "起点", CorporateKind: "101", RegistryCheckedAt: checked()},
		"地方公共団体":  {ID: 1, Name: "起点", CorporateKind: "201", RegistryCheckedAt: checked()},
		"閉鎖された無名": {ID: 1, Name: "起点", CorporateKind: "301", RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
	}
	for name, focus := range cases {
		t.Run(name, func(t *testing.T) {
			all := []models.Company{focus, {ID: 2, Name: "子", CorporateKind: "301", RegistryCheckedAt: checked()}}
			svc := buildExclusionSvc(t, all, map[uint]*models.CompanyMarketInfo{}, []models.CompanyRelation{capitalRel(1, 2, "capital_subsidiary")})
			_, err := svc.Build(1)
			require.Error(t, err)
			assert.ErrorIs(t, err, repository.ErrNotFound)
		})
	}
}

func TestCorrelationMap_ClosedCompanies(t *testing.T) {
	listed := map[uint]*models.CompanyMarketInfo{3: {CompanyID: 3, IsListed: true, MarketType: "prime"}}
	all := []models.Company{
		{ID: 1, Name: "起点", NameReading: "きてん", CorporateKind: "301", RegistryCheckedAt: checked()},
		{ID: 2, Name: "無名の閉鎖", NameReading: "あ", RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
		{ID: 3, Name: "上場の閉鎖", NameReading: "い", RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
		{ID: 4, Name: "大規模の閉鎖", NameReading: "う", EmployeeCount: 1000, RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
		{ID: 5, Name: "閾値未満の閉鎖", NameReading: "え", EmployeeCount: 999, RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
		{ID: 6, Name: "閉鎖なし", NameReading: "お", CorporateKind: "301", RegistryCheckedAt: checked()},
	}
	var rels []models.CompanyRelation
	for i := uint(2); i <= 6; i++ {
		rels = append(rels, capitalRel(1, i, "capital_subsidiary"))
	}
	got, err := buildExclusionSvc(t, all, listed, rels).Build(1)
	require.NoError(t, err)

	_, ok2 := nodeByID(got, 2)
	_, ok5 := nodeByID(got, 5)
	assert.False(t, ok2, "無名の閉鎖企業は出ない")
	assert.False(t, ok5, "閾値未満の閉鎖企業は出ない")

	n3, ok3 := nodeByID(got, 3)
	require.True(t, ok3, "上場している閉鎖企業は残る")
	assert.True(t, n3.RegistryClosed)
	assert.Equal(t, "2020-04-01", n3.RegistryClosedOn)

	n4, ok4 := nodeByID(got, 4)
	require.True(t, ok4, "従業員数が閾値以上の閉鎖企業は残る")
	assert.True(t, n4.RegistryClosed)

	n6, ok6 := nodeByID(got, 6)
	require.True(t, ok6)
	assert.False(t, n6.RegistryClosed)
	assert.Empty(t, n6.RegistryClosedOn)
	assert.Equal(t, 3, got.Meta.DirectChildTotal)
}

func TestCorrelationMap_CompareUsesSameRules(t *testing.T) {
	listed := map[uint]*models.CompanyMarketInfo{3: {CompanyID: 3, IsListed: true}}
	all := []models.Company{
		{ID: 1, Name: "起点", MainBusiness: "事業A", CorporateKind: "301", RegistryCheckedAt: checked()},
		{ID: 2, Name: "国の機関", MainBusiness: "事業B", CorporateKind: "101", RegistryCheckedAt: checked()},
		{ID: 3, Name: "上場の閉鎖", MainBusiness: "事業C", RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
		{ID: 4, Name: "無名の閉鎖", MainBusiness: "事業D", RegistryClosedOn: closedOn(), RegistryCheckedAt: checked()},
		{ID: 5, Name: "未確認", MainBusiness: "事業E"},
	}
	svc := buildExclusionSvc(t, all, listed, nil)

	for _, id := range []uint{2, 4} {
		_, err := svc.CompareBusiness(1, id)
		require.Error(t, err, "選択側が除外対象")
		assert.ErrorIs(t, err, repository.ErrNotFound)
		_, err = svc.CompareBusiness(id, 1)
		require.Error(t, err, "起点側が除外対象")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	}
	for _, id := range []uint{3, 5} {
		_, err := svc.CompareBusiness(1, id)
		require.NoError(t, err)
	}
}

// 未確認（registry_checked_at が NULL）の企業は、種別や閉鎖日が入っていても除外しない。
func TestCorrelationMap_UncheckedIsNeverExcluded(t *testing.T) {
	all := []models.Company{
		{ID: 1, Name: "起点", NameReading: "きてん", CorporateKind: "301", RegistryCheckedAt: checked()},
		{ID: 2, Name: "未確認の101", NameReading: "あ", CorporateKind: "101"},
		{ID: 3, Name: "未確認の閉鎖", NameReading: "い", RegistryClosedOn: closedOn()},
	}
	rels := []models.CompanyRelation{capitalRel(1, 2, "capital_subsidiary"), capitalRel(1, 3, "capital_subsidiary")}
	svc := buildExclusionSvc(t, all, map[uint]*models.CompanyMarketInfo{}, rels)

	got, err := svc.Build(1)
	require.NoError(t, err)
	_, ok2 := nodeByID(got, 2)
	_, ok3 := nodeByID(got, 3)
	assert.True(t, ok2)
	assert.True(t, ok3)

	for _, id := range []uint{2, 3} {
		_, err := svc.CompareBusiness(1, id)
		require.NoError(t, err)
	}
}
