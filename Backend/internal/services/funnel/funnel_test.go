package funnel

import (
	"math"
	"testing"
)

// #2039: 機能横断のコンバージョンファネル。
func TestBuild(t *testing.T) {
	students := []uint{1, 2, 3, 4}
	reached := map[string][]uint{
		StageDiagnosis: {1, 2, 3, 1}, // 重複は1人として数える
		StageInterview: {1, 2},
		StageES:        {2, 4},     // 4 は面接を経ずに ES へ
		StageApplied:   {2, 4, 99}, // 99 は対象外（退会者など）なので数えない
		StageOffered:   {4},
	}
	got := Build(students, reached)
	if got.Students != 4 || len(got.Stages) != len(Stages) {
		t.Fatalf("students=%d stages=%d", got.Students, len(got.Stages))
	}
	want := []StageResult{
		{Stage: StageDiagnosis, Reached: 3, RateOfStudents: 0.75, ContinuedFromPrevious: 3, ContinuationRate: 0.75},
		{Stage: StageInterview, Reached: 2, RateOfStudents: 0.5, ContinuedFromPrevious: 2, ContinuationRate: 2.0 / 3},
		{Stage: StageES, Reached: 2, RateOfStudents: 0.5, ContinuedFromPrevious: 1, ContinuationRate: 0.5},
		{Stage: StageApplied, Reached: 2, RateOfStudents: 0.5, ContinuedFromPrevious: 2, ContinuationRate: 1},
		{Stage: StageOffered, Reached: 1, RateOfStudents: 0.25, ContinuedFromPrevious: 1, ContinuationRate: 0.5},
	}
	for i, w := range want {
		g := got.Stages[i]
		if g.Stage != w.Stage || g.Reached != w.Reached || g.ContinuedFromPrevious != w.ContinuedFromPrevious ||
			!near(g.RateOfStudents, w.RateOfStudents) || !near(g.ContinuationRate, w.ContinuationRate) {
			t.Errorf("stage %d = %+v, want %+v", i, g, w)
		}
	}
}

// 母数や前の段階が0人でも 0 で割らない（NaN を返さない）。
func TestBuild_Empty(t *testing.T) {
	got := Build(nil, map[string][]uint{StageOffered: {1}})
	for _, s := range got.Stages {
		if s.Reached != 0 || s.RateOfStudents != 0 || s.ContinuationRate != 0 {
			t.Fatalf("空の母数で %+v", s)
		}
	}
	got = Build([]uint{1}, map[string][]uint{StageInterview: {1}})
	if s := got.Stages[1]; s.Reached != 1 || s.ContinuationRate != 0 {
		t.Fatalf("前の段階が0人のとき = %+v", s)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
