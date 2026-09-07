package resume

import (
	"Backend/internal/config"
)

// ResumeStatus は学生の履歴書の対応要否を表す(#1030)。
// リマインダー表示の判定に使う。
type ResumeStatus struct {
	// HasDocument は履歴書を一度でもアップロードしているか。
	HasDocument bool `json:"has_document"`
	// LatestScore は最新レビューのスコア(0-100)。レビュー未生成なら nil。
	LatestScore *int `json:"latest_score"`
	// NeedsAttention は学生に改善を促すべき状態か。
	NeedsAttention bool `json:"needs_attention"`
}

// EvaluateResumeStatus は取得済みの事実から対応要否を判定する。
//
// DB へ触らない純関数にしてあるのは、教員向け一覧(#1027)でも同じ判定を
// 使い回せるようにするため。一覧側は N+1 を避けて一括取得した結果を渡す。
//
// 判定は3状態。
//   - 未提出            -> 要対応(履歴書を作るよう促す)
//   - 提出済み・レビュー未生成 -> 対応不要(処理中であり学生に打つ手がない)
//   - 提出済み・スコアあり   -> 閾値未満なら要対応
func EvaluateResumeStatus(hasDocument bool, latestScore *int, threshold int) ResumeStatus {
	status := ResumeStatus{HasDocument: hasDocument, LatestScore: latestScore}
	switch {
	case !hasDocument:
		status.NeedsAttention = true
	case latestScore == nil:
		status.NeedsAttention = false
	default:
		status.NeedsAttention = *latestScore < threshold
	}
	return status
}

// GetResumeStatus は指定ユーザーの履歴書の対応要否を返す。
// 呼び出し元(コントローラ)で本人以外のIDを渡さないこと。
func (s *ResumeService) GetResumeStatus(userID uint) (*ResumeStatus, error) {
	doc, review, err := s.repo.FindLatestDocumentWithReview(userID)
	if err != nil {
		return nil, err
	}

	var latestScore *int
	if review != nil {
		score := review.Score
		latestScore = &score
	}

	status := EvaluateResumeStatus(doc != nil, latestScore, config.ResumeCompletenessThreshold())
	return &status, nil
}
