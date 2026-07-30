package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeRelationsResult_FillsTransactionDescription(t *testing.T) {
	existing := []RelationEntry{
		{Name: "取引先C", RelationType: "business_partner", Description: ""},
		{Name: "子会社A", RelationType: "capital_subsidiary", Description: "主要取引先"}, // 種別ラベルは sanitize で空扱い
	}
	ai := &CompanyRelationsResult{
		Relations: []RelationEntry{
			{Name: "取引先C", RelationType: "business_partner", Description: "決済代行"},
			{Name: "子会社A", RelationType: "capital_subsidiary", Description: "完全子会社"},
			{Name: "新規D", RelationType: "business_partner", Description: "クラウド基盤の共同開発"},
		},
		MarketInfo: &CompanyMarketInfoResult{IsListed: true, MarketType: "prime", StockCode: "4755"},
	}

	out := mergeRelationsResult(existing, nil, ai)
	require.Len(t, out.Relations, 3)

	byName := map[string]RelationEntry{}
	for _, r := range out.Relations {
		byName[r.Name] = r
	}
	assert.Equal(t, "決済代行", byName["取引先C"].Description)
	assert.Equal(t, "完全子会社", byName["子会社A"].Description)
	assert.Equal(t, "クラウド基盤の共同開発", byName["新規D"].Description)
	assert.Equal(t, "4755", out.MarketInfo.StockCode)
}

func TestRelationsNeedDescriptionEnrichment(t *testing.T) {
	assert.False(t, relationsNeedDescriptionEnrichment(nil))
	assert.False(t, relationsNeedDescriptionEnrichment([]RelationEntry{
		{Name: "A", RelationType: "business_partner", Description: "決済代行"},
	}))
	assert.False(t, relationsNeedDescriptionEnrichment([]RelationEntry{
		{Name: "子会社A", RelationType: "capital_subsidiary", Description: ""},
	}))
	assert.True(t, relationsNeedDescriptionEnrichment([]RelationEntry{
		{Name: "A", RelationType: "business_partner", Description: "決済代行"},
		{Name: "B", RelationType: "business_partner", Description: ""},
	}))
	assert.True(t, relationsNeedDescriptionEnrichment([]RelationEntry{
		{Name: "B", RelationType: "business_partner", Description: "主要取引先"},
	}))
}
