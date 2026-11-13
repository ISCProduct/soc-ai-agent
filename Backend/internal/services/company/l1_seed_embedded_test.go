package company

import (
	"strings"
	"testing"

	l1catalog "Backend/config"
)

// 環境構築時に投入する L1 カタログ CSV が、読めて・重複せず・件数を保っていること。
//
// 件数の下限は 350。社名キーワードで集めた候補のうち、l1enrich で実際に Claude
// に確認すると約半数は IT 企業でないと判定されて除外される（800 社を全件処理した
// 実測で 420 社が残った）。700 という当初の下限は、この除外前の未検証の件数を
// そのまま基準にしていた誤り。
func TestEmbeddedL1CatalogSeed(t *testing.T) {
	rows, err := ParseL1SeedCSV(strings.NewReader(l1catalog.SeedCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 350 {
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
