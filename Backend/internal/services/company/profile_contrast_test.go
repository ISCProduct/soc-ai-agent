package company

import (
	"Backend/internal/models"
	"testing"
)

func TestProfileHasContrast(t *testing.T) {
	flat := &models.CompanyWeightProfile{
		TechnicalOrientation: 55, TeamworkOrientation: 50, LeadershipOrientation: 48,
		CreativityOrientation: 52, StabilityOrientation: 50, GrowthOrientation: 55,
		WorkLifeBalance: 45, ChallengeSeeking: 50, DetailOrientation: 52, CommunicationSkill: 48,
	}
	if profileHasContrast(flat) {
		t.Fatal("mid cluster should lack contrast")
	}

	sharp := &models.CompanyWeightProfile{
		TechnicalOrientation: 85, TeamworkOrientation: 75, LeadershipOrientation: 30,
		CreativityOrientation: 80, StabilityOrientation: 25, GrowthOrientation: 70,
		WorkLifeBalance: 40, ChallengeSeeking: 72, DetailOrientation: 35, CommunicationSkill: 55,
	}
	if !profileHasContrast(sharp) {
		t.Fatal("polarized profile should have contrast")
	}
}

func TestEnsureProfileContrast_UpdatesFlat(t *testing.T) {
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 55, TeamworkOrientation: 52, LeadershipOrientation: 48,
		CreativityOrientation: 51, StabilityOrientation: 50, GrowthOrientation: 53,
		WorkLifeBalance: 49, ChallengeSeeking: 54, DetailOrientation: 47, CommunicationSkill: 50,
	}
	if !EnsureProfileContrast(p) {
		t.Fatal("expected update")
	}
	if !profileHasContrast(p) {
		t.Fatalf("still flat after force poles: %+v", profileAxisValues(p))
	}
	if EnsureProfileContrast(p) {
		t.Fatal("second call should be no-op")
	}
}
