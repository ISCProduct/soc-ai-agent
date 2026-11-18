package matching

import "testing"

// 集計コマンド（cmd/profile-distribution）の数字の元。合成の小さな分布で、分布と効果の集計が正しいことを固定する。
func TestBuildFitReport(t *testing.T) {
	companies := saturatedCompanies(60, 3)
	sessions := syntheticPersonas(8, 5)
	r := BuildFitReport(companies, sessions, 10)

	if r.Companies != 60 || r.Sessions != 8 {
		t.Fatalf("件数: %+v", r)
	}
	// 100 付近に張り付いた軸（技術・チームワーク・安定・成長・細部・コミュニケーション）は、ばらつきのある軸ではない
	for _, i := range []int{0, 1, 4, 5, 8, 9} {
		if r.Axes[i].Informative {
			t.Errorf("%s は対象外のはず: %+v", r.Axes[i].Name, r.Axes[i])
		}
	}
	for _, i := range []int{2, 3, 6, 7} {
		if !r.Axes[i].Informative {
			t.Errorf("%s は対象のはず: %+v", r.Axes[i].Name, r.Axes[i])
		}
	}
	if r.CompaniesGE6At100 == 0 {
		t.Errorf("100点が多い企業が数えられていない")
	}
	if r.TopOnRef > r.TopOffRef || r.AllRef < 0 || r.AllRef > 1 {
		t.Errorf("効果の集計が不自然: %+v", r)
	}
	// 評価済みのセッションが無いときは、分布だけを返す
	if empty := BuildFitReport(companies, nil, 10); empty.Sessions != 0 || empty.TopOn1 != 0 || empty.Axes[2].Std == 0 {
		t.Errorf("セッション無し: %+v", empty)
	}
}
