package interview

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/usagectx"
)

// countingAIServer は AI（STT/LLM/TTS）が呼ばれた回数を数える。
func countingAIServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":"回答です"}`))
		case "/audio/speech":
			_, _ = w.Write([]byte("audio"))
		default:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"次の質問です"}}]}`))
		}
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

func nUtterances(n int) []models.InterviewUtterance {
	u := make([]models.InterviewUtterance, n)
	for i := range n {
		u[i] = models.InterviewUtterance{SessionID: 1, Role: "ai", Text: fmt.Sprintf("発話%d", i)}
	}
	return u
}

// TestTurnLimits は /turn・/start-turn の入口で、発話数・経過時間・予算を AI を呼ぶ前に
// 判定することを固定する（SOCAIAGENT-696）。上限時は AI を呼ばず、固定の締め文と
// LimitReached を返す（FE はこれを受けて既存の終了経路でレポートを作る）。
func TestTurnLimits(t *testing.T) {
	budgetErr := func(err error) func(context.Context) error {
		return func(context.Context) error { return err }
	}
	tests := []struct {
		name       string
		status     string
		startedAgo time.Duration // 0 なら StartedAt なし
		saved      int
		env        map[string]string
		budget     func(context.Context) error
		wantErr    error
		wantLimit  string // 空なら通常のターン
	}{
		{name: "通常のターンは AI を呼ぶ", status: "in_progress", startedAgo: time.Minute, saved: 2},
		{name: "発話数が上限に達したら止める", status: "in_progress", startedAgo: time.Minute, saved: maxUtterancesPerSession, wantLimit: limitReasonUtterances},
		{name: "StartedAt から45分で止める", status: "in_progress", startedAgo: 46 * time.Minute, saved: 2, wantLimit: limitReasonDuration},
		{name: "44分ならまだ続ける", status: "in_progress", startedAgo: 44 * time.Minute, saved: 2},
		{name: "発話数の上限は環境変数で変えられる", status: "in_progress", startedAgo: time.Minute, saved: 4,
			env: map[string]string{"INTERVIEW_MAX_UTTERANCES": "4"}, wantLimit: limitReasonUtterances},
		{name: "時間の上限は環境変数で変えられる", status: "in_progress", startedAgo: 11 * time.Minute, saved: 2,
			env: map[string]string{"INTERVIEW_TURN_MAX_MINUTES": "10"}, wantLimit: limitReasonDuration},
		{name: "予算超過は既定ではログのみで続ける(monitor)", status: "in_progress", startedAgo: time.Minute, saved: 2,
			budget: budgetErr(usagectx.ErrAIBudgetExceeded)},
		{name: "予算超過はフラグがオンなら止める", status: "in_progress", startedAgo: time.Minute, saved: 2,
			env: map[string]string{"INTERVIEW_TURN_BUDGET_ENFORCE": "true"}, budget: budgetErr(usagectx.ErrAIBudgetExceeded), wantLimit: limitReasonBudget},
		{name: "予算状態が読めないときはフラグがオンでも続ける(fail-open)", status: "in_progress", startedAgo: time.Minute, saved: 2,
			env: map[string]string{"INTERVIEW_TURN_BUDGET_ENFORCE": "true"}, budget: budgetErr(usagectx.ErrAIBudgetUnavailable)},
		{name: "開始前(ready)のセッションは断る", status: "ready", saved: 0, wantErr: ErrSessionNotStarted},
	}

	for _, tt := range tests {
		for _, route := range []string{"turn", "start-turn"} {
			t.Run(tt.name+"/"+route, func(t *testing.T) {
				for k, v := range tt.env {
					t.Setenv(k, v)
				}
				server, hits := countingAIServer(t)
				session := &models.InterviewSession{ID: 1, UserID: 10, Status: tt.status, Language: "ja"}
				if tt.startedAgo > 0 {
					started := time.Now().Add(-tt.startedAgo)
					session.StartedAt = &started
				}
				svc := NewInterviewService(newSessionRepoStub(session), &utterRepoStub{utterances: nUtterances(tt.saved)},
					nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil)
				ctx := context.Background()
				if tt.budget != nil {
					ctx = usagectx.WithAIBudgetCheck(ctx, tt.budget)
				}

				var result *TurnResult
				var err error
				if route == "turn" {
					result, err = svc.Turn(ctx, 10, 1, []byte("audio"), "", "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
				} else {
					result, err = svc.StartTurn(ctx, 10, 1, "", "", "", "", "", 0, 1, 1, 0, 0)
				}

				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("err = %v, want %v", err, tt.wantErr)
					}
					if hits.Load() != 0 {
						t.Fatalf("AI が %d 回呼ばれた", hits.Load())
					}
					return
				}
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if tt.wantLimit == "" {
					if result.LimitReached || hits.Load() == 0 {
						t.Fatalf("通常のターンのはず: limit=%v AI呼び出し=%d", result.LimitReached, hits.Load())
					}
					return
				}
				if !result.LimitReached || result.LimitReason != tt.wantLimit {
					t.Fatalf("LimitReached=%v reason=%q, want %q", result.LimitReached, result.LimitReason, tt.wantLimit)
				}
				if result.AIText != interviewLimitClosingText || len(result.Audio) != 0 {
					t.Fatalf("固定の締め文・音声なしで返すこと: %+v", result)
				}
				if hits.Load() != 0 {
					t.Fatalf("上限時に AI が %d 回呼ばれた", hits.Load())
				}
			})
		}
	}
}
