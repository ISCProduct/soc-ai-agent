package models

import (
	"fmt"
	"strings"
)

const (
	EmployeeCountBasisConsolidated = "consolidated"
	EmployeeCountBasisStandalone   = "standalone"
)

// NormalizeEmployeeCountBasis は連結/単体の表記を保存用の値へ揃える。
func NormalizeEmployeeCountBasis(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case EmployeeCountBasisConsolidated, "連結", "group":
		return EmployeeCountBasisConsolidated
	case EmployeeCountBasisStandalone, "単体", "単独":
		return EmployeeCountBasisStandalone
	default:
		return ""
	}
}

// FormatEmployeeCount は表示用。定義が無い既存データは人数だけ返す。
func FormatEmployeeCount(n int, basis string) string {
	if n <= 0 {
		return ""
	}
	switch NormalizeEmployeeCountBasis(basis) {
	case EmployeeCountBasisConsolidated:
		return fmt.Sprintf("%d名（連結）", n)
	case EmployeeCountBasisStandalone:
		return fmt.Sprintf("%d名（単体）", n)
	default:
		return fmt.Sprintf("%d名", n)
	}
}
