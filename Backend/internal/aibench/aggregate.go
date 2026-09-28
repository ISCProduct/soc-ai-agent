package aibench

import (
	"sort"

	"Backend/internal/services/costs"
)

// 破損の理由。
//
// 分けて数えるのは対策が違うため。出力上限到達は max_tokens を上げれば直るが、
// JSON不正はプロンプトかモデルの問題で、上限を上げても直らない。
const (
	BrokenJSONInvalid = "json_invalid"      // JSONとして読めない（本番の復旧処理を通した後）
	BrokenTruncated   = "output_truncated"  // 出力上限に到達して途中で切れた
	BrokenCallFailed  = "call_failed"       // 応答は得たが使える出力ではなかった
	BrokenSchema      = "schema_incomplete" // JSONは読めたが必須キーが無くスコアが取れない

	// BrokenNetwork は通信・レート制限・サーバ側エラー。品質ではないので
	// 破損率とレイテンシの両方から除外し、「計測できなかった回数」として別に出す。
	//
	// 実測でこれを混ぜると数字が壊れた: ローカルの通信断2回で破損率が 11.1% になり、
	// 120秒のタイムアウトが p95 を 120,001ms に押し上げた。どちらもモデルの品質とは
	// 無関係で、測り直せば消える。破損率に混ぜると「プロンプトを直したら破損率が
	// 下がった」という誤った結論が出る。
	BrokenNetwork = "network_error"
)

// Observation は1ケース1回の実行結果。
type Observation struct {
	CaseID string `json:"case_id"`
	Label  string `json:"label"`
	Run    int    `json:"run"`

	LatencyMS        int64 `json:"latency_ms"`
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	// TokensEstimated はトークン数が実測ではなく概算であることを示す。
	// ES添削は RAG 経由で usage が返らないため概算になる。
	TokensEstimated bool `json:"tokens_estimated"`

	Broken       bool   `json:"broken"`
	BrokenReason string `json:"broken_reason,omitempty"`
	// Fatal は「設定の誤りであって品質ではない」失敗を示す（認証エラー・APIキー未設定）。
	// これを破損率として集計すると、設定ミスの実行が「破損率100%」という
	// もっともらしい数字を出してしまう。検知したら実行を打ち切る。
	Fatal bool `json:"-"`
	// Error は破損時の内容。人が原因を追えるようにする（APIキーは含めない）。
	Error string `json:"error,omitempty"`

	// Score は 0.0〜1.0 に正規化したスコア。Broken のときは無効。
	Score float64 `json:"score"`
	// RawScore は正規化前の値（ES: 1-10、履歴書: 0-100、面接: 0-5）。
	RawScore float64 `json:"raw_score"`

	// Violations は指示違反の内容。空なら指示を守れている。
	Violations []string `json:"violations,omitempty"`
}

// CaseSummary はケース単位の集計。再現性（同一入力のばらつき）をここで見る。
type CaseSummary struct {
	CaseID      string  `json:"case_id"`
	Label       string  `json:"label"`
	Runs        int     `json:"runs"`
	BrokenRuns  int     `json:"broken_runs"`
	MeanScore   float64 `json:"mean_score"`
	ScoreStdDev float64 `json:"score_stddev"`
}

