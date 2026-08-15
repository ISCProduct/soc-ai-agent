package shared

import "strings"

// ContainsAny は s に terms のいずれかが(小文字化して)含まれるかを判定する。
func ContainsAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
