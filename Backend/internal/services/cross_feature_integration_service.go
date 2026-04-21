package services

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"encoding/json"
	"fmt"
	"log"
	"math"
)

// CrossFeatureIntegrationService 機能間データ連携サービス
// 面接レポート・職務経歴書レビューの結果を UserWeightScore に反映し、
// チャット分析スコアを面接・RAG のコンテキストとして活用する。
type CrossFeatureIntegrationService struct {
	weightScoreRepo *repositories.UserWeightScoreRepository
}

func NewCrossFeatureIntegrationService(
	weightScoreRepo *repositories.UserWeightScoreRepository,
) *CrossFeatureIntegrationService {
	return &CrossFeatureIntegrationService{weightScoreRepo: weightScoreRepo}
}

// ── 面接レポート → UserWeightScore ──────────────────────────────────────────

// interviewScoreMapping 面接5項目と10カテゴリの対応と重み
// 面接スコアは 0-5、UserWeightScore は 0-100 なので ×20 で正規化する
var interviewScoreMapping = []struct {
	interviewKey string
	categories   []string // 反映先カテゴリ（複数可: 均等に按分）
}{
	{"communication", []string{"コミュニケーション力"}},
	{"logic", []string{"技術志向"}},
	{"specificity", []string{"細部志向"}},
	{"ownership", []string{"リーダーシップ", "チャレンジ志向"}},
	{"enthusiasm", []string{"成長志向", "チームワーク"}},
}

// UpdateScoresFromInterviewReport 面接レポートを元に UserWeightScore を更新する
// セッションIDは面接セッションID（文字列変換して利用）
func (s *CrossFeatureIntegrationService) UpdateScoresFromInterviewReport(
	userID uint,
	chatSessionID string,
	report *models.InterviewReport,
) error {
	if report == nil || report.ScoresJSON == "" {
		return nil
	}

	var interviewScores map[string]int
	if err := json.Unmarshal([]byte(report.ScoresJSON), &interviewScores); err != nil {
		return fmt.Errorf("面接スコアのパースエラー: %w", err)
	}

	for _, mapping := range interviewScoreMapping {
		raw, ok := interviewScores[mapping.interviewKey]
		if !ok {
			continue
		}
		// 0-5 → 0-100 に正規化
		normalized := raw * 20

		// 複数カテゴリに均等按分して移動平均で更新
		for _, category := range mapping.categories {
			if err := s.applyMovingAverage(userID, chatSessionID, category, normalized); err != nil {
				// 更新失敗は警告ログのみ（処理継続）
				log.Printf("[CrossFeature] interview→score update failed (cat=%s): %v\n", category, err)
			}
		}
	}
	return nil
}

// ── 職務経歴書レビュー → UserWeightScore ──────────────────────────────────

// UpdateScoresFromResumeReview 職務経歴書レビューを元に UserWeightScore を補正する
func (s *CrossFeatureIntegrationService) UpdateScoresFromResumeReview(
	userID uint,
	chatSessionID string,
	review *models.ResumeReview,
	items []models.ResumeReviewItem,
) error {
	if review == nil {
		return nil
	}

	// 総合スコアを 0-100 として活用
	score := review.Score

	// critical 件数をカウント
	criticalCount := 0
	for _, item := range items {
		if item.Severity == "critical" {
			criticalCount++
		}
	}

	// スコアが高い場合: 表現力・詳細志向・技術志向を加点
	if score >= 70 {
		bonus := int(math.Round(float64(score-70) / 3)) // 最大 +10
		for _, category := range []string{"細部志向", "コミュニケーション力", "技術志向"} {
			if err := s.applyMovingAverage(userID, chatSessionID, category, score+bonus); err != nil {
				log.Printf("[CrossFeature] resume→score bonus failed (cat=%s): %v\n", category, err)
			}
		}
	}

	// critical 指摘が多い場合: 関連カテゴリを現在値より低く調整
	if criticalCount >= 3 {
		penalty := clampInt(score-10*criticalCount, 0, 100)
		for _, category := range []string{"細部志向", "コミュニケーション力"} {
			if err := s.applyMovingAverage(userID, chatSessionID, category, penalty); err != nil {
				log.Printf("[CrossFeature] resume→score penalty failed (cat=%s): %v\n", category, err)
			}
		}
	}
	return nil
}