// Summary は1回の実行（1 target × 1モデル）の集計。
//
// Model と GeneratedAt を必ず残す。OpenAI 側のモデル更新で同じモデル名でも
// 数字が変わるため、いつ・どのモデルで測ったかが無い結果は比較に使えない。
type Summary struct {
	Target      string `json:"target"`
	Model       string `json:"model"`
	GeneratedAt string `json:"generated_at"`
	// Endpoint は呼び出し先。ES は RAG の URL、それ以外は OpenAI の API 種別。
	Endpoint string `json:"endpoint"`
	// ManifestPath / ManifestSHA256 は入力の同一性を示す。
	// 入力が違えば数字が違うのは当たり前なので、差分を見る前にここを比べる。
	ManifestPath   string `json:"manifest_path"`
	ManifestSHA256 string `json:"manifest_sha256"`

	Cases int `json:"cases"`
	Runs  int `json:"runs_per_case"`

	// 破損率: JSON不正＋出力上限到達＋使えない出力 / 計測できた実行回数
	BrokenRate     float64        `json:"broken_rate"`
	BrokenByReason map[string]int `json:"broken_by_reason,omitempty"`
	// UnmeasuredRuns は通信エラー等で計測できなかった回数（破損率の分母から除外）。
	// これが多い実行は数字自体が信用できない。
	UnmeasuredRuns int `json:"unmeasured_runs"`

	// 再現性: ケースごとのスコア標準偏差の平均と最大（小さいほど安定）
	MeanScoreStdDev float64 `json:"mean_score_stddev"`
	MaxScoreStdDev  float64 `json:"max_score_stddev"`
	// UnstableCaseIDs は標準偏差が UnstableStdDevThreshold を超えたケース。
	UnstableCaseIDs []string `json:"unstable_case_ids,omitempty"`

	// 弁別力: ゴールドラベル（良/中/悪）とスコアの順位相関（1.0が完全一致）。
	LabelRankCorrelation float64            `json:"label_rank_correlation"`
	MeanScoreByLabel     map[string]float64 `json:"mean_score_by_label"`

	// 指示遵守率: 違反が1件も無かった実行の割合（破損した実行は分母から外す）
	ComplianceRate  float64        `json:"compliance_rate"`
	ViolationCounts map[string]int `json:"violation_counts,omitempty"`

	LatencyP50MS int64 `json:"latency_p50_ms"`
	LatencyP95MS int64 `json:"latency_p95_ms"`

	// コスト: 1件（1ケース1回）あたりの概算と合計
	CostPerCallUSD  float64 `json:"cost_per_call_usd"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	CostIsEstimated bool    `json:"cost_is_estimated"`

	CaseDetails  []CaseSummary `json:"cases_detail"`
	Observations []Observation `json:"observations"`
}

// UnstableStdDevThreshold は「再現性が低い」とみなす標準偏差の閾値（正規化スコア）。
// 0.1 は 10点満点で1点、100点満点で10点のばらつきに相当する。
const UnstableStdDevThreshold = 0.1

// Aggregate は実行結果を集計する。
//
// スコア系の指標（再現性・弁別力）は破損した実行を除いて計算する。
// 破損時のスコアは存在しないため、0 として混ぜると破損が多いモデルの
// 弁別力が偶然高く出る。破損は破損率で別に数える。
// レイテンシは破損した実行も含める。待ち時間は失敗でも発生する。
func Aggregate(target, model, endpoint, generatedAt string, obs []Observation) *Summary {
	s := &Summary{
		Target:           target,
		Model:            model,
		Endpoint:         endpoint,
		GeneratedAt:      generatedAt,
		BrokenByReason:   map[string]int{},
		ViolationCounts:  map[string]int{},
		MeanScoreByLabel: map[string]float64{},
		Observations:     obs,
	}
	if len(obs) == 0 {
		return s
	}

	byCase := map[string][]Observation{}
	caseOrder := []string{}
	latencies := make([]float64, 0, len(obs))
	broken := 0
	compliant, compliantDenom := 0, 0
	var promptTokens, completionTokens int
	estimated := false

	measured := 0
	for _, o := range obs {
		if _, ok := byCase[o.CaseID]; !ok {
			caseOrder = append(caseOrder, o.CaseID)
		}
		byCase[o.CaseID] = append(byCase[o.CaseID], o)
		promptTokens += o.PromptTokens
		completionTokens += o.CompletionTokens
		if o.TokensEstimated {
			estimated = true
		}
		// 通信エラーは計測できなかった回数として別に数え、
		// 破損率・レイテンシの両方から外す（品質の話ではない）。
		if o.BrokenReason == BrokenNetwork {
			s.UnmeasuredRuns++
			continue
		}
		measured++
		latencies = append(latencies, float64(o.LatencyMS))
		if o.Broken {
			broken++
			s.BrokenByReason[o.BrokenReason]++
			continue
		}
		// 指示遵守率は「出力が得られた実行」の中で測る。
		// 破損した実行を違反として数えると破損率と二重計上になる。
		compliantDenom++
		if len(o.Violations) == 0 {
			compliant++
		}
		for _, v := range o.Violations {
			s.ViolationCounts[v]++
		}
	}

	s.Cases = len(byCase)
	s.Runs = len(obs) / max(1, len(byCase))
	if measured > 0 {
		s.BrokenRate = float64(broken) / float64(measured)
	}
	if compliantDenom > 0 {
		s.ComplianceRate = float64(compliant) / float64(compliantDenom)
	}
	s.LatencyP50MS = int64(Percentile(latencies, 50))
	s.LatencyP95MS = int64(Percentile(latencies, 95))

	// ケース単位: 再現性と、弁別力に使う代表値（破損を除いた平均）
	labelSeries := make([]float64, 0, len(byCase))
	scoreSeries := make([]float64, 0, len(byCase))
	stddevs := make([]float64, 0, len(byCase))
	scoresByLabel := map[string][]float64{}
	for _, id := range caseOrder {
		runs := byCase[id]
		scores := make([]float64, 0, len(runs))
		brokenRuns := 0
		for _, o := range runs {
			if o.Broken {
				if o.BrokenReason != BrokenNetwork {
					brokenRuns++
				}
				continue
			}
			scores = append(scores, o.Score)
		}
		cs := CaseSummary{CaseID: id, Label: runs[0].Label, Runs: len(runs), BrokenRuns: brokenRuns}
		if len(scores) > 0 {
			cs.MeanScore = Mean(scores)
			cs.ScoreStdDev = StdDev(scores)
			stddevs = append(stddevs, cs.ScoreStdDev)
			if cs.ScoreStdDev > UnstableStdDevThreshold {
				s.UnstableCaseIDs = append(s.UnstableCaseIDs, id)
			}
			if rank, ok := labelRank[cs.Label]; ok {
				labelSeries = append(labelSeries, rank)
				scoreSeries = append(scoreSeries, cs.MeanScore)
				scoresByLabel[cs.Label] = append(scoresByLabel[cs.Label], cs.MeanScore)
			}
		}
		s.CaseDetails = append(s.CaseDetails, cs)
	}
	s.MeanScoreStdDev = Mean(stddevs)
	for _, d := range stddevs {
		if d > s.MaxScoreStdDev {
			s.MaxScoreStdDev = d
		}
	}
	sort.Strings(s.UnstableCaseIDs)
	s.LabelRankCorrelation = SpearmanCorrelation(labelSeries, scoreSeries)
	for label, xs := range scoresByLabel {
		s.MeanScoreByLabel[label] = Mean(xs)
	}

	s.TotalCostUSD = costs.EstimateCostUSD(model, promptTokens, completionTokens)
	// 1件あたりは計測できた回数で割る。課金は破損した呼び出しにも発生するが、
	// 通信エラーで応答が返らなかった回にはトークン課金が無い。
	if measured > 0 {
		s.CostPerCallUSD = s.TotalCostUSD / float64(measured)
	}
	s.CostIsEstimated = estimated
	return s
}
