package company

// 企業プロファイルの識別力のテスト（#1331）。
// 実行: cd Backend && go test ./internal/services/company/ -run Profile -v
//
// マッチ度は 100 - |学生スコア - 企業重視度| の平均で決まる。
// ある軸が全社で同じ値なら、その軸は全企業に同じ定数を足すだけになり、
// 並び順に一切寄与しない。実測で teamwork 平均99/標準偏差8.1、
// detail 平均99/標準偏差4.5 という状態になっていた。

import (
	"testing"

	"Backend/internal/models"
)

func profileOf(v ...int) *models.CompanyWeightProfile {
	p := &models.CompanyWeightProfile{}
	axes := profileAxes(p)
	for i := range axes {
		*axes[i] = v[i%len(v)]
	}
	return p
}

func spreadOf(p *models.CompanyWeightProfile) int {
	axes := profileAxes(p)
	minV, maxV := *axes[0], *axes[0]
	for _, a := range axes {
		if *a < minV {
			minV = *a
		}
		if *a > maxV {
			maxV = *a
		}
	}
	return maxV - minV
}

func TestProfileAxes_10軸すべてを参照している(t *testing.T) {
	// 軸を足したときに走査から漏れると、その軸だけ正規化されない。
	p := &models.CompanyWeightProfile{}
	if got := len(profileAxes(p)); got != 10 {
		t.Errorf("軸の数が違う: %d（正典は10軸）", got)
	}

	// 参照であること（書き戻しが効く）
	axes := profileAxes(p)
	*axes[0] = 77
	if p.TechnicalOrientation != 77 {
		t.Error("参照になっていない。正規化の結果が書き戻らない")
	}
}

func TestNormalizeProfileSpread_狭い幅を広げる(t *testing.T) {
	// 実測に近い形：ほぼ全軸が上限側に張り付いている
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 95, TeamworkOrientation: 99, LeadershipOrientation: 96,
		CreativityOrientation: 97, StabilityOrientation: 98, GrowthOrientation: 94,
		WorkLifeBalance: 99, ChallengeSeeking: 96, DetailOrientation: 99,
		CommunicationSkill: 97,
	}
	before := spreadOf(p)

	if !normalizeProfileSpread(p) {
		t.Fatal("正規化されていない")
	}

	after := spreadOf(p)
	if after < minProfileSpread {
		t.Errorf("幅が足りない: %d → %d（目標 %d 以上）", before, after, minProfileSpread)
	}
}

func TestNormalizeProfileSpread_順序を保つ(t *testing.T) {
	// LLM が付けた順序は情報なので壊さない。壊すと存在しない差を捏造することになる。
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 90, TeamworkOrientation: 99, LeadershipOrientation: 92,
		CreativityOrientation: 95, StabilityOrientation: 97, GrowthOrientation: 91,
		WorkLifeBalance: 98, ChallengeSeeking: 93, DetailOrientation: 96,
		CommunicationSkill: 94,
	}
	beforeAxes := profileAxes(p)
	before := make([]int, len(beforeAxes))
	for i, a := range beforeAxes {
		before[i] = *a
	}

	normalizeProfileSpread(p)

	after := profileAxes(p)
	for i := range before {
		for j := range before {
			if before[i] < before[j] && *after[i] > *after[j] {
				t.Errorf("順序が入れ替わった: 軸%d(%d→%d) 軸%d(%d→%d)",
					i, before[i], *after[i], j, before[j], *after[j])
			}
		}
	}
}

func TestNormalizeProfileSpread_十分な幅なら触らない(t *testing.T) {
	// 既に識別力があるものを引き伸ばすと、逆に極端な値になる。
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 90, TeamworkOrientation: 65, LeadershipOrientation: 55,
		CreativityOrientation: 70, StabilityOrientation: 35, GrowthOrientation: 85,
		WorkLifeBalance: 60, ChallengeSeeking: 75, DetailOrientation: 70,
		CommunicationSkill: 55,
	}
	want := *p

	if normalizeProfileSpread(p) {
		t.Error("十分な幅があるのに正規化された")
	}
	if *p != want {
		t.Errorf("値が変わっている: %+v", p)
	}
}

