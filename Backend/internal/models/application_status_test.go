package models

import (
	"slices"
	"strings"
	"testing"
)

func TestFlywheelPassedStatuses_IncludesDocumentPassedOnward(t *testing.T) {
	want := []string{
		"document_passed",
		"interview_scheduled",
		"interview_in_progress",
		"offered",
		"accepted",
	}
	for _, s := range want {
		if !slices.Contains(FlywheelPassedStatuses, s) {
			t.Errorf("FlywheelPassedStatuses に %s が含まれていない", s)
		}
	}
}

func TestFlywheelPassedStatuses_ExcludesNonPass(t *testing.T) {
	notPassed := []string{
		"not_applied", "applied", "document_screening",
		"withdrawn", "rejected", "declined", "interview",
	}
	for _, s := range notPassed {
		if slices.Contains(FlywheelPassedStatuses, s) {
			t.Errorf("FlywheelPassedStatuses に非通過 %s が含まれている", s)
		}
	}
}

func TestFlywheelPassedStatusFilter_IncludesLegacyInterview(t *testing.T) {
	filter := FlywheelPassedStatusFilter()
	if !slices.Contains(filter, "interview") {
		t.Error("旧 interview が読み取りフィルタに含まれていない")
	}
	if !slices.Contains(filter, "interview_in_progress") {
		t.Error("interview_in_progress が読み取りフィルタに含まれていない")
	}
	for _, s := range []string{"withdrawn", "rejected", "declined"} {
		if slices.Contains(filter, s) {
			t.Errorf("フィルタに非通過 %s が含まれている", s)
		}
	}
}

func TestFlywheelPassedStatusSQLIn_MatchesFilter(t *testing.T) {
	got := FlywheelPassedStatusSQLIn()
	for _, s := range FlywheelPassedStatusFilter() {
		if !strings.Contains(got, "'"+s+"'") {
			t.Errorf("SQL IN に '%s' がない: %s", s, got)
		}
	}
	for _, s := range []string{"withdrawn", "rejected", "declined"} {
		if strings.Contains(got, "'"+s+"'") {
			t.Errorf("SQL IN に非通過 '%s' がある: %s", s, got)
		}
	}
}
