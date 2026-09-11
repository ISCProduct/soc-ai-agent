package costs

import (
	"Backend/internal/companyfetch"
	"Backend/internal/repositories"
	"Backend/internal/services/email"
	"Backend/internal/services/shared"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CompanySearchBudgetStatus は管理画面向けの月次 Search 予算状況。
type CompanySearchBudgetStatus struct {
	Month     string `json:"month"`
	Count     int64  `json:"count"`
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
	Enforce   bool   `json:"enforce"`
	Exceeded  bool   `json:"exceeded"`
}

// CompanySearchBudgetService は企業 FirstTouch / Write Search の月次上限とアラートを担当する。
type CompanySearchBudgetService struct {
	repo           *repositories.APICallLogRepository
	emailService   *email.EmailService
	limit          int64
	alertThreshold int64
	enforce        bool

	mu               sync.Mutex
	lastAlertMonthID string
}

func NewCompanySearchBudgetService(repo *repositories.APICallLogRepository, emailService *email.EmailService) *CompanySearchBudgetService {
	limit := int64(shared.GetIntEnv("COMPANY_SEARCH_MONTHLY_LIMIT", 2000))
	if limit <= 0 {
		limit = 2000
	}
	alert := int64(shared.GetIntEnv("COMPANY_SEARCH_ALERT_THRESHOLD", int(limit)))
	if alert <= 0 {
		alert = limit
	}
	enforce := true
	if v := strings.TrimSpace(os.Getenv("COMPANY_SEARCH_BUDGET_ENFORCE")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			enforce = b
		}
	}
	return &CompanySearchBudgetService{
		repo:           repo,
		emailService:   emailService,
		limit:          limit,
		alertThreshold: alert,
		enforce:        enforce,
	}
}

// AllowSearch は月次予算内なら nil、超過かつ enforce 時は ErrSearchBudgetExceeded。
func (s *CompanySearchBudgetService) AllowSearch() error {
	if s == nil || s.repo == nil {
		return nil
	}
	status, err := s.Status()
	if err != nil {
		log.Printf("[CompanySearchBudget] status check failed: %v", err)
		return nil // 監視失敗で Write を止めない
	}
	if status.Count >= s.alertThreshold {
		s.notifyIfNeeded(status)
	}
	if s.enforce && status.Count >= s.limit {
		log.Printf("[CompanySearchBudget] DENY month=%s count=%d limit=%d", status.Month, status.Count, s.limit)
		return companyfetch.ErrSearchBudgetExceeded
	}
	return nil
}

// Status は当月の企業 Search 回数と残枠を返す。
func (s *CompanySearchBudgetService) Status() (CompanySearchBudgetStatus, error) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	month := now.Format("2006-01")
	count, err := s.repo.CountSearchCallsSince(monthStart)
	if err != nil {
		return CompanySearchBudgetStatus{}, err
	}
	remaining := s.limit - count
	if remaining < 0 {
		remaining = 0
	}
	return CompanySearchBudgetStatus{
		Month:     month,
		Count:     count,
		Limit:     s.limit,
		Remaining: remaining,
		Enforce:   s.enforce,
		Exceeded:  count >= s.limit,
	}, nil
}

func (s *CompanySearchBudgetService) notifyIfNeeded(status CompanySearchBudgetStatus) {
	s.mu.Lock()
	if s.lastAlertMonthID == status.Month {
		s.mu.Unlock()
		return
	}
	s.lastAlertMonthID = status.Month
	s.mu.Unlock()

	subject := fmt.Sprintf("[SOC AI] 企業Search月次予算閾値超過 (%s)", status.Month)
	body := fmt.Sprintf(
		"Company Search monthly budget threshold exceeded.\nmonth=%s\ncount=%d\nlimit=%d\nalert_threshold=%d\nenforce=%v\n",
		status.Month, status.Count, s.limit, s.alertThreshold, s.enforce,
	)
	log.Printf("[CompanySearchBudget] ALERT %s", strings.ReplaceAll(body, "\n", " "))

	if webhook := strings.TrimSpace(os.Getenv("COMPANY_SEARCH_ALERT_SLACK_WEBHOOK_URL")); webhook != "" {
		if err := postCompanySearchSlackAlert(webhook, subject+"\n"+body); err != nil {
			log.Printf("[CompanySearchBudget] slack alert failed: %v", err)
		}
	} else if webhook := strings.TrimSpace(os.Getenv("REALTIME_ALERT_SLACK_WEBHOOK_URL")); webhook != "" {
		if err := postCompanySearchSlackAlert(webhook, subject+"\n"+body); err != nil {
			log.Printf("[CompanySearchBudget] slack alert (realtime webhook) failed: %v", err)
		}
	}

	if s.emailService != nil {
		recipients := parseEmailsEnv("COMPANY_SEARCH_ALERT_EMAILS")
		if len(recipients) == 0 {
			recipients = parseEmailsEnv("REALTIME_ALERT_EMAILS")
		}
		if len(recipients) > 0 {
			if err := s.emailService.SendSystemAlertEmail(recipients, subject, body); err != nil {
				log.Printf("[CompanySearchBudget] email alert failed: %v", err)
			}
		}
	}

	if webhook := strings.TrimSpace(os.Getenv("COMPANY_SEARCH_ALERT_DISCORD_WEBHOOK_URL")); webhook != "" {
		if err := postCompanySearchDiscordAlert(webhook, subject+"\n"+body); err != nil {
			log.Printf("[CompanySearchBudget] discord alert failed: %v", err)
		}
	} else if webhook := strings.TrimSpace(os.Getenv("OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL")); webhook != "" {
		if err := postCompanySearchDiscordAlert(webhook, subject+"\n"+body); err != nil {
			log.Printf("[CompanySearchBudget] discord alert (openai cost webhook) failed: %v", err)
		}
	}
}

func postCompanySearchSlackAlert(webhookURL, text string) error {
	payload := map[string]string{"text": text}
	b, _ := json.Marshal(payload)
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
		return fmt.Errorf("slack webhook status=%d", resp.StatusCode)
	}
	return nil
}

func postCompanySearchDiscordAlert(webhookURL, text string) error {
	// Discord Incoming Webhook: content 最大 2000 文字
	if len(text) > 1900 {
		text = text[:1900] + "…"
	}
	payload := map[string]string{"content": text}
	b, _ := json.Marshal(payload)
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
		return fmt.Errorf("discord webhook status=%d", resp.StatusCode)
	}
	return nil
}
