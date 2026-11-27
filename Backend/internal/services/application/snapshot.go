package application

import (
	"encoding/json"
	"time"

	"Backend/domain/valueobject"
	"Backend/internal/models"
	"Backend/internal/services/assessment"
)

// snapshotLogicVersion は合成に使ったルールの識別子。ChatWeight を変えたら一緒に変える。
const snapshotLogicVersion = "merge-chat-weight-0.5"

// scoreSnapshotDocument は応募時点の版付きスナップショット。
// categories は従来の user_weight_scores（チャット側）。較正はここを読み続ける。
// match_score と axis_match は、その応募に使ったマッチ行のコピー。再マッチングで上書きされない。
type scoreSnapshotDocument struct {
	Version          int                `json:"version"`
	LogicVersion     string             `json:"logic_version"`
	ChatWeight       float64            `json:"chat_weight"`
	CapturedAt       string             `json:"captured_at"`
	Categories       map[string]float64 `json:"categories"`
	MatchScore       float64            `json:"match_score"`
	MatchedAxisCount int                `json:"matched_axis_count"`
	AxisMatch        map[string]float64 `json:"axis_match"`
}

func buildScoreSnapshotDocument(categories map[string]float64, match *models.UserCompanyMatch, now time.Time) string {
	if len(categories) == 0 && match == nil {
		return ""
	}
	doc := scoreSnapshotDocument{
		Version:      2,
		LogicVersion: snapshotLogicVersion,
		ChatWeight:   assessment.ChatWeight,
		CapturedAt:   now.UTC().Format(time.RFC3339),
		Categories:   categories,
		AxisMatch:    map[string]float64{},
	}
	if doc.Categories == nil {
		doc.Categories = map[string]float64{}
	}
	if match != nil {
		doc.MatchScore = match.MatchScore
		doc.MatchedAxisCount = match.MatchedAxisCount
		doc.AxisMatch = axisMatch(match)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(b)
}

func axisMatch(match *models.UserCompanyMatch) map[string]float64 {
	return map[string]float64{
		string(valueobject.CategoryTechnical):     match.TechnicalMatch,
		string(valueobject.CategoryTeamwork):      match.TeamworkMatch,
		string(valueobject.CategoryLeadership):    match.LeadershipMatch,
		string(valueobject.CategoryCreativity):    match.CreativityMatch,
		string(valueobject.CategoryStability):     match.StabilityMatch,
		string(valueobject.CategoryGrowth):        match.GrowthMatch,
		string(valueobject.CategoryWorkLife):      match.WorkLifeMatch,
		string(valueobject.CategoryChallenge):     match.ChallengeMatch,
		string(valueobject.CategoryDetail):        match.DetailMatch,
		string(valueobject.CategoryCommunication): match.CommunicationMatch,
	}
}
