package services

import (
	"Backend/internal/models"
	"testing"
	"time"
)

func TestMissingNeedsFromCompany(t *testing.T) {
	now := time.Now()
	stale := now.Add(-200 * 24 * time.Hour)

	fresh := &models.Company{
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, jobs, tech, rel := MissingNeedsFromCompany(fresh)
	if info || jobs || tech || rel {
		t.Fatalf("fresh company should need nothing, got info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	empty := &models.Company{}
	info, jobs, tech, rel = MissingNeedsFromCompany(empty)
	if !info || !jobs || !tech || !rel {
		t.Fatalf("empty company should need all, got info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	staleInfo := &models.Company{
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &stale,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, jobs, tech, rel = MissingNeedsFromCompany(staleInfo)
	if !info || jobs || tech || rel {
		t.Fatalf("stale info only: info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	emptyTech := &models.Company{
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          "[]",
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	_, _, tech, _ = MissingNeedsFromCompany(emptyTech)
	if !tech {
		t.Fatal("empty tech payload [] should need tech")
	}

	emptyInfoDespiteStamp := &models.Company{
		Description:        "",
		WebsiteURL:         "",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, _, _, _ = MissingNeedsFromCompany(emptyInfoDespiteStamp)
	if !info {
		t.Fatal("fresh InfoFetchedAt with empty description/url should still need info")
	}
}
