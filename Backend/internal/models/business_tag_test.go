package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBusinessTagNames(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "半角カナと全角英数をそろえる", in: []string{"ｸﾗｳﾄﾞ", "ＡＩ"}, want: []string{"クラウド", "AI"}},
		{name: "空白を整えて重複をまとめる", in: []string{" クラウド ", "クラウド", "業務　 システム", "  "}, want: []string{"クラウド", "業務 システム"}},
		{name: "幅の無い文字を除く", in: []string{"クラ\u200bウド", "\ufeffクラウド"}, want: []string{"クラウド"}},
		{name: "濁点の有無は別のタグ", in: []string{"ハ", "パ"}, want: []string{"ハ", "パ"}},
		{name: "空配列", in: []string{}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeBusinessTagNames(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeBusinessTagNames_Limits(t *testing.T) {
	_, err := NormalizeBusinessTagNames([]string{strings.Repeat("あ", MaxBusinessTagNameLength+1)})
	assert.Error(t, err)

	_, err = NormalizeBusinessTagNames([]string{strings.Repeat("あ", MaxBusinessTagNameLength)})
	assert.NoError(t, err)

	// 長さは NFKC の後で数える。「㍿」は「株式会社」(4文字)になるので 17 個で 68 文字
	_, err = NormalizeBusinessTagNames([]string{strings.Repeat("㍿", 16)})
	assert.NoError(t, err)
	_, err = NormalizeBusinessTagNames([]string{strings.Repeat("㍿", 17)})
	assert.Error(t, err)

	many := make([]string, MaxBusinessTagsPerCompany+1)
	for i := range many {
		many[i] = strings.Repeat("あ", i+1)
	}
	_, err = NormalizeBusinessTagNames(many)
	assert.Error(t, err)

	// 重複をまとめた後の件数で数える
	_, err = NormalizeBusinessTagNames(append(many[:MaxBusinessTagsPerCompany], many[0]))
	assert.NoError(t, err)
}
