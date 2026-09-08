package models

import (
	"os"
	"regexp"
	"testing"

	"Backend/domain/valueobject"
)

// TestSeedCategories_AreCanonical は、シードデータのカテゴリ名が
// 正典10種だけで構成されていることを検証する（#929）。
//
// シードが正典外の値を入れると、それがそのまま user_weight_scores へ流れ、
// マッチングから引かれない死んだスコアになる。実際に seed.go には
// 「分析思考」「問題解決力」など正典に無い値が混在していた。
//
// ソースを正規表現で走査するのは、シード関数が *gorm.DB を要求するため
// 実行せずに中身だけ検証したいから。
func TestSeedCategories_AreCanonical(t *testing.T) {
	canonical := map[string]bool{}
	for _, c := range valueobject.AllWeightCategories() {
		canonical[string(c)] = true
	}

	files := []string{
		"seed.go",
		"seed_predefined_questions.go",
		"../../cmd/seed/main.go",
	}
	// Category: "..." / WeightCategory: "..." の値だけを拾う。
	pat := regexp.MustCompile(`\b(?:Weight)?Category:\s*"([^"]+)"`)

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range pat.FindAllStringSubmatch(string(src), -1) {
			if !canonical[m[1]] {
				t.Errorf("%s: 正典外のカテゴリ %q がシードされている。"+
					"domain/valueobject/match.go の10種に揃えること", f, m[1])
			}
		}
	}
}
