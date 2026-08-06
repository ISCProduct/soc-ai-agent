package company_test

import (
	"fmt"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/company"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func companyNode(id uint, name string) *models.Company {
	return &models.Company{ID: id, Name: name}
}

func capitalRel(parentID, childID uint, relationType string) models.CompanyRelation {
	return models.CompanyRelation{ParentID: ptrUint(parentID), ChildID: ptrUint(childID), RelationType: relationType}
}

func businessRel(fromID, toID uint, relationType, description string) models.CompanyRelation {
	return models.CompanyRelation{FromID: ptrUint(fromID), ToID: ptrUint(toID), RelationType: relationType, Description: description}
}

func TestCompanyRelationGraphService_BuildGraph_MultiLevelCapitalChain(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "親会社"), nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 2, Name: "子会社"}, {ID: 3, Name: "孫会社"},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.MatchedBy(func(ids []uint) bool {
		for _, id := range ids {
			if id == 1 {
				return true
			}
		}
		return false
	})).Return([]models.CompanyRelation{capitalRel(1, 2, "capital_subsidiary")}, nil).Once()
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(2, 3, "capital_affiliate"),
	}, nil).Once()
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{}, nil)

	svc := company.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)

	assert.Len(t, graph.Nodes, 3)
	assert.Len(t, graph.CapitalEdges, 2)
	assert.False(t, graph.Truncated)

	var focusCount int
	ids := map[uint]bool{}
	for _, n := range graph.Nodes {
		ids[n.ID] = true
		if n.IsFocus {
			focusCount++
			assert.Equal(t, uint(1), n.ID)
		}
	}
	assert.Equal(t, 1, focusCount)
	assert.True(t, ids[1] && ids[2] && ids[3])
}

func TestCompanyRelationGraphService_BuildGraph_AncestorDirection(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(2)).Return(companyNode(2, "起点企業"), nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 1, Name: "親会社"},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(1, 2, "capital_subsidiary"),
	}, nil).Once()
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{}, nil)

	svc := company.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(2)
	require.NoError(t, err)

	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.CapitalEdges, 1)
}

func TestCompanyRelationGraphService_BuildGraph_CycleDoesNotLoopForever(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "企業A"), nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 2, Name: "企業B"},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(1, 2, "capital_affiliate"),
		capitalRel(2, 1, "capital_affiliate"),
	}, nil).Once()
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		capitalRel(1, 2, "capital_affiliate"),
		capitalRel(2, 1, "capital_affiliate"),
	}, nil)

	svc := company.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)
	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.CapitalEdges, 2)
}

func TestCompanyRelationGraphService_BuildGraph_NodeCapTruncates(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "親会社"), nil)

	childCount := company.RelationGraphMaxNodes + 10
	relations := make([]models.CompanyRelation, 0, childCount)
	children := make([]models.Company, 0, childCount)
	for i := 0; i < childCount; i++ {
		childID := uint(100 + i)
		children = append(children, models.Company{ID: childID, Name: fmt.Sprintf("子会社%d", i)})
		relations = append(relations, capitalRel(1, childID, "capital_subsidiary"))
	}
	companyRepo.On("FindByIDs", mock.Anything).Return(children, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return(relations, nil).Once()
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{}, nil)

	svc := company.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)

	assert.True(t, graph.Truncated)
	assert.LessOrEqual(t, len(graph.Nodes), company.RelationGraphMaxNodes)
}

func TestCompanyRelationGraphService_BuildGraph_BusinessRelationsOnlyDirect(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "起点企業"), nil)
	companyRepo.On("FindByIDs", mock.Anything).Return([]models.Company{
		{ID: 2, Name: "取引先"},
	}, nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyIDs", mock.Anything).Return(map[uint]*models.CompanyMarketInfo{}, nil)
	relRepo.On("GetRelationsByCompanyIDs", mock.Anything).Return([]models.CompanyRelation{
		businessRel(1, 2, "business_partner", "主要取引先"),
	}, nil)

	svc := company.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)

	require.Len(t, graph.BusinessRelations, 1)
	assert.Equal(t, uint(2), graph.BusinessRelations[0].CompanyID)
	assert.Equal(t, "取引先", graph.BusinessRelations[0].Name)
	assert.Len(t, graph.Nodes, 1)
}
