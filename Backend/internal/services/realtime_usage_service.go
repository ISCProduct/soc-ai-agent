package services

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// TokenUsage は Realtime API の response.done イベントから取得するトークン使用量。
// nil のとき時間ベースにフォールバックする。
type TokenUsage struct {
	InputTextTokens        int
	OutputTextTokens       int
	InputAudioTokens       int
	OutputAudioTokens      int
	InputCachedAudioTokens int
}

// realtimeTokenRates はトークン単価（USD/1Mトークン）を保持する。
type realtimeTokenRates struct {
	textInput        float64
	textOutput       float64
	audioInput       float64
	audioOutput      float64
	cachedAudioInput float64
}

func loadTokenRates() realtimeTokenRates {
	return realtimeTokenRates{
		textInput:        getFloatEnv("REALTIME_TEXT_INPUT_COST_PER_1M_USD", 5.0),
		textOutput:       getFloatEnv("REALTIME_TEXT_OUTPUT_COST_PER_1M_USD", 15.0),
		audioInput:       getFloatEnv("REALTIME_AUDIO_INPUT_COST_PER_1M_USD", 100.0),
		audioOutput:      getFloatEnv("REALTIME_AUDIO_OUTPUT_COST_PER_1M_USD", 200.0),
		cachedAudioInput: getFloatEnv("REALTIME_CACHED_AUDIO_INPUT_COST_PER_1M_USD", 20.0),
	}
}

func (r realtimeTokenRates) calcCost(u TokenUsage) float64 {
	const perM = 1_000_000.0
	return float64(u.InputTextTokens)/perM*r.textInput +
		float64(u.OutputTextTokens)/perM*r.textOutput +
		float64(u.InputAudioTokens)/perM*r.audioInput +
		float64(u.OutputAudioTokens)/perM*r.audioOutput +
		float64(u.InputCachedAudioTokens)/perM*r.cachedAudioInput
}

// CalcTokenCost はデフォルト単価を使ってトークンベースのコストを計算する（テスト用公開関数）。
func CalcTokenCost(u TokenUsage) float64 {
	return loadTokenRates().calcCost(u)
}

type RealtimeDailySummary struct {
	Date                       string  `json:"date"`
	TotalCostUSD               float64 `json:"total_cost_usd"`
	TotalDurationSeconds       int64   `json:"total_duration_seconds"`
	SessionCount               int64   `json:"session_count"`
	UserCount                  int64   `json:"user_count"`
	TotalInputAudioTokens      int64   `json:"total_input_audio_tokens"`
	TotalOutputAudioTokens     int64   `json:"total_output_audio_tokens"`
	TotalInputTextTokens       int64   `json:"total_input_text_tokens"`
	TotalOutputTextTokens      int64   `json:"total_output_text_tokens"`
}

type RealtimeMonthlySummary struct {
	Month                      string  `json:"month"`
	TotalCostUSD               float64 `json:"total_cost_usd"`
	TotalDurationSeconds       int64   `json:"total_duration_seconds"`
	SessionCount               int64   `json:"session_count"`
	UserCount                  int64   `json:"user_count"`
	TotalInputAudioTokens      int64   `json:"total_input_audio_tokens"`
	TotalOutputAudioTokens     int64   `json:"total_output_audio_tokens"`
	TotalInputTextTokens       int64   `json:"total_input_text_tokens"`
	TotalOutputTextTokens      int64   `json:"total_output_text_tokens"`
}

type RealtimeUserSummary struct {
	UserID                     uint    `json:"user_id"`
	TotalCostUSD               float64 `json:"total_cost_usd"`
	TotalDurationSeconds       int64   `json:"total_duration_seconds"`
	SessionCount               int64   `json:"session_count"`
	TotalInputAudioTokens      int64   `json:"total_input_audio_tokens"`
	TotalOutputAudioTokens     int64   `json:"total_output_audio_tokens"`
	TotalInputTextTokens       int64   `json:"total_input_text_tokens"`
	TotalOutputTextTokens      int64   `json:"total_output_text_tokens"`
}

type RealtimeUsageService struct {
	repo                   *repositories.RealtimeUsageRepository
	emailService           *EmailService
	alertThresholdUSD      float64
	maxConcurrent          int64
	ratePerMinuteUSD       float64
	sessionDurationMinutes int
	tokenRates             realtimeTokenRates

	mu               sync.Mutex
	lastAlertMonthID string
}

