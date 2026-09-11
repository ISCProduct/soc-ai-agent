package company

import "testing"

func TestCapitalRelationType(t *testing.T) {
	pct := func(v float64) *float64 { return &v }
	tests := []struct {
		claimed string
		ratio   *float64
		want    string
	}{
		{"capital_subsidiary", pct(100), "capital_subsidiary"},
		{"capital_subsidiary", pct(5.02), "capital_affiliate"},
		{"capital_affiliate", pct(51), "capital_subsidiary"},
		{"business_partner", pct(49), "capital_affiliate"},
		{"capital_subsidiary", nil, "capital_subsidiary"},
		{"business_partner", nil, "business_partner"},
		{"", nil, "business_partner"},
	}
	for _, tt := range tests {
		got := capitalRelationType(tt.claimed, tt.ratio)
		if got != tt.want {
			t.Errorf("capitalRelationType(%q, %v) = %q, want %q", tt.claimed, tt.ratio, got, tt.want)
		}
	}
}

func TestParseCompanyRelationsResult_RatioOverridesBucket(t *testing.T) {
	raw := `{
	  "subsidiaries": [{"name": "いすゞ自動車株式会社", "ratio": 5.02, "description": "資本提携"}],
	  "affiliates": [{"name": "NECフィールディング株式会社", "ratio": 100, "description": "完全子会社"}],
	  "business_partners": [{"name": "デジタル庁", "description": "調達"}]
	}`
	got, err := parseCompanyRelationsResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Relations) != 3 {
		t.Fatalf("len=%d", len(got.Relations))
	}
	byName := map[string]RelationEntry{}
	for _, r := range got.Relations {
		byName[r.Name] = r
	}
	if byName["いすゞ自動車株式会社"].RelationType != "capital_affiliate" {
		t.Errorf("いすゞ type=%s", byName["いすゞ自動車株式会社"].RelationType)
	}
	if byName["NECフィールディング株式会社"].RelationType != "capital_subsidiary" {
		t.Errorf("フィールディング type=%s", byName["NECフィールディング株式会社"].RelationType)
	}
	if byName["デジタル庁"].RelationType != "business_partner" {
		t.Errorf("デジタル庁 type=%s", byName["デジタル庁"].RelationType)
	}
}
