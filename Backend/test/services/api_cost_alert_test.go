package services_test

// 実行: cd Backend && go test ./test/services/... -run 'TestAPICostAlert|TestNewAPICostService' -v

import (
	"strings"
	"sync"
	"testing"

	"Backend/internal/services"
)

func TestAPICostAlert_ThresholdAndMonthlyDedupe(t *testing.T) {
	tests := []struct {
		name         string
		threshold    float64
		total        float64
		monthID      string
		priorMonthID string
		wantNotified bool
		wantSlack    int
		wantDiscord  int
	}{
		{
			name:         "閾値未満は通知しない",
			threshold:    40,
			total:        39.99,
			monthID:      "2026-07",
			wantNotified: false,
		},
		{
			name:         "閾値ちょうどは通知しない",
			threshold:    40,
			total:        40.0,
			monthID:      "2026-07",
			wantNotified: false,
		},
		{
			name:         "閾値超過で Slack/Discord に通知",
			threshold:    40,
			total:        40.01,
			monthID:      "2026-07",
			wantNotified: true,
			wantSlack:    1,
			wantDiscord:  1,
		},
		{
			name:         "同一月の再チェックは重複通知しない",
			threshold:    40,
			total:        55,
			monthID:      "2026-07",
			priorMonthID: "2026-07",
			wantNotified: false,
		},
		{
			name:         "月が変われば再通知できる",
			threshold:    40,
			total:        45,
			monthID:      "2026-08",
			priorMonthID: "2026-07",
			wantNotified: true,
			wantSlack:    1,
			wantDiscord:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				mu           sync.Mutex
				slackCalls   int
				discordCalls int
				lastSlack    string
			)

			svc := services.NewAPICostService(nil)
			svc.SetAlertHooksForTest(tt.threshold,
				func(text string) error {
					mu.Lock()
					defer mu.Unlock()
					slackCalls++
					lastSlack = text
					return nil
				},
				func(content string) error {
					mu.Lock()
					defer mu.Unlock()
					discordCalls++
					return nil
				},
			)

			if tt.priorMonthID != "" {
				_ = svc.NotifyIfMonthCostExceeded(tt.threshold+1, tt.priorMonthID)
				mu.Lock()
				slackCalls = 0
				discordCalls = 0
				mu.Unlock()
			}

			got := svc.NotifyIfMonthCostExceeded(tt.total, tt.monthID)
			if got != tt.wantNotified {
				t.Fatalf("notified=%v want=%v", got, tt.wantNotified)
			}

			mu.Lock()
			defer mu.Unlock()
			if slackCalls != tt.wantSlack {
				t.Fatalf("slack calls=%d want=%d", slackCalls, tt.wantSlack)
			}
			if discordCalls != tt.wantDiscord {
				t.Fatalf("discord calls=%d want=%d", discordCalls, tt.wantDiscord)
			}
			if tt.wantNotified {
				for _, part := range []string{tt.monthID, "total_usd", "threshold_usd"} {
					if !strings.Contains(lastSlack, part) {
						t.Fatalf("alert body missing %q: %q", part, lastSlack)
					}
				}
			}
		})
	}
}

func TestAPICostAlert_WebhookUnsetDoesNotPanic(t *testing.T) {
	t.Setenv("OPENAI_COST_ALERT_SLACK_WEBHOOK_URL", "")
	t.Setenv("OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL", "")
	t.Setenv("REALTIME_ALERT_SLACK_WEBHOOK_URL", "")

	svc := services.NewAPICostService(nil)
	svc.SetAlertHooksForTest(40, nil, nil)

	if !svc.NotifyIfMonthCostExceeded(50, "2026-07") {
		t.Fatal("expected first exceed to return true (log-only when webhooks unset)")
	}
	if svc.NotifyIfMonthCostExceeded(60, "2026-07") {
		t.Fatal("expected same-month dedupe")
	}
}

func TestNewAPICostService_DefaultThreshold40(t *testing.T) {
	t.Setenv("OPENAI_COST_ALERT_THRESHOLD_USD", "")
	t.Setenv("API_COST_ALERT_THRESHOLD_USD", "")
	svc := services.NewAPICostService(nil)
	if got := svc.AlertThresholdUSD(); got != 40 {
		t.Fatalf("default threshold=%v want=40", got)
	}
}

func TestNewAPICostService_OpenAIEnvOverrides(t *testing.T) {
	t.Setenv("OPENAI_COST_ALERT_THRESHOLD_USD", "25.5")
	t.Setenv("API_COST_ALERT_THRESHOLD_USD", "100")
	svc := services.NewAPICostService(nil)
	if got := svc.AlertThresholdUSD(); got != 25.5 {
		t.Fatalf("threshold=%v want=25.5", got)
	}
}

func TestNewAPICostService_LegacyEnvFallback(t *testing.T) {
	t.Setenv("OPENAI_COST_ALERT_THRESHOLD_USD", "")
	t.Setenv("API_COST_ALERT_THRESHOLD_USD", "80")
	svc := services.NewAPICostService(nil)
	if got := svc.AlertThresholdUSD(); got != 80 {
		t.Fatalf("threshold=%v want=80", got)
	}
}
