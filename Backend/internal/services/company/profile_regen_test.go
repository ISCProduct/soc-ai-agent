package company

import (
	"testing"

	"Backend/internal/models"
)

func profile(v [10]int) *models.CompanyWeightProfile {
	p := &models.CompanyWeightProfile{}
	for i, a := range profileAxes(p) {
		*a = v[i]
	}
	return p
}

func TestIsLegacySaturated(t *testing.T) {
	tests := []struct {
		name string
		p    *models.CompanyWeightProfile
		want bool
	}{
		{"nil", nil, false},
		{"相対評価（80以上は3項目まで、100は無い）", profile([10]int{82, 55, 62, 64, 35, 52, 60, 83, 78, 80}), false},
		{"100点が3軸", profile([10]int{100, 100, 100, 40, 35, 52, 60, 83, 78, 80}), false},
		{"100点が4軸（旧い生成の形）", profile([10]int{100, 100, 100, 100, 35, 52, 60, 83, 78, 80}), true},
		{"100点が6軸（開発DBに多い形）", profile([10]int{100, 100, 40, 40, 100, 100, 60, 90, 100, 100}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLegacySaturated(tt.p); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeasureProfileSpread(t *testing.T) {
	// 旧い生成: 6軸が全社 100、4軸だけばらつく
	legacy := []*models.CompanyWeightProfile{
		profile([10]int{100, 100, 20, 70, 100, 100, 30, 90, 100, 100}),
		profile([10]int{100, 100, 80, 30, 100, 100, 70, 20, 100, 100}),
		profile([10]int{100, 100, 50, 90, 100, 100, 10, 60, 100, 100}),
	}
	l := MeasureProfileSpread(legacy)
	if l.Companies != 3 || l.Saturated != 3 || l.Informative != 4 {
		t.Fatalf("旧い生成: %+v", l)
	}
	// 相対評価: どの軸もばらつく
	fresh := []*models.CompanyWeightProfile{
		profile([10]int{82, 30, 62, 64, 35, 52, 60, 83, 78, 80}),
		profile([10]int{40, 85, 20, 30, 80, 75, 45, 30, 35, 40}),
		profile([10]int{60, 50, 85, 80, 25, 30, 82, 55, 50, 82}),
	}
	f := MeasureProfileSpread(fresh)
	if f.Saturated != 0 || f.Informative <= l.Informative {
		t.Fatalf("相対評価: %+v（旧い生成より、ばらつく軸が多いはず）", f)
	}
	if empty := MeasureProfileSpread(nil); empty.Companies != 0 || empty.Informative != 0 {
		t.Fatalf("空: %+v", empty)
	}
}
