package chat

// フェーズごとの出題対象が、正典10軸を取りこぼしていないことのテスト（#1333）。
// 実行: cd Backend && go test ./internal/services/chat/ -run PhaseCategories -v
//
// どのフェーズにも入っていない軸は永久に出題されない。マッチングは企業側の
// 10軸と突き合わせるので、覆われていない軸のぶんだけ情報が捨てられる。
// 「安定志向」は全セッションで一度も測れていなかった。

import (
	"testing"

	"Backend/domain/valueobject"
)

func TestPhaseCategories_全フェーズの和集合が正典10軸を覆う(t *testing.T) {
	covered := map[string][]string{} // 軸 -> それを含むフェーズ
	for phase, cats := range phaseCategories {
		for _, c := range cats {
			covered[c] = append(covered[c], phase)
		}
	}

	for _, c := range valueobject.AllWeightCategories() {
		if len(covered[string(c)]) == 0 {
			t.Errorf("どのフェーズにも含まれない軸がある: %s（永久に測れない）", c)
		}
	}
}

func TestPhaseCategories_正典に無い軸を含まない(t *testing.T) {
	// 表記ゆれを入れると、そのフェーズだけ出題対象が実質減る。
	for phase, cats := range phaseCategories {
		for _, c := range cats {
			if _, ok := valueobject.NormalizeWeightCategory(c); !ok {
				t.Errorf("%s に正典に無いカテゴリ名: %q", phase, c)
			}
		}
	}
}

func TestPhaseCategories_各フェーズが空でない(t *testing.T) {
	// 空だと allowedCategories が全軸のままになり、フェーズの意味が消える。
	for phase, cats := range phaseCategories {
		if len(cats) == 0 {
			t.Errorf("出題対象が空のフェーズ: %s", phase)
		}
	}
}
