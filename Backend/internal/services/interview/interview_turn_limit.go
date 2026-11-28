package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"Backend/internal/usagectx"
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrSessionNotStarted は /start 前（ready）のセッションへのターン。コントローラーが 409 にする。
var ErrSessionNotStarted = errors.New("session not started")

// interviewLimitClosingText は上限に達したときの固定の締め文。AI を使わない（費用0）。
const interviewLimitClosingText = "本日の面接はここまでとさせていただきます。お時間をいただき、ありがとうございました。"

// 上限の理由。ログと FE への応答（limit_reason）に使う。
const (
	limitReasonUtterances = "utterances"
	limitReasonDuration   = "duration"
	limitReasonBudget     = "budget"
)

// defaultTurnMaxMinutes は StartedAt からの経過時間の上限（分）の既定値。
const defaultTurnMaxMinutes = 45

// utteranceLimit は1セッションの発話数の上限。INTERVIEW_MAX_UTTERANCES で変えられる（既定200）。
// ターン入口の判定と保存時の上限で同じ値を使う。
func utteranceLimit() int {
	if n := shared.GetIntEnv("INTERVIEW_MAX_UTTERANCES", maxUtterancesPerSession); n > 0 {
		return n
	}
	return maxUtterancesPerSession
}

// turnMaxDuration は StartedAt からの経過時間の上限。INTERVIEW_TURN_MAX_MINUTES で変えられる（既定45分）。
func turnMaxDuration() time.Duration {
	n := shared.GetIntEnv("INTERVIEW_TURN_MAX_MINUTES", defaultTurnMaxMinutes)
	if n <= 0 {
		n = defaultTurnMaxMinutes
	}
	return time.Duration(n) * time.Minute
}

// turnBudgetEnforced はターンで予算超過を止めるか。既定はログのみ（monitor）。
// 実測を見てから INTERVIEW_TURN_BUDGET_ENFORCE=true にする。
func turnBudgetEnforced() bool {
	b, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("INTERVIEW_TURN_BUDGET_ENFORCE")))
	return err == nil && b
}

// turnLimitReason は AI を呼ぶ前に、発話数・経過時間・予算の上限を判定する（SOCAIAGENT-696）。
// 判定はサーバーの値だけで行い、クライアントの turn_count・remaining_seconds は使わない。
// adding はこのターンで保存する発話数。空文字なら上限内。
func (s *InterviewService) turnLimitReason(ctx context.Context, session *models.InterviewSession, adding int) string {
	if s.utterRepo != nil {
		// 読めないときは止めない（保存時の上限が最後の砦になる）
		if existing, err := s.utterRepo.FindBySessionID(session.ID); err == nil && len(existing)+adding > utteranceLimit() {
			return limitReasonUtterances
		}
	}
	if session.StartedAt != nil && time.Since(*session.StartedAt) >= turnMaxDuration() {
		return limitReasonDuration
	}
	if err := usagectx.CheckAIBudget(ctx); err != nil {
		if !errors.Is(err, usagectx.ErrAIBudgetExceeded) {
			// 予算状態が読めないときは止めない（障害時に面接が全停止しないよう fail-open）
			log.Printf("[Interview] turn budget check unavailable session=%d: %v", session.ID, err)
			return ""
		}
		enforce := turnBudgetEnforced()
		log.Printf("[Interview] turn budget exceeded session=%d user=%d enforce=%t", session.ID, session.UserID, enforce)
		if enforce {
			return limitReasonBudget
		}
	}
	return ""
}

// checkTurnStart はターンの入口判定。ready は ErrSessionNotStarted、上限時は締めの結果を返す。
func (s *InterviewService) checkTurnStart(ctx context.Context, session *models.InterviewSession, adding int) (*TurnResult, error) {
	if session.Status == "ready" {
		return nil, ErrSessionNotStarted
	}
	reason := s.turnLimitReason(ctx, session, adding)
	if reason == "" {
		return nil, nil
	}
	log.Printf("[Interview] turn limit reached session=%d user=%d reason=%s", session.ID, session.UserID, reason)
	return &TurnResult{AIText: interviewLimitClosingText, LimitReached: true, LimitReason: reason}, nil
}