// ── チャット分析スコア → 面接コンテキスト ────────────────────────────────

// BuildInterviewContextFromScores チャット分析スコアを面接システムプロンプト用テキストに変換する
func (s *CrossFeatureIntegrationService) BuildInterviewContextFromScores(
	userID uint,
	chatSessionID string,
) string {
	scores, err := s.weightScoreRepo.FindByUserAndSession(userID, chatSessionID)
	if err != nil || len(scores) == 0 {
		return ""
	}

	top, bottom := extractTopBottom(scores)

	lines := "【受験者プロファイル（参考情報）】\n"
	if len(top) > 0 {
		lines += "強み傾向: "
		for i, s := range top {
			if i > 0 {
				lines += "、"
			}
			lines += fmt.Sprintf("%s(%d点)", s.WeightCategory, s.Score)
		}
		lines += "\n"
	}
	if len(bottom) > 0 {
		lines += "成長余地: "
		for i, s := range bottom {
			if i > 0 {
				lines += "、"
			}
			lines += fmt.Sprintf("%s(%d点)", s.WeightCategory, s.Score)
		}
		lines += "\n"
	}
	lines += "※ 上記は参考情報です。面接では受験者の実際の回答を重視してください。\n"
	return lines
}

// BuildInterviewContextFromUser チャットセッションIDなしでユーザーの最新スコアを面接コンテキストに変換する
func (s *CrossFeatureIntegrationService) BuildInterviewContextFromUser(userID uint) string {
	scores, err := s.weightScoreRepo.FindLatestByUser(userID)
	if err != nil || len(scores) == 0 {
		return ""
	}

	top, bottom := extractTopBottom(scores)

	lines := "【受験者プロファイル（参考情報）】\n"
	if len(top) > 0 {
		lines += "強み傾向: "
		for i, s := range top {
			if i > 0 {
				lines += "、"
			}
			lines += fmt.Sprintf("%s(%d点)", s.WeightCategory, s.Score)
		}
		lines += "\n"
	}
	if len(bottom) > 0 {
		lines += "成長余地: "
		for i, s := range bottom {
			if i > 0 {
				lines += "、"
			}
			lines += fmt.Sprintf("%s(%d点)", s.WeightCategory, s.Score)
		}
		lines += "\n"
	}
	lines += "※ 上記は参考情報です。面接では受験者の実際の回答を重視してください。\n"
	return lines
}

// BuildResumeContextFromUser チャットセッションIDなしでユーザーの最新スコアを職務経歴書レビューコンテキストに変換する
func (s *CrossFeatureIntegrationService) BuildResumeContextFromUser(userID uint) string {
	scores, err := s.weightScoreRepo.FindLatestByUser(userID)
	if err != nil || len(scores) == 0 {
		return ""
	}

	top, _ := extractTopBottom(scores)
	if len(top) == 0 {
		return ""
	}

	text := "【候補者の強み傾向（チャット診断より）】\n"
	for _, s := range top {
		text += fmt.Sprintf("- %s: %d点\n", s.WeightCategory, s.Score)
	}
	text += "上記の強みが経歴書でどう表現されているか、特に確認してください。\n"
	return text
}

