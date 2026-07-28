package services_test

import (
	"fmt"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services"
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
	// 1(起点) -> 2(子会社) -> 3(孫会社) の多段階資本関係を辿れること。
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "親会社"), nil)
	companyRepo.On("FindByID", uint(2)).Return(companyNode(2, "子会社"), nil)
	companyRepo.On("FindByID", uint(3)).Return(companyNode(3, "孫会社"), nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(2)).Return(nil, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(3)).Return(nil, nil)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{capitalRel(1, 2, "capital_subsidiary")}, nil)
	relRepo.On("GetRelationsByCompanyID", uint(2)).Return([]models.CompanyRelation{capitalRel(2, 3, "capital_affiliate")}, nil)
	relRepo.On("GetRelationsByCompanyID", uint(3)).Return([]models.CompanyRelation{}, nil)

	svc := services.NewCompanyRelationGraphService(companyRepo, relRepo)
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
	// 起点企業(2)の親会社(1)も辿れること。
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "親会社"), nil)
	companyRepo.On("FindByID", uint(2)).Return(companyNode(2, "起点企業"), nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(2)).Return(nil, nil)
	relRepo.On("GetRelationsByCompanyID", uint(2)).Return([]models.CompanyRelation{capitalRel(1, 2, "capital_subsidiary")}, nil)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{}, nil)

	svc := services.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(2)
	require.NoError(t, err)

	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.CapitalEdges, 1)
}

func TestCompanyRelationGraphService_BuildGraph_CycleDoesNotLoopForever(t *testing.T) {
	// 1 -> 2 -> 1 の循環関係があっても停止し、重複ノード・エッジを作らないこと。
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "企業A"), nil)
	companyRepo.On("FindByID", uint(2)).Return(companyNode(2, "企業B"), nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)
	relRepo.On("GetMarketInfoByCompanyID", uint(2)).Return(nil, nil)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{capitalRel(1, 2, "capital_affiliate")}, nil)
	relRepo.On("GetRelationsByCompanyID", uint(2)).Return([]models.CompanyRelation{
		capitalRel(1, 2, "capital_affiliate"),
		capitalRel(2, 1, "capital_affiliate"),
	}, nil)

	svc := services.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)
	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.CapitalEdges, 2)
}

func TestCompanyRelationGraphService_BuildGraph_NodeCapTruncates(t *testing.T) {
	// 起点企業直下に上限を超える子会社がぶら下がる場合、ノード数上限で打ち切られること。
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "親会社"), nil)

	childCount := services.RelationGraphMaxNodes + 10
	relations := make([]models.CompanyRelation, 0, childCount)
	for i := 0; i < childCount; i++ {
		childID := uint(100 + i)
		companyRepo.On("FindByID", childID).Return(companyNode(childID, fmt.Sprintf("子会社%d", i)), nil)
		relations = append(relations, capitalRel(1, childID, "capital_subsidiary"))
	}

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyID", mock.Anything).Return(nil, nil)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return(relations, nil)
	for i := 0; i < childCount; i++ {
		relRepo.On("GetRelationsByCompanyID", uint(100+i)).Return([]models.CompanyRelation{}, nil)
	}

	svc := services.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)

	assert.True(t, graph.Truncated)
	assert.LessOrEqual(t, len(graph.Nodes), services.RelationGraphMaxNodes)
}

func TestCompanyRelationGraphService_BuildGraph_BusinessRelationsOnlyDirect(t *testing.T) {
	// 取引関係は起点企業から直接分のみ。取引先(2)がさらに持つ取引関係(2->3)は含めない。
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindByID", uint(1)).Return(companyNode(1, "起点企業"), nil)
	companyRepo.On("FindByID", uint(2)).Return(companyNode(2, "取引先"), nil)

	relRepo := &relationRepoMock{}
	relRepo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)
	relRepo.On("GetRelationsByCompanyID", uint(1)).Return([]models.CompanyRelation{
		businessRel(1, 2, "business_partner", "主要取引先"),
	}, nil)

	svc := services.NewCompanyRelationGraphService(companyRepo, relRepo)
	graph, err := svc.BuildGraph(1)
	require.NoError(t, err)

	require.Len(t, graph.BusinessRelations, 1)
	assert.Equal(t, uint(2), graph.BusinessRelations[0].CompanyID)
	assert.Equal(t, "取引先", graph.BusinessRelations[0].Name)
	// 起点企業自身のみがグラフのノードとして扱われ、取引先はノード側には含まれない（資本関係専用）。
	assert.Len(t, graph.Nodes, 1)
	// GetRelationsByCompanyID(2) が呼ばれていないこと = 取引先の取引先までは辿っていないこと。
	relRepo.AssertNotCalled(t, "GetRelationsByCompanyID", uint(2))
}
