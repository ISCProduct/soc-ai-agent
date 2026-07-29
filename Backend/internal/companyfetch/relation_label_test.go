package companyfetch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRelationDescription(t *testing.T) {
	assert.Equal(t, "子会社", DefaultRelationDescription("capital_subsidiary"))
	assert.Equal(t, "主要取引先", DefaultRelationDescription("business_partner"))
	assert.Equal(t, "調達・契約", DefaultRelationDescription("business_procurement"))
}

func TestIsSourceTagDescription(t *testing.T) {
	assert.True(t, IsSourceTagDescription(""))
	assert.True(t, IsSourceTagDescription("web_search:sky株式会社"))
	assert.True(t, IsSourceTagDescription("llm_web_search:テスト"))
	assert.False(t, IsSourceTagDescription("クラウド基盤の共同開発"))
	assert.False(t, IsSourceTagDescription("調達契約 (2024-01-01)"))
}

func TestSanitizeRelationDescription(t *testing.T) {
	assert.Equal(t, "", SanitizeRelationDescription("web_search:sky株式会社"))
	assert.Equal(t, "", SanitizeRelationDescription("主要取引先"))
	assert.Equal(t, "", SanitizeRelationDescription("取引先"))
	assert.Equal(t, "決済代行", SanitizeRelationDescription("決済代行"))
	assert.Equal(t, "クラウド基盤の共同開発", SanitizeRelationDescription("  クラウド基盤の共同開発  "))
}

func TestNormalizeRelationDescription(t *testing.T) {
	assert.Equal(t, "主要取引先", NormalizeRelationDescription("web_search:sky株式会社", "business_partner"))
	assert.Equal(t, "主要取引先", NormalizeRelationDescription("主要取引先", "business_partner"))
	assert.Equal(t, "決済代行", NormalizeRelationDescription("決済代行", "business_partner"))
}
