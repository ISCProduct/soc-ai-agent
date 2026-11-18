package matching

import (
	"errors"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// MatchFeedbackNone は、判定の取り消し（フィードバックを消す）。
const MatchFeedbackNone = "none"

// ErrInvalidFeedbackVerdict は、知らない判定。
var ErrInvalidFeedbackVerdict = errors.New("invalid feedback verdict")

// MatchFeedbackStore は、フィードバックの永続化。
type MatchFeedbackStore interface {
	// FindMatch は、判定を付ける推薦。無ければ repository.ErrNotFound。
	FindMatch(matchID uint) (*models.UserCompanyMatch, error)
	Upsert(f *models.UserMatchFeedback) error
	Delete(userID, matchID uint) error
	VerdictsByMatchIDs(userID uint, matchIDs []uint) (map[uint]string, error)
}

// MatchFeedbackService は、推薦への「合う／合わない」を記録する（SOCAIAGENT-670）。
// 精度の評価（「参考」の判定・職種の加点が当たっているか）に使うため、押した時点の適合度などを写しで残す。
type MatchFeedbackService struct {
	store MatchFeedbackStore
}

func NewMatchFeedbackService(store MatchFeedbackStore) *MatchFeedbackService {
	return &MatchFeedbackService{store: store}
}

// Submit は、本人の推薦への判定を保存する。verdict が none なら取り消す。他人の推薦には付けられない。
func (s *MatchFeedbackService) Submit(userID, matchID uint, verdict string) error {
	if verdict != MatchFeedbackNone && !models.IsValidMatchFeedbackVerdict(verdict) {
		return ErrInvalidFeedbackVerdict
	}
	match, err := s.store.FindMatch(matchID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return shared.ErrNotFound
		}
		return err
	}
	if match.UserID != userID {
		return shared.ErrForbidden
	}
	if verdict == MatchFeedbackNone {
		return s.store.Delete(userID, matchID)
	}
	return s.store.Upsert(&models.UserMatchFeedback{
		UserID:          userID,
		MatchID:         matchID,
		Verdict:         verdict,
		MatchScore:      match.MatchScore,
		FitMismatchAxes: match.FitMismatchAxes,
		JobMatched:      match.JobMatched,
	})
}

// Verdicts は、本人の判定を推薦 ID から引く。読めなくても推薦の表示は止めない（空を返す）。
func (s *MatchFeedbackService) Verdicts(userID uint, matchIDs []uint) map[uint]string {
	out, err := s.store.VerdictsByMatchIDs(userID, matchIDs)
	if err != nil {
		return map[uint]string{}
	}
	return out
}
