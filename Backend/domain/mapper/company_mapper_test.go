package mapper

import (
	"testing"

	"Backend/internal/models"
)

func TestUserCompanyMatchToEntity_JobTitle(t *testing.T) {
	titled := UserCompanyMatchToEntity(&models.UserCompanyMatch{
		JobPosition: &models.CompanyJobPosition{ID: 3, Title: "生産管理"},
	})
	if titled.JobTitle != "生産管理" {
		t.Fatalf("JobTitle = %q, want 生産管理", titled.JobTitle)
	}

	empty := UserCompanyMatchToEntity(&models.UserCompanyMatch{})
	if empty.JobTitle != "" {
		t.Fatalf("求人が無いマッチの職種は空。got %q", empty.JobTitle)
	}

	// 表示用の職種名を保存側へ戻すと、求人の紐づけを上書きしうる。
	back := UserCompanyMatchFromEntity(titled)
	if back.JobPosition != nil || back.JobPositionID != nil {
		t.Fatal("職種名は表示用で、保存側へ戻さない")
	}
}
