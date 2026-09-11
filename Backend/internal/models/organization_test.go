package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestOrganization_MarshalJSON_ContractDates(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name         string
		org          Organization
		wantContains string
		wantOmits    string
	}{
		{
			name:         "契約日ありはYYYY-MM-DD形式",
			org:          Organization{ID: 1, ContractStartDate: &start},
			wantContains: `"contract_start_date":"2026-04-01"`,
		},
		{
			name:      "契約日なしは省略される",
			org:       Organization{ID: 1},
			wantOmits: `contract_start_date`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.org)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(b)
			if tt.wantContains != "" && !strings.Contains(s, tt.wantContains) {
				t.Fatalf("expected %q to contain %q", s, tt.wantContains)
			}
			if tt.wantOmits != "" && strings.Contains(s, tt.wantOmits) {
				t.Fatalf("expected %q to omit %q", s, tt.wantOmits)
			}
		})
	}
}
