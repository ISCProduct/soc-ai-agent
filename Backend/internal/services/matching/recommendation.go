package matching

import (
	"Backend/internal/models"
	"Backend/internal/services/diagnosis"
	"encoding/json"
	"slices"
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
	JobTitle       string         `json:"job_title,omitempty"`
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
	// Total はお気に入り一覧の、ページを切る前の社数。
	// おすすめ企業の応答では 0 のままなので出さない。
	Total int `json:"total,omitempty"`
	// FavoriteCompanyIDs はお気に入りの全企業ID（適合度の高い順）。
	// 1ページに載らない企業のハートを、結果画面で付けるために返す。
	FavoriteCompanyIDs []uint `json:"favorite_company_ids,omitempty"`
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

// DedupedFavorites はお気に入りを企業ごとに1行へまとめる。
// 同じ企業は診断がいちばん新しい行（マッチの created_at。同点はIDが大きい方）を残す。
// updated_at は再計算や閲覧でも進むので、代表行の判定には使わない。
// 並びは適合度の高い順で、同点は企業IDの昇順。課金などで順位を上げない。
// 1社にまとめたあとに limit を超えた分を捨てる。
func DedupedFavorites(matches []*models.UserCompanyMatch, limit int) []*models.UserCompanyMatch {
	ordered := append([]*models.UserCompanyMatch(nil), matches...)
	slices.SortStableFunc(ordered, func(a, b *models.UserCompanyMatch) int {
		if a == nil || b == nil {
			if a == b {
				return 0
			}
			if a == nil {
				return 1
			}
			return -1
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			if a.CreatedAt.After(b.CreatedAt) {
				return -1
			}
			return 1
		}
		if a.ID == b.ID {
			return 0
		}
		if a.ID > b.ID {
			return -1
		}
		return 1
	})

	seen := map[uint]struct{}{}
	picked := make([]*models.UserCompanyMatch, 0)
	for _, match := range ordered {
		if match == nil || match.CompanyID == 0 {
			continue
		}
		if _, ok := seen[match.CompanyID]; ok {
			continue
		}
		seen[match.CompanyID] = struct{}{}
		picked = append(picked, match)
	}

	slices.SortStableFunc(picked, func(a, b *models.UserCompanyMatch) int {
		if a.MatchScore == b.MatchScore {
			if a.CompanyID == b.CompanyID {
				return 0
			}
			if a.CompanyID < b.CompanyID {
				return -1
			}
			return 1
		}
		if a.MatchScore > b.MatchScore {
			return -1
		}
		return 1
	})
	if limit >= 0 && len(picked) > limit {
		picked = picked[:limit]
	}
	return picked
}

// PageFavorites は企業ごとに1行へまとめたお気に入りを、offset 社目から limit 件返す。
// total はページを切る前の社数。companyIDs は表示できる全社の企業ID（適合度の高い順）。
// offset が末尾を超えてもエラーにせず、空のページと社数を返す。
// limit が負のときは末尾まで返す。
func PageFavorites(matches []*models.UserCompanyMatch, offset, limit int) (page []*models.UserCompanyMatch, total int, companyIDs []uint) {
	// 表示できない行を先に外す。ページを切ったあとで落とすと、
	// 100社に満たないページの次に、まだ見えていない企業が出てしまう。
	all := DisplayableMatches(DedupedFavorites(matches, -1))
	total = len(all)
	companyIDs = make([]uint, total)
	for i, row := range all {
		companyIDs[i] = row.CompanyID
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*models.UserCompanyMatch{}, total, companyIDs
	}
	end := total
	if limit >= 0 && offset+limit < end {
		end = offset + limit
	}
	return all[offset:end], total, companyIDs
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

		jobTitle := ""
		if match.JobPosition != nil && match.JobPosition.ID != 0 {
			jobTitle = match.JobPosition.Title
		}

		items = append(items, Recommendation{
			ID:               int(match.Company.ID),
			MatchID:          match.ID,
			CategoryName:     match.Company.Name,
			Score:            int(match.MatchScore),
			Reason:           BuildMatchReason(match, reasonScores),
			Industry:         match.Company.Industry,
			JobTitle:         jobTitle,
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
