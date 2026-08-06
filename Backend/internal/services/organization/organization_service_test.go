package organization

import (
	"errors"
	"testing"

	"Backend/internal/models"
)

func TestOrganizationService_EnsureSameOrganization(t *testing.T) {
	svc := &OrganizationService{}
	tests := []struct {
		name         string
		actorOrgID   uint
		resourceOrgID uint
		wantErr      error
	}{
		{name: "same org", actorOrgID: 1, resourceOrgID: 1, wantErr: nil},
		{name: "different org", actorOrgID: 1, resourceOrgID: 2, wantErr: ErrCrossOrganization},
		{name: "zero actor", actorOrgID: 0, resourceOrgID: 1, wantErr: ErrCrossOrganization},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.EnsureSameOrganization(tt.actorOrgID, tt.resourceOrgID)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidOrgRole(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{models.OrgRoleOwner, true},
		{models.OrgRoleAdmin, true},
		{models.OrgRoleMember, true},
		{"viewer", false},
		{"", false},
	}
	for _, tt := range cases {
		t.Run(tt.role, func(t *testing.T) {
			if got := validOrgRole(tt.role); got != tt.want {
				t.Fatalf("role %q: got %v want %v", tt.role, got, tt.want)
			}
		})
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
