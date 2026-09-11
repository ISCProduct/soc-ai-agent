package util

import "strings"

// NormalizeSchoolName は学校名の表記ゆれ（空白・法人名プレフィックス）を除いて比較用に正規化する。
func NormalizeSchoolName(name string) string {
	s := strings.TrimSpace(name)
	for _, prefix := range []string{"学校法人", "株式会社", "（株）", "(株)"} {
		s = strings.TrimPrefix(s, prefix)
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "")
	return strings.TrimSpace(s)
}