func TestNormalizeProfileSpread_全軸同値は引き伸ばせない(t *testing.T) {
	// 順序が無いので広げようがない。捏造せず、呼び出し元の判断に委ねる。
	p := profileOf(99)

	if normalizeProfileSpread(p) {
		t.Error("全軸同値を引き伸ばしてはいけない（順序が無い）")
	}
	if !profileIsDegenerate(p) {
		t.Error("識別力なしと判定されるべき")
	}
}

func TestNormalizeProfileSpread_0から100をはみ出さない(t *testing.T) {
	// 上限側に寄ったものを広げると 100 を超えうる。
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 97, TeamworkOrientation: 100, LeadershipOrientation: 98,
		CreativityOrientation: 99, StabilityOrientation: 100, GrowthOrientation: 97,
		WorkLifeBalance: 100, ChallengeSeeking: 98, DetailOrientation: 99,
		CommunicationSkill: 98,
	}
	normalizeProfileSpread(p)

	for i, a := range profileAxes(p) {
		if *a < 0 || *a > 100 {
			t.Errorf("軸%d が範囲外: %d", i, *a)
		}
	}
}

func TestProfileIsDegenerate(t *testing.T) {
	tests := []struct {
		name string
		p    *models.CompanyWeightProfile
		want bool
	}{
		{"全軸同値", profileOf(50), true},
		{"幅9（順位をほぼ動かさない）", profileOf(50, 59), true},
		{"幅10", profileOf(50, 60), false},
		{"nil", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profileIsDegenerate(tt.p); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// 正規化が実際にマッチ度の差を生むことを確認する（#1331）。
//
// マッチ度は 100 - |学生スコア - 企業重視度|。全社が同じ値の軸は
// 全企業に同じ定数を足すだけで並び順に寄与しない。
// 正規化後に「企業ごとの差」が生まれていなければ意味がない。
func TestNormalizeProfileSpread_企業間の差が生まれる(t *testing.T) {
	match := func(user, company int) int {
		d := user - company
		if d < 0 {
			d = -d
		}
		if v := 100 - d; v > 0 {
			return v
		}
		return 0
	}

	// 実測に近い2社。どちらも上限側に張り付いており、技術志向だけ僅かに違う。
	it := &models.CompanyWeightProfile{
		TechnicalOrientation: 99, TeamworkOrientation: 98, LeadershipOrientation: 96,
		CreativityOrientation: 97, StabilityOrientation: 94, GrowthOrientation: 98,
		WorkLifeBalance: 95, ChallengeSeeking: 97, DetailOrientation: 96,
		CommunicationSkill: 95,
	}
	fin := &models.CompanyWeightProfile{
		TechnicalOrientation: 94, TeamworkOrientation: 98, LeadershipOrientation: 96,
		CreativityOrientation: 95, StabilityOrientation: 99, GrowthOrientation: 95,
		WorkLifeBalance: 96, ChallengeSeeking: 94, DetailOrientation: 99,
		CommunicationSkill: 97,
	}

	const techStudent = 90 // 技術志向の高い学生

	beforeDiff := match(techStudent, it.TechnicalOrientation) - match(techStudent, fin.TechnicalOrientation)

	normalizeProfileSpread(it)
	normalizeProfileSpread(fin)

	afterDiff := match(techStudent, it.TechnicalOrientation) - match(techStudent, fin.TechnicalOrientation)

	if afterDiff <= beforeDiff {
		t.Errorf("正規化後も企業間の差が広がっていない: %d → %d", beforeDiff, afterDiff)
	}
	// 技術志向の高い学生には、技術志向の高い企業のほうが高く出るべき。
	if match(techStudent, it.TechnicalOrientation) <= match(techStudent, fin.TechnicalOrientation) {
		t.Errorf("技術志向の学生に技術志向の企業が勝っていない: IT=%d FIN=%d",
			match(techStudent, it.TechnicalOrientation), match(techStudent, fin.TechnicalOrientation))
	}
}
