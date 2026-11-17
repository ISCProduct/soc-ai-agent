package chat

import (
	"context"
	"fmt"
	"sync"

	internalOpenAI "Backend/internal/openai"
	"Backend/internal/services/piimask"
)

// withPIIGuard は、このリクエストで行う外部 AI 呼び出し（回答の評価・妥当性判定・質問生成・要約・埋め込み）の
// ユーザー入力へ、送る前に伏字をかけるようにする。学生の登録情報（氏名・学校名・メール）から辞書を作る。
//
// 辞書は最初に使うときに一度だけ作る。作れなかったら（登録情報を読めないなど）、外部へ送らない（fail-closed）。
// 未ログイン（userID==0）や userRepo が無いときは、登録情報なしの規則だけで伏字にする。
func (s *ChatService) withPIIGuard(ctx context.Context, userID uint) context.Context {
	var (
		once   sync.Once
		masker *piimask.Masker
		merr   error
	)
	return internalOpenAI.WithInputGuard(ctx, func(text string) (string, error) {
		once.Do(func() { masker, merr = s.piiMaskerFor(userID) })
		if merr != nil {
			return "", merr
		}
		return masker.Mask(text), nil
	})
}

func (s *ChatService) piiMaskerFor(userID uint) (*piimask.Masker, error) {
	var subj piimask.Subject
	if s.userRepo != nil && userID != 0 {
		u, err := s.userRepo.GetUserByID(userID)
		if err != nil {
			return nil, fmt.Errorf("load user for masking: %w", err)
		}
		if u != nil {
			subj = piimask.Subject{Name: u.Name, Email: u.Email, SchoolName: u.SchoolName}
		}
	}
	return piimask.ForLLM(subj)
}
