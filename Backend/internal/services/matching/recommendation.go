package matching

import (
	"Backend/internal/models"
	"Backend/internal/services/diagnosis"
	"encoding/json"
	"strings"
)

// MinEvaluatedCategoriesForFinal は、これ未満の評価済みカテゴリ数だと推薦を暫定表示にする閾値。
// 教員向けの暫定判定(teacher/student_insight_service.go)も同じ 4 を使う。
const MinEvaluatedCategoriesForFinal = 4

// CategoryScores は推薦1件ごとの10カテゴリのマッチ度。
type CategoryScores struct {
	Technical     float64 `json:"technical"`
	Teamwork      float64 `json:"teamwork"`
	Leadership    float64 `json:"leadership"`
	Creativity    float64 `json:"creativity"`
	Stability     float64 `json:"stability"`
	Growth        float64 `json:"growth"`
	WorkLife      float64 `json:"work_life"`
	Challenge     float64 `json:"challenge"`
	Detail        float64 `json:"detail"`
	Communication float64 `json:"communication"`
}

// Recommendation は推薦一覧の1社ぶん。
type Recommendation struct {
	ID             int            `json:"id"`
	MatchID        uint           `json:"match_id"`      // マッチングレコードID（お気に入り操作に使用）
	CategoryName   string         `json:"category_name"` // 企業名
	Score          int            `json:"score"`         // マッチスコア
	Reason         string         `json:"reason"`        // マッチ理由
	Industry       string         `json:"industry"`
	Location       string         `json:"location"`
	Employees      string         `json:"employees"`
	TechStack      []string       `json:"tech_stack"`
	CategoryScores CategoryScores `json:"category_scores"`
	IsFavorited    bool           `json:"is_favorited"`
	IsApplied      bool           `json:"is_applied"`
	// MatchedAxisCount は Score の算出に使えた軸の数（0-10、#1124）。
	// 少ないほど根拠が薄い。低マッチ警告の抑制に使う。
	// レスポンス直下の evaluated_categories（ユーザースコアの非ゼロ件数）とは別物。
	MatchedAxisCount int `json:"matched_axis_count"`
}

// RecommendationResponse は GET /api/chat/recommendations のレスポンス。
// 0件のときだけ Reason と Diagnostics が付く。
type RecommendationResponse struct {
	Recommendations     []Recommendation     `json:"recommendations"`
	Reason              string               `json:"reason,omitempty"`
	Diagnostics         *MatchingDiagnostics `json:"diagnostics,omitempty"`
	EvaluatedCategories int                  `json:"evaluated_categories"`
	IsProvisional       bool                 `json:"is_provisional"`
	DiagnosisConfidence int                  `json:"diagnosis_confidence,omitempty"`
	DiagnosisFlags      []string             `json:"diagnosis_flags,omitempty"`
	DiagnosisSummary    string               `json:"diagnosis_summary,omitempty"`
	MinMatchedAxisCount int                  `json:"min_matched_axis_count,omitempty"`
}

// CountEvaluatedCategories はスコアが0でないカテゴリの数を返す。
func CountEvaluatedCategories(scores []models.UserWeightScore) int {
	count := 0
	for _, score := range scores {
		if score.Score != 0 {
			count++
		}
	}
	return count
}

// MinMatchedAxisCount は根拠軸数の最小値を返す。
// 呼び出し側で表示対象の match だけに絞ってから渡す前提。
func MinMatchedAxisCount(matches []*models.UserCompanyMatch) int {
	if len(matches) == 0 {
		return 0
	}
	minAxes := matches[0].MatchedAxisCount
	for _, m := range matches[1:] {
		if m.MatchedAxisCount < minAxes {
			minAxes = m.MatchedAxisCount
		}
	}
	return minAxes
}

