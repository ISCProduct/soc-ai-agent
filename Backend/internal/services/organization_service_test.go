package services

import (
	"testing"

	"Backend/internal/models"
)

func TestOrganizationService_EnsureSameOrganization(t *testing.T) {
	svc := &OrganizationService{}
	if err := svc.EnsureSameOrganization(1, 1); err != nil {
		t.Fatalf("same org should pass: %v", err)
	}
	if err := svc.EnsureSameOrganization(1, 2); err != ErrCrossOrganization {
		t.Fatalf("expected ErrCrossOrganization, got %v", err)
	}
	if err := svc.EnsureSameOrganization(0, 1); err != ErrCrossOrganization {
		t.Fatalf("expected ErrCrossOrganization for zero actor, got %v", err)
	}
}

func TestValidOrgRole(t *testing.T) {
	cases := map[string]bool{
		models.OrgRoleOwner:  true,
		models.OrgRoleAdmin:  true,
		models.OrgRoleMember: true,
		"viewer":             false,
		"":                   false,
	}
	for role, want := range cases {
		if got := validOrgRole(role); got != want {
			t.Fatalf("role %q: got %v want %v", role, got, want)
		}
	}
}

func TestSlugPattern(t *testing.T) {
	ok := []string{"default", "acme-corp", "school1", "a"}
	for _, s := range ok {
		if !slugPattern.MatchString(s) {
			t.Fatalf("expected valid slug %q", s)
		}
	}
	ng := []string{"", "-bad", "Bad", "has_underscore", "has space"}
	for _, s := range ng {
		if slugPattern.MatchString(s) {
			t.Fatalf("expected invalid slug %q", s)
		}
	}
}
