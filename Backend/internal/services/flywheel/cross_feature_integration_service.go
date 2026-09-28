package flywheel

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
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

// interviewScoreMapping 面接5項目と10カテゴリの対応
// 値域の変換は mapInterviewScore（score_mapping.go）が行う
//
// interviewKey は面接プロンプトのルーブリックキー（interview.RubricKeys）と同じ綴りである必要がある。
// ずれた項目は interviewScores から引けず、そのカテゴリが一切書かれない（ログも出ない）。
// flywheel は interview を import できない（逆向きの依存がある）ため、
// 一致は InterviewRubricKeys 経由で interview 側のテストが固定している（#1554）。
var interviewScoreMapping = []struct {
	interviewKey string
	categories   []string // 反映先カテゴリ（複数可: 均等に按分）
}{
	{"communication", []string{"コミュニケーション力"}},
	{"logic", []string{"技術志向"}},
	{"specificity", []string{"細部志向"}},
	{"ownership", []string{"リーダーシップ志向", "チャレンジ志向"}},
	{"enthusiasm", []string{"成長志向", "チームワーク志向"}},
}

// InterviewRubricKeys は写像が参照している面接ルーブリックキーを定義順で返す。
// プロンプト側との一致をテストで固定するために公開している。
func InterviewRubricKeys() []string {
	keys := make([]string, len(interviewScoreMapping))
	for i, m := range interviewScoreMapping {
		keys[i] = m.interviewKey
	}
	return keys
}

// ResolveDiagnosisSessionID は面接スコアを流し込むチャット診断セッションを決める。
// チャット診断が無い場合のみ interview-{userID} スナップショットへフォールバックする。
// 検索そのものが失敗した場合はエラーを返す（一時障害をスナップショットへ書き込まないため）。
func (s *CrossFeatureIntegrationService) ResolveDiagnosisSessionID(userID uint) (string, error) {
	if s.weightScoreRepo == nil {
		return "", errors.New("weightScoreRepo が未注入")
	}
	id, err := s.weightScoreRepo.FindLatestDiagnosisSessionID(userID)
	return PickDiagnosisSessionID(userID, id, err)
}

// PickDiagnosisSessionID は診断セッション選定の純関数（テスト用にも公開）。
// 「診断が無い」(gorm.ErrRecordNotFound) だけをフォールバック条件にし、
// 接続断などの一時エラーは呼び出し元へ返す。
func PickDiagnosisSessionID(userID uint, latestChatSession string, err error) (string, error) {
	fallback := fmt.Sprintf("interview-%d", userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fallback, nil
		}
		return "", err
	}
	if strings.TrimSpace(latestChatSession) == "" || repositories.IsInterviewSnapshotSession(latestChatSession) {
		return fallback, nil
	}
	return latestChatSession, nil
}

// UpdateScoresFromInterviewReport 面接レポートを元に UserWeightScore を更新する
// chatSessionID は診断・マッチング対象のセッション（ResolveDiagnosisSessionID の結果を渡す）。
// stats は発話ログから作る補正用の連続量（NewInterviewTranscriptStats）。
// ゼロ値を渡した場合は補正を中立扱いにするので、発話を持たない呼び出し元でも使える。
func (s *CrossFeatureIntegrationService) UpdateScoresFromInterviewReport(
	userID uint,
	chatSessionID string,
	report *models.InterviewReport,
	stats InterviewTranscriptStats,
) error {
	if report == nil || report.ScoresJSON == "" {
		return nil
	}
	if strings.TrimSpace(chatSessionID) == "" {
		resolved, err := s.ResolveDiagnosisSessionID(userID)
		if err != nil {
			return fmt.Errorf("診断セッションの解決に失敗: %w", err)
		}
		chatSessionID = resolved
	}

	var interviewScores map[string]int
	if err := json.Unmarshal([]byte(report.ScoresJSON), &interviewScores); err != nil {
		return fmt.Errorf("面接スコアのパースエラー: %w", err)
	}
	// evidence は壊れていても補正が中立に寄るだけなのでエラーにしない。
	// nil map になっても evidence[key] は "" で、interviewSignal が欠損を中立(0.5)扱いにするため
	// ルーブリック相応の値がそのまま反映される（#1554 で実装をこのコメントに合わせた）。
	var evidence map[string]string
	_ = json.Unmarshal([]byte(report.EvidenceJSON), &evidence)

	applied, failed := 0, 0
	for _, mapping := range interviewScoreMapping {
		raw, ok := interviewScores[mapping.interviewKey]
		if !ok {
			continue
		}
		// 0-5 → 0-100。evidence 量と回答量で ±5点だけ補正して分解能を上げる（#1528）
		normalized := mapInterviewScore(raw, evidence[mapping.interviewKey], stats)

		// 複数カテゴリへ同じ値を移動平均で反映
		for _, category := range mapping.categories {
			if err := s.applyMovingAverage(userID, chatSessionID, category, normalized); err != nil {
				// 一部失敗は警告ログのみ（処理継続）
				log.Printf("[CrossFeature] interview→score update failed (cat=%s): %v\n", category, err)
				failed++
				continue
			}
			applied++
		}
	}
	// 1件も書けていないのに成功を返すと、呼び出し元が面接前スコアで再マッチングしてしまう。
	// 部分成功（applied > 0）は許容する。移動平均なので一部でも反映された方が面接前より近く、
	// ここでエラーにすると書けた分まで再マッチングされずに宙に浮く。
	if applied == 0 && failed > 0 {
		return fmt.Errorf("面接スコアの反映が全件失敗しました (%d件)", failed)
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

	criticalCount := 0
	for _, item := range items {
		if item.Severity == "critical" {
			criticalCount++
		}
	}

	// 対応表と式は resumeScoreMapping / mapResumeScore（score_mapping.go）に集約している。
	// 高ければ高く、低ければ低く、スコア全域を使って反映する（#1528）。
	for _, mapped := range mapResumeScore(review.Score, criticalCount) {
		if err := s.applyMovingAverage(userID, chatSessionID, mapped.category, mapped.score); err != nil {
			log.Printf("[CrossFeature] resume→score update failed (cat=%s): %v\n", mapped.category, err)
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
	// リポジトリ側も 0〜100 に丸めるが、呼び出し側でも範囲を守る。
	// 丸めに頼ると、範囲外の値が「意図」なのか事故なのか読み取れなくなる（#1528）。
	newValue = clampInt(newValue, 0, 100)

	existing, err := s.weightScoreRepo.FindByUserSessionAndCategory(userID, sessionID, category)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("スコア取得エラー: %w", err)
	}
	if existing == nil {
		// 新規: 絶対値でセット
		return s.weightScoreRepo.SetScore(userID, sessionID, category, newValue)
	}

	delta := blendScore(existing.Score, newValue) - existing.Score
	if delta == 0 {
		return nil
	}
	return s.weightScoreRepo.AddScore(userID, sessionID, category, delta)
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
