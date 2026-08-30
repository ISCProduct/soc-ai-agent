package hr

import (
	"Backend/internal/services/flywheel"
	"errors"
)

// ErrStudentNotVisible 学生が存在しない、またはスカウト公開に未同意のとき（404で返す）
var ErrStudentNotVisible = errors.New("student not visible")

// StudentAnalysisResponse 企業向け学生分析プロファイル
type StudentAnalysisResponse struct {
	UserID            uint                           `json:"user_id"`
	IntegratedProfile *flywheel.UserIntegratedProfile `json:"integrated_profile"`
	ChatSummary       *ChatSummaryView               `json:"chat_summary,omitempty"`
	InterviewReports  []InterviewReportView          `json:"interview_reports"`
}

// ChatSummaryView チャットセッションのLLM要約（企業公開用）
type ChatSummaryView struct {
	Strengths               string `json:"strengths"`
	Weaknesses              string `json:"weaknesses"`
	RecommendedWorkingStyle string `json:"recommended_working_style"`
}

// InterviewReportView 面接レポート（教員向け詳細は除外）
type InterviewReportView struct {
	SessionID        uint   `json:"session_id"`
	SummaryText      string `json:"summary_text"`
	StrengthsJSON    string `json:"strengths_json"`
	ImprovementsJSON string `json:"improvements_json"`
}
