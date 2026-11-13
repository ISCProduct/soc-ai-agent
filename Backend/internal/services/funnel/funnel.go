// Package funnel は機能横断のコンバージョンファネル（診断 → 面接 → ES → 応募 → 内定）を組み立てる（#2039）。
//
// 段階ごとの到達者（ユーザー ID の集合）から、到達人数・母数に対する割合・前の段階からの継続率を出す。
// DB に触れる部分（到達者の取得）はリポジトリに置き、ここは集合の計算だけにして単体テストで固定する。
package funnel

// 段階の識別子。並びがファネルの順番。
const (
	StageDiagnosis = "diagnosis" // AI 診断チャットで回答した、または学校の診断を提出した
	StageInterview = "interview" // AI 面接を最後まで終えた
	StageES        = "es"        // ES の添削を受けた
	StageApplied   = "applied"   // 企業に応募した（選考ステータスが応募済み以降）
	StageOffered   = "offered"   // 内定を得た（内定・内定承諾）
)

// Stages はファネルの段階の順番。
var Stages = []string{StageDiagnosis, StageInterview, StageES, StageApplied, StageOffered}

// StageResult は1段階の集計結果。
type StageResult struct {
	Stage string `json:"stage"`
	// Reached はこの段階に到達した学生の数（前の段階を経たかは問わない）。
	Reached int `json:"reached"`
	// RateOfStudents は母数（対象の学生数）に対する Reached の割合（0〜1）。母数が0なら0。
	RateOfStudents float64 `json:"rate_of_students"`
	// ContinuedFromPrevious は、前の段階に到達した学生のうち、この段階にも到達した数。最初の段階では Reached と同じ。
	ContinuedFromPrevious int `json:"continued_from_previous"`
	// ContinuationRate は ContinuedFromPrevious / 前の段階の Reached（0〜1）。最初の段階は RateOfStudents と同じ。
	// 前の段階の到達者が0なら0。応募だけして面接練習をしない学生もいるので、到達人数の比ではなく
	// 「前の段階の到達者のうち次にも進んだ人」の割合にして、100% を超えないようにする。
	ContinuationRate float64 `json:"continuation_rate"`
}

// Result はファネル全体の集計結果。
type Result struct {
	Students int           `json:"students"`
	Stages   []StageResult `json:"stages"`
}

// Build は対象の学生と段階ごとの到達者からファネルを組み立てる。
// 到達者のうち対象の学生に含まれない ID（退会者・職員など）は数えない。
func Build(students []uint, reached map[string][]uint) Result {
	population := make(map[uint]struct{}, len(students))
	for _, id := range students {
		population[id] = struct{}{}
	}
	res := Result{Students: len(population), Stages: make([]StageResult, 0, len(Stages))}

	var prev map[uint]struct{}
	for i, stage := range Stages {
		cur := make(map[uint]struct{})
		for _, id := range reached[stage] {
			if _, ok := population[id]; ok {
				cur[id] = struct{}{}
			}
		}
		r := StageResult{Stage: stage, Reached: len(cur), RateOfStudents: ratio(len(cur), len(population))}
		if i == 0 {
			r.ContinuedFromPrevious = r.Reached
			r.ContinuationRate = r.RateOfStudents
		} else {
			both := 0
			for id := range cur {
				if _, ok := prev[id]; ok {
					both++
				}
			}
			r.ContinuedFromPrevious = both
			r.ContinuationRate = ratio(both, len(prev))
		}
		res.Stages = append(res.Stages, r)
		prev = cur
	}
	return res
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
