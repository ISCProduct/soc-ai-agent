package valueobject

import "testing"

// TestNormalizeWeightCategory は表記揺れが正典へ寄ること、
// 未知の値が弾かれることを検証する（#929）。
func TestNormalizeWeightCategory(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want WeightCategory
		ok   bool
	}{
		{name: "正典はそのまま通る", raw: "チームワーク志向", want: CategoryTeamwork, ok: true},
		{name: "志向サフィックスの欠落を補う", raw: "チームワーク", want: CategoryTeamwork, ok: true},
		{name: "リーダーシップ", raw: "リーダーシップ", want: CategoryLeadership, ok: true},
		{name: "創造性", raw: "創造性", want: CategoryCreativity, ok: true},
		{name: "能力と力の揺れ", raw: "コミュニケーション能力", want: CategoryCommunication, ok: true},
		{name: "質問体系の別名(創造性・発想力)", raw: "創造性・発想力", want: CategoryCreativity, ok: true},
		{name: "質問体系の別名(問題解決力)", raw: "問題解決力", want: CategoryTechnical, ok: true},
		{name: "質問体系の別名(分析思考)", raw: "分析思考", want: CategoryTechnical, ok: true},
		{name: "質問体系の別名(計画性・実行力)", raw: "計画性・実行力", want: CategoryDetail, ok: true},
		{name: "質問体系の別名(ストレス耐性)", raw: "ストレス耐性・粘り強さ", want: CategoryChallenge, ok: true},
		{name: "前後の空白を無視する", raw: "  成長志向 ", want: CategoryGrowth, ok: true},
		{name: "空文字は弾く", raw: "", ok: false},
		{name: "未知の値は弾く", raw: "ぜんぜん違うカテゴリ", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeWeightCategory(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (raw=%q)", ok, tt.ok, tt.raw)
			}
			if ok && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// 正典10種はすべて NormalizeWeightCategory を素通りすること。
// 正典を増やしたのに別名表の整備を忘れる、といった片手落ちを防ぐ。
func TestNormalizeWeightCategory_AllCanonicalPassThrough(t *testing.T) {
	all := AllWeightCategories()
	if len(all) != 10 {
		t.Fatalf("正典は10種のはず: %d", len(all))
	}
	for _, c := range all {
		got, ok := NormalizeWeightCategory(string(c))
		if !ok || got != c {
			t.Errorf("%q が素通りしない: got=%q ok=%v", c, got, ok)
		}
	}
}

// 別名表の写像先はすべて正典であること。
// 別名を足すときに正典外の値を書いてしまう事故を防ぐ。
func TestWeightCategoryAliases_MapToCanonical(t *testing.T) {
	canonical := map[WeightCategory]bool{}
	for _, c := range AllWeightCategories() {
		canonical[c] = true
	}
	for alias, target := range weightCategoryAliases {
		if !canonical[target] {
			t.Errorf("別名 %q の写像先 %q が正典に無い", alias, target)
		}
	}
}

func TestParseWeightCategory_Error(t *testing.T) {
	if _, err := ParseWeightCategory("未知"); err == nil {
		t.Fatal("未知のカテゴリでエラーにならない")
	}
	if _, err := ParseWeightCategory("チームワーク"); err != nil {
		t.Fatalf("別名で失敗した: %v", err)
	}
}
