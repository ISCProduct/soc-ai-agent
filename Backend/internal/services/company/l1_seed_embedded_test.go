package company

import (
	"strings"
	"testing"

	l1catalog "Backend/config"
)

// 環境構築時に投入する L1 カタログ CSV が、読めて・重複せず・件数を保っていること。
func TestEmbeddedL1CatalogSeed(t *testing.T) {
	rows, err := ParseL1SeedCSV(strings.NewReader(l1catalog.SeedCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 700 {
		t.Fatalf("件数が少なすぎる: %d", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.Name] {
			t.Errorf("社名が重複: %s", r.Name)
		}
		seen[r.Name] = true
		if r.Segment != "core" && r.Segment != "sme_si" {
			t.Errorf("segment が不正: %s (%s)", r.Name, r.Segment)
		}
	}
}