// IsRecommendationProvisional は推薦を暫定表示にするかを返す。
// minAxes が 0 のとき(マッチ無し)は、軸数を理由に暫定化しない。
func IsRecommendationProvisional(evaluatedCategories, diagnosisConfidence int, diagnosisFlags []string, minAxes int) bool {
	if evaluatedCategories < MinEvaluatedCategoriesForFinal {
		return true
	}
	if minAxes > 0 && minAxes < MinEvaluatedCategoriesForFinal {
		return true
	}
	return diagnosis.ForcesProvisional(diagnosisConfidence, diagnosisFlags)
}

// EmptyRecommendationReason は推薦が0件のときの理由コードを返す。
func EmptyRecommendationReason(d *MatchingDiagnostics) string {
	if d == nil {
		return "matching_results_empty"
	}
	switch {
	case d.UserScoreCount == 0:
		return "insufficient_user_scores"
	case d.ActiveCompanyCount == 0:
		return "insufficient_company_data"
	case d.CompaniesWithoutProfile >= d.ActiveCompanyCount:
		// 公開企業はあってもプロファイルが1社も無ければマッチングは0件になる（#1380）。
		// insufficient_company_data と混ぜると、フロントが
		// 「企業情報を公開するまでお待ちください」と案内してしまう。
		// 公開は済んでいて、必要なのはプロファイル生成なので別 reason にする。
		return "insufficient_company_profiles"
	}
	return "matching_results_empty"
}

// DisplayableMatches は企業を解決できる match だけを返す。
// 企業を解決できない match は表示されないので、根拠軸数の集計からも外す。
func DisplayableMatches(matches []*models.UserCompanyMatch) []*models.UserCompanyMatch {
	displayed := make([]*models.UserCompanyMatch, 0, len(matches))
	for _, match := range matches {
		if match.Company.ID == 0 {
			continue
		}
		displayed = append(displayed, match)
	}
	return displayed
}

// BuildRecommendations は表示対象の match を推薦一覧に整形する。
// 理由文はマッチ度と同じ合流後の適性(reasonScores)で書く。
// 0件のときは nil を返す(JSON では null)。既存の挙動を保つため。
func BuildRecommendations(displayed []*models.UserCompanyMatch, reasonScores []models.UserWeightScore) []Recommendation {
	var items []Recommendation
	for _, match := range displayed {
		employeeCount := "未定"
		if label := models.FormatEmployeeCount(match.Company.EmployeeCount, match.Company.EmployeeCountBasis); label != "" {
			employeeCount = label
		}

		items = append(items, Recommendation{
			ID:               int(match.Company.ID),
			MatchID:          match.ID,
			CategoryName:     match.Company.Name,
			Score:            int(match.MatchScore),
			Reason:           BuildMatchReason(match, reasonScores),
			Industry:         match.Company.Industry,
			Location:         match.Company.Location,
			Employees:        employeeCount,
			TechStack:        SplitTechStack(match.Company.TechStack),
			IsFavorited:      match.IsFavorited,
			IsApplied:        match.IsApplied,
			MatchedAxisCount: match.MatchedAxisCount,
			CategoryScores: CategoryScores{
				Technical:     match.TechnicalMatch,
				Teamwork:      match.TeamworkMatch,
				Leadership:    match.LeadershipMatch,
				Creativity:    match.CreativityMatch,
				Stability:     match.StabilityMatch,
				Growth:        match.GrowthMatch,
				WorkLife:      match.WorkLifeMatch,
				Challenge:     match.ChallengeMatch,
				Detail:        match.DetailMatch,
				Communication: match.CommunicationMatch,
			},
		})
	}
	return items
}

// SplitTechStack は技術スタック文字列を配列にする。JSON 配列ならそのまま、
// そうでなければカンマ区切りとして扱う。空なら空配列(null にしない)。
func SplitTechStack(techStack string) []string {
	if techStack == "" {
		return []string{}
	}
	var stack []string
	if err := json.Unmarshal([]byte(techStack), &stack); err == nil {
		return stack
	}
	result := []string{}
	for _, p := range strings.Split(techStack, ",") {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
