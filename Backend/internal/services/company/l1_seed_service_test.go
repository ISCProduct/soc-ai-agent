package company

import (
	"strings"
	"testing"
)

func TestParseL1SeedCSV_WithHeader(t *testing.T) {
	csv := "name,industry,segment,website_url,publish\n" +
		"株式会社コアテック,情報通信,core,https://example.com,true\n" +
		"中小SI株式会社,,sme_si,,1\n" +
		"# comment,,,,\n"
	rows, err := ParseL1SeedCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2", len(rows))
	}
	if rows[0].Segment != "core" || !rows[0].Publish {
		t.Fatalf("row0=%+v", rows[0])
	}
	if rows[1].Segment != "sme_si" || !rows[1].Publish {
		t.Fatalf("row1=%+v", rows[1])
	}
}

func TestNormalizeL1Segment(t *testing.T) {
	if normalizeL1Segment("SI") != "sme_si" {
		t.Fatal(normalizeL1Segment("SI"))
	}
}
