package repositories

import (
	"os"
	"strings"
	"testing"

	"Backend/internal/models"
)

func TestFlywheelPassedSQL_NoLegacyHardcodedIN(t *testing.T) {
	files := []string{
		"score_validation_repository.go",
		"profile_recalculation_repository.go",
	}
	banned := "'document_passed','interview','offered','accepted'"
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), banned) {
			t.Errorf("%s に旧通過判定リテラルが残っている", f)
		}
	}
}

func TestScoreValidationSQL_UsesSharedHelper(t *testing.T) {
	src, err := os.ReadFile("score_validation_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if !strings.Contains(body, "FlywheelPassedStatusSQLIn") {
		t.Error("score_validation_repository.go が FlywheelPassedStatusSQLIn を使っていない")
	}
	if strings.Count(body, "flywheelPassedSQL(") < 4 {
		t.Error("通過判定 SQL が共有ヘルパー経由でない")
	}
}

func TestProfileRecalc_UsesSharedFilter(t *testing.T) {
	src, err := os.ReadFile("profile_recalculation_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "FlywheelPassedStatusFilter") {
		t.Error("profile_recalculation_repository.go が FlywheelPassedStatusFilter を使っていない")
	}
}

func TestFlywheelPassedSQLIn_ContainsCurrentInterviewStatus(t *testing.T) {
	got := models.FlywheelPassedStatusSQLIn()
	if !strings.Contains(got, "'interview_in_progress'") {
		t.Errorf("SQL IN に interview_in_progress が無い: %s", got)
	}
	if strings.Contains(got, "'withdrawn'") || strings.Contains(got, "'rejected'") {
		t.Errorf("SQL IN に非通過ステータスがある: %s", got)
	}
}
