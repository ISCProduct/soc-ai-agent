package organization

import (
	"errors"
	"testing"
	"time"

	"Backend/internal/models"
)

func TestOrganizationService_EnsureSameOrganization(t *testing.T) {
	svc := &OrganizationService{}
	tests := []struct {
		name          string
		actorOrgID    uint
		resourceOrgID uint
		wantErr       error
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

func TestValidOrgPlan(t *testing.T) {
	cases := []struct {
		plan string
		want bool
	}{
		{models.OrgPlanFree, true},
		{models.OrgPlanStandard, true},
		{models.OrgPlanPro, true},
		{"enterprise", false},
		{"", false},
	}
	for _, tt := range cases {
		t.Run(tt.plan, func(t *testing.T) {
			if got := validOrgPlan(tt.plan); got != tt.want {
				t.Fatalf("plan %q: got %v want %v", tt.plan, got, tt.want)
			}
		})
	}
}

func TestParseContractDate(t *testing.T) {
	t.Run("空文字はnil", func(t *testing.T) {
		got, err := parseContractDate("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("YYYY-MM-DDを解釈する", func(t *testing.T) {
		got, err := parseContractDate("2026-04-01")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || !got.Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("不正な形式はエラー", func(t *testing.T) {
		if _, err := parseContractDate("2026/04/01"); !errors.Is(err, ErrInvalidContractDate) {
			t.Fatalf("got %v want ErrInvalidContractDate", err)
		}
	})
}

func TestValidateContractDateRange(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	after := time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC)

	if err := validateContractDateRange(nil, nil); err != nil {
		t.Fatalf("both nil should be valid: %v", err)
	}
	if err := validateContractDateRange(&start, &after); err != nil {
		t.Fatalf("start before end should be valid: %v", err)
	}
	if err := validateContractDateRange(&start, &before); !errors.Is(err, ErrContractDateRange) {
		t.Fatalf("got %v want ErrContractDateRange", err)
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