func NewRealtimeUsageService(repo *repositories.RealtimeUsageRepository, emailService *EmailService) *RealtimeUsageService {
	threshold := getFloatEnv("REALTIME_MONTHLY_ALERT_THRESHOLD_USD", 200.0)
	maxConcurrent := int64(getIntEnv("REALTIME_MAX_CONCURRENT_CONNECTIONS", 30))
	if maxConcurrent <= 0 {
		maxConcurrent = 30
	}
	rate := getFloatEnv("INTERVIEW_COST_PER_MIN_USD", 0.18)
	sessionMinutes := getIntEnv("INTERVIEW_SESSION_MINUTES", 10)
	if sessionMinutes <= 0 {
		sessionMinutes = 10
	}
	return &RealtimeUsageService{
		repo:                   repo,
		emailService:           emailService,
		alertThresholdUSD:      threshold,
		maxConcurrent:          maxConcurrent,
		ratePerMinuteUSD:       rate,
		sessionDurationMinutes: sessionMinutes,
		tokenRates:             loadTokenRates(),
	}
}

// SessionDurationMinutes はユーザー向けのセッション時間（分）を返す。
// コスト上限は内部管理のみとし、UXには時間として公開する。
func (s *RealtimeUsageService) SessionDurationMinutes() int {
	return s.sessionDurationMinutes
}

func (s *RealtimeUsageService) CanOpenNewConnection() (bool, int64, int64, error) {
	active, err := s.repo.CountActiveSessions()
	if err != nil {
		return false, 0, s.maxConcurrent, err
	}
	return active < s.maxConcurrent, active, s.maxConcurrent, nil
}

func (s *RealtimeUsageService) EnsureSessionStarted(userID, sessionID uint) error {
	if _, err := s.repo.FindActiveBySessionID(sessionID); err == nil {
		return nil
	}
	entry := &models.RealtimeUsageLog{
		UserID:             userID,
		InterviewSessionID: sessionID,
		Status:             "active",
		StartedAt:          time.Now().UTC(),
	}
	if err := s.repo.Create(entry); err != nil {
		return err
	}
	go s.checkAndNotifyThreshold()
	return nil
}

// CloseSession はセッションを終了し、duration と cost を返す。
// tokens が非 nil の場合はトークンベースでコストを計算し、nil の場合は時間ベースにフォールバックする。
func (s *RealtimeUsageService) CloseSession(sessionID uint, endedAt time.Time, tokens *TokenUsage) (durationSec int64, costUSD float64, err error) {
	entry, err := s.repo.FindActiveBySessionID(sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return -1, 0, nil
		}
		return 0, 0, err
	}
	dur := int64(endedAt.Sub(entry.StartedAt).Seconds())
	if dur < 0 {
		dur = 0
	}

	var cost float64
	if tokens != nil {
		cost = s.tokenRates.calcCost(*tokens)
		entry.InputTextTokens = tokens.InputTextTokens
		entry.OutputTextTokens = tokens.OutputTextTokens
		entry.InputAudioTokens = tokens.InputAudioTokens
		entry.OutputAudioTokens = tokens.OutputAudioTokens
		entry.InputCachedAudioTokens = tokens.InputCachedAudioTokens
		entry.TokenBasedCost = true
	} else {
		cost = (float64(dur) / 60.0) * s.ratePerMinuteUSD
	}

	entry.EndedAt = &endedAt
	entry.DurationSeconds = int(dur)
	entry.CostUSD = cost
	entry.Status = "finished"
	if err := s.repo.Update(entry); err != nil {
		return 0, 0, err
	}
	go s.checkAndNotifyThreshold()
	return dur, cost, nil
}

func (s *RealtimeUsageService) CurrentMonthTotalCost() (float64, error) {
	return s.repo.CurrentMonthTotalCostEstimated(s.ratePerMinuteUSD)
}

func (s *RealtimeUsageService) CurrentActiveCount() (int64, error) {
	return s.repo.CountActiveSessions()
}

func (s *RealtimeUsageService) GetDailyUsage(nDays int) ([]RealtimeDailySummary, error) {
	rows, err := s.repo.DailyUsage(nDays, s.ratePerMinuteUSD)
	if err != nil {
		return nil, err
	}
	out := make([]RealtimeDailySummary, len(rows))
	for i, r := range rows {
		out[i] = RealtimeDailySummary{
			Date:                   r.Date,
			TotalCostUSD:           r.TotalCostUSD,
			TotalDurationSeconds:   r.TotalDurationSeconds,
			SessionCount:           r.SessionCount,
			UserCount:              r.UserCount,
			TotalInputAudioTokens:  r.TotalInputAudioTokens,
			TotalOutputAudioTokens: r.TotalOutputAudioTokens,
			TotalInputTextTokens:   r.TotalInputTextTokens,
			TotalOutputTextTokens:  r.TotalOutputTextTokens,
		}
	}
	return out, nil
}