// BuildResumeContextFromScores チャット分析スコアを RAG レビューのコンテキストに変換する
func (s *CrossFeatureIntegrationService) BuildResumeContextFromScores(
	userID uint,
	chatSessionID string,
) string {
	scores, err := s.weightScoreRepo.FindByUserAndSession(userID, chatSessionID)
	if err != nil || len(scores) == 0 {
		return ""
	}

	top, _ := extractTopBottom(scores)
	if len(top) == 0 {
		return ""
	}

	text := "【候補者の強み傾向（チャット診断より）】\n"
	for _, s := range top {
		text += fmt.Sprintf("- %s: %d点\n", s.WeightCategory, s.Score)
	}
	text += "上記の強みが経歴書でどう表現されているか、特に確認してください。\n"
	return text
}

// ── 統合プロファイル ──────────────────────────────────────────────────────

// UserIntegratedProfile ユーザーの統合プロファイル
type UserIntegratedProfile struct {
	UserID        uint                     `json:"user_id"`
	ChatSessionID string                   `json:"chat_session_id"`
	WeightScores  []entity.UserWeightScore `json:"weight_scores"`
	TopCategories []entity.UserWeightScore `json:"top_categories"`
	SourceSummary ProfileSourceSummary     `json:"source_summary"`
}

// ProfileSourceSummary 各機能からのデータ取得状況
type ProfileSourceSummary struct {
	HasChatScores    bool `json:"has_chat_scores"`
	InterviewCount   int  `json:"interview_count"`
	ResumeReviewDone bool `json:"resume_review_done"`
}

// BuildIntegratedProfile 統合プロファイルを構築する
func (s *CrossFeatureIntegrationService) BuildIntegratedProfile(
	userID uint,
	chatSessionID string,
	interviewCount int,
	resumeReviewDone bool,
) (*UserIntegratedProfile, error) {
	scores, err := s.weightScoreRepo.FindByUserAndSession(userID, chatSessionID)
	if err != nil {
		return nil, fmt.Errorf("スコア取得エラー: %w", err)
	}

	top, _ := extractTopBottom(scores)

	return &UserIntegratedProfile{
		UserID:        userID,
		ChatSessionID: chatSessionID,
		WeightScores:  scores,
		TopCategories: top,
		SourceSummary: ProfileSourceSummary{
			HasChatScores:    len(scores) > 0,
			InterviewCount:   interviewCount,
			ResumeReviewDone: resumeReviewDone,
		},
	}, nil
}

// ── ヘルパー ─────────────────────────────────────────────────────────────

// applyMovingAverage 移動平均（新30% + 既存70%）でスコアを更新する
// UpdateScore は加算式なので、差分（delta）を計算して渡す
func (s *CrossFeatureIntegrationService) applyMovingAverage(
	userID uint, sessionID, category string, newValue int,
) error {
	existing, err := s.weightScoreRepo.FindByUserSessionAndCategory(userID, sessionID, category)
	if err != nil || existing == nil {
		// 新規: そのまま設定（incremental = newValue）
		return s.weightScoreRepo.UpdateScore(userID, sessionID, category, newValue)
	}

	// 移動平均: new = existing * 0.7 + newValue * 0.3
	blended := int(math.Round(float64(existing.Score)*0.7 + float64(newValue)*0.3))
	delta := blended - existing.Score
	if delta == 0 {
		return nil
	}
	return s.weightScoreRepo.UpdateScore(userID, sessionID, category, delta)
}

// extractTopBottom スコア上位3件と下位3件を返す
func extractTopBottom(scores []entity.UserWeightScore) (top, bottom []entity.UserWeightScore) {
	if len(scores) == 0 {
		return nil, nil
	}
	sorted := make([]entity.UserWeightScore, len(scores))
	copy(sorted, scores)

	// バブルソート（降順）
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Score > sorted[i].Score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := 3
	if len(sorted) < n {
		n = len(sorted)
	}
	top = sorted[:n]

	if len(sorted) > n {
		bottomStart := len(sorted) - n
		if bottomStart < n {
			bottomStart = n
		}
		bottom = sorted[bottomStart:]
	}
	return top, bottom
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
