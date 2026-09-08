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
// DB へ触らない純関数なので、教員向け一覧(#1027)からも同じ判定を呼べる。
//
// 判定は3状態。
//   - 未提出              -> 要対応(履歴書を作るよう促す)
//   - 提出済み・レビュー未実施 -> 要対応(レビューを実行するよう促す)
//   - 提出済み・スコアあり    -> 閾値未満なら要対応
//
// 「レビュー未実施」を要対応にしている理由。
// PRD はこの状態を「レビュー処理中なので対応不要」とし、どちらにするかは実装時に
// 決めるとしていた。しかし実際のレビューは自動生成ではなく、学生が企業名か職種を
// 入力して明示的に実行する操作である(resume_review.go:81 が唯一の CreateReview 経路)。
// 対応不要にすると次の2つが起きる。
//  1. アップロードしただけの学生に永久にリマインダーが出ない
//  2. 低スコアで警告中の学生が新しい履歴書を上げると、最新ドキュメントに
//     レビューが無いため警告が黙って消える
//
// どちらも本機能の目的に反するため、学生が行動できる状態として要対応にした。
func EvaluateResumeStatus(hasDocument bool, latestScore *int, threshold int) ResumeStatus {
	status := ResumeStatus{HasDocument: hasDocument, LatestScore: latestScore}
	switch {
	case !hasDocument:
		status.NeedsAttention = true
	case latestScore == nil:
		status.NeedsAttention = true
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
