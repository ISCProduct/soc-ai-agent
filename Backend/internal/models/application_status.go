package models

import "strings"

// FlywheelPassedStatuses は書類通過以降の現行ステータス。
// ValidStatuses（application パッケージ）の部分集合。旧 interview は含めない。
var FlywheelPassedStatuses = []string{
	"document_passed",
	"interview_scheduled",
	"interview_in_progress",
	"offered",
	"accepted",
}

// flywheelPassedLegacyAliases は旧リテラル。集計の読み取り IN にだけ載せる。
var flywheelPassedLegacyAliases = []string{"interview"}

// FlywheelPassedStatusFilter はフライホイール集計の IN 対象（現行 + 旧 interview）。
func FlywheelPassedStatusFilter() []string {
	out := make([]string, 0, len(FlywheelPassedStatuses)+len(flywheelPassedLegacyAliases))
	out = append(out, FlywheelPassedStatuses...)
	return append(out, flywheelPassedLegacyAliases...)
}

// FlywheelPassedStatusSQLIn は Raw SQL の IN (...) 用リテラル。値は定数のみ。
func FlywheelPassedStatusSQLIn() string {
	return "'" + strings.Join(FlywheelPassedStatusFilter(), "','") + "'"
}