func (s *RealtimeUsageService) GetMonthlyUsage(nMonths int) ([]RealtimeMonthlySummary, error) {
	rows, err := s.repo.MonthlyUsage(nMonths, s.ratePerMinuteUSD)
	if err != nil {
		return nil, err
	}
	out := make([]RealtimeMonthlySummary, len(rows))
	for i, r := range rows {
		out[i] = RealtimeMonthlySummary{
			Month:                  r.Month,
			TotalCostUSD:           r.TotalCostUSD,
			TotalDurationSeconds:   r.TotalDurationSeconds,
			SessionCount:           r.SessionCount,
			UserCount:              r.UserCount,
			TotalInputAudioTokens:  r.TotalInputAudioTokens,
			TotalOutputAudioTokens: r.TotalOutputAudioTokens,
			TotalInputTextTokens:   r.TotalInputTextTokens,
			TotalOutputTextTokens:  r.TotalOutputTextTokens,
		}
	}
	return out, nil
}

func (s *RealtimeUsageService) GetUserBreakdown(days int, limit int) ([]RealtimeUserSummary, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	rows, err := s.repo.UserBreakdown(since, s.ratePerMinuteUSD, limit)
	if err != nil {
		return nil, err
	}
	out := make([]RealtimeUserSummary, len(rows))
	for i, r := range rows {
		out[i] = RealtimeUserSummary{
			UserID:                 r.UserID,
			TotalCostUSD:           r.TotalCostUSD,
			TotalDurationSeconds:   r.TotalDurationSeconds,
			SessionCount:           r.SessionCount,
			TotalInputAudioTokens:  r.TotalInputAudioTokens,
			TotalOutputAudioTokens: r.TotalOutputAudioTokens,
			TotalInputTextTokens:   r.TotalInputTextTokens,
			TotalOutputTextTokens:  r.TotalOutputTextTokens,
		}
	}
	return out, nil
}

func (s *RealtimeUsageService) checkAndNotifyThreshold() {
	total, err := s.CurrentMonthTotalCost()
	if err != nil {
		return
	}
	if total < s.alertThresholdUSD {
		return
	}
	monthID := time.Now().UTC().Format("2006-01")
	s.mu.Lock()
	if s.lastAlertMonthID == monthID {
		s.mu.Unlock()
		return
	}
	s.lastAlertMonthID = monthID
	s.mu.Unlock()

	subject := fmt.Sprintf("[SOC AI] Realtime月額コスト閾値超過 (%s)", monthID)
	body := fmt.Sprintf(
		"Realtime API monthly cost exceeded threshold.\nmonth=%s\ntotal_usd=%.4f\nthreshold_usd=%.4f\nactive_limit=%d\n",
		monthID, total, s.alertThresholdUSD, s.maxConcurrent,
	)
	log.Printf("[RealtimeUsage] ALERT %s", strings.ReplaceAll(body, "\n", " "))

	if webhook := strings.TrimSpace(os.Getenv("REALTIME_ALERT_SLACK_WEBHOOK_URL")); webhook != "" {
		if err := postSlackAlert(webhook, subject+"\n"+body); err != nil {
			log.Printf("[RealtimeUsage] slack alert failed: %v", err)
		}
	}

	if s.emailService != nil {
		recipients := parseEmailsEnv("REALTIME_ALERT_EMAILS")
		if len(recipients) > 0 {
			if err := s.emailService.SendSystemAlertEmail(recipients, subject, body); err != nil {
				log.Printf("[RealtimeUsage] email alert failed: %v", err)
			}
		}
	}
}

func postSlackAlert(webhookURL, text string) error {
	payload := map[string]string{"text": text}
	return postWebhookJSON(webhookURL, payload)
}

// postDiscordAlert は Discord Incoming Webhook に content を送信する。
func postDiscordAlert(webhookURL, content string) error {
	payload := map[string]string{"content": content}
	return postWebhookJSON(webhookURL, payload)
}

func postWebhookJSON(webhookURL string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d", resp.StatusCode)
	}
	return nil
}

func parseEmailsEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		addr := strings.TrimSpace(p)
		if addr != "" {
			out = append(out, addr)
		}
	}
	return out
}
