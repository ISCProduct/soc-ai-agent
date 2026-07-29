package companyfetch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsClearOrganizationName(t *testing.T) {
	assert.True(t, IsClearOrganizationName("デジタル庁"))
	assert.True(t, IsClearOrganizationName("厚生労働省"))
	assert.True(t, IsClearOrganizationName("株式会社パートナーA"))
	assert.True(t, IsClearOrganizationName("トヨタ自動車株式会社"))
	assert.True(t, IsClearOrganizationName("東京都"))

	assert.False(t, IsClearOrganizationName(""))
	assert.False(t, IsClearOrganizationName("不明"))
	assert.False(t, IsClearOrganizationName("その他"))
	assert.False(t, IsClearOrganizationName("株式会社"))
	assert.False(t, IsClearOrganizationName("共同企業体"))
	assert.False(t, IsClearOrganizationName("○○特定共同企業体"))
	assert.False(t, IsClearOrganizationName("主要取引先"))
	assert.False(t, IsClearOrganizationName("A"))
	assert.False(t, IsClearOrganizationName("ほか数社"))
}
