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
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		raw     string
		want    *time.Time
		wantErr error
	}{
		{"空文字はnil", "", nil, nil},
		{"YYYY-MM-DDを解釈する", "2026-04-01", &want, nil},
		{"不正な形式はエラー", "2026/04/01", nil, ErrInvalidContractDate},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseContractDate(tt.raw)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil || !got.Equal(*tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestValidateContractDateRange(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	after := time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		start   *time.Time
		end     *time.Time
		wantErr error
	}{
		{"両方nilは有効", nil, nil, nil},
		{"開始日が終了日より前は有効", &start, &after, nil},
		{"終了日が開始日より前はエラー", &start, &before, ErrContractDateRange},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContractDateRange(tt.start, tt.end)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
