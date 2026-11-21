package interview

import (
	"context"
	"fmt"

	"Backend/internal/openai"
	"Backend/internal/services/piimask"
)

// withPIIGuard は、このコンテキストで行う外部 AI 呼び出し（面接官の応答・フォローアップ・フレーズ提案・レポート）の
// ユーザー入力（学生の発話）へ、送る前に伏字をかけるようにする。学生の登録情報（氏名・学校名・メール）から辞書を作る。
//
// 辞書は最初に使うときに一度だけ作る。作れなかったら外部へ送らない（fail-closed）。
//
// 対象外（意図的）:
//   - 面接官のシステムプロンプトに入れる登録氏名（面接官が名前で呼びかけるための機能）。
//   - 音声そのもの。文字起こし前の音声は伏字にできない（方針は別途）。
func (s *InterviewService) withPIIGuard(ctx context.Context, userID uint) context.Context {
	return openai.WithInputGuard(ctx, piimask.LazyGuard(func() (piimask.Subject, bool, error) {
		if s.userRepo == nil || userID == 0 {
			return piimask.Subject{}, false, nil
		}
		u, err := s.userRepo.GetUserByID(userID)
		if err != nil {
			return piimask.Subject{}, false, fmt.Errorf("load user for masking: %w", err)
		}
		if u == nil {
			return piimask.Subject{}, false, nil
		}
		return piimask.Subject{Name: u.Name, Email: u.Email, SchoolName: u.SchoolName}, true, nil
	}))
}
