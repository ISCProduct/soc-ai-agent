package costs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"Backend/internal/services/shared"
)

// ErrInterviewBudgetExceeded は月次上限に達したことを表す。
var ErrInterviewBudgetExceeded = errors.New("interview monthly budget exceeded")

// InterviewBudgetStatus は当月の面接回数と残枠。
type InterviewBudgetStatus struct {
	Month     string `json:"month"`
	Count     int64  `json:"count"`
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
	Enforce   bool   `json:"enforce"`
	Exceeded  bool   `json:"exceeded"`
}

// interviewSessionCounter は面接回数の読み出し面。
type interviewSessionCounter interface {
	CountSince(since time.Time) (int64, error)
}

// InterviewBudgetService は AI面接の月次上限とアラートを担当する。
//
// 面接1回ごとに STT / LLM / TTS を叩くため、セッション数がそのまま費用に効く。
// 要件定義書の試算では「100人×毎日」で月 $180〜360 になり、
// 学校向けの月額予算を大きく超えるが、これまで上限の仕組みが無かった。
//
// **既定では止めない（enforce=false）。** 実際の利用量が見えないまま
// 遮断すると、面接練習をしたい学生を理由も分からず弾いてしまう。
// まず記録と通知だけ行い、実測を見てから INTERVIEW_BUDGET_ENFORCE=true にする。
type InterviewBudgetService struct {
	repo           interviewSessionCounter
	limit          int64
	alertThreshold int64
	enforce        bool
	notify         func(InterviewBudgetStatus)

	mu               sync.Mutex
	lastAlertMonthID string
}

// defaultInterviewMonthlyLimit は月次の面接回数の既定上限。
//
// 面接1回あたり約 $0.035（STTが約6割、`RESULTS_fallback.md`）として、
// 月 $50 の予算のうち面接に $35 を割く想定で 1000 回。
// 費用の内訳は未実測なので、実測が出たら見直すこと。
const defaultInterviewMonthlyLimit = 1000

func NewInterviewBudgetService(repo interviewSessionCounter, notify func(InterviewBudgetStatus)) *InterviewBudgetService {
	limit := int64(shared.GetIntEnv("INTERVIEW_MONTHLY_LIMIT", defaultInterviewMonthlyLimit))
	if limit <= 0 {
		limit = defaultInterviewMonthlyLimit
	}
	// 既定は上限の8割で通知する。超えてから気づくのでは遅い
	alert := int64(shared.GetIntEnv("INTERVIEW_ALERT_THRESHOLD", int(limit*8/10)))
	if alert <= 0 || alert > limit {
		alert = limit
	}
	enforce := false
	if v := strings.TrimSpace(os.Getenv("INTERVIEW_BUDGET_ENFORCE")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			enforce = b
		}
	}
	return &InterviewBudgetService{
		repo:           repo,
		limit:          limit,
		alertThreshold: alert,
		enforce:        enforce,
		notify:         notify,
	}
}

// AllowStart は面接を開始してよいかを返す。
//
// 上限内なら nil。超過していても enforce が false なら nil を返し、ログだけ残す。
// 集計に失敗したときも nil を返す。**監視の失敗で面接を止めない。**
func (s *InterviewBudgetService) AllowStart() error {
	if s == nil || s.repo == nil {
		return nil
	}
	status, err := s.Status()
	if err != nil {
		log.Printf("[InterviewBudget] status check failed: %v", err)
		return nil
	}
	if status.Count >= s.alertThreshold {
		s.notifyIfNeeded(status)
	}
	if status.Count >= s.limit {
		if !s.enforce {
			log.Printf("[InterviewBudget] OVER (monitor only) month=%s count=%d limit=%d",
				status.Month, status.Count, s.limit)
			return nil
		}
		log.Printf("[InterviewBudget] DENY month=%s count=%d limit=%d", status.Month, status.Count, s.limit)
		return ErrInterviewBudgetExceeded
	}
	return nil
}

// Status は当月の面接回数と残枠を返す。
func (s *InterviewBudgetService) Status() (InterviewBudgetStatus, error) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	count, err := s.repo.CountSince(monthStart)
	if err != nil {
		return InterviewBudgetStatus{}, err
	}
	remaining := s.limit - count
	if remaining < 0 {
		remaining = 0
	}
	return InterviewBudgetStatus{
		Month:     now.Format("2006-01"),
		Count:     count,
		Limit:     s.limit,
		Remaining: remaining,
		Enforce:   s.enforce,
		Exceeded:  count >= s.limit,
	}, nil
}

// notifyIfNeeded は同じ月に何度も通知しない。
func (s *InterviewBudgetService) notifyIfNeeded(status InterviewBudgetStatus) {
	if s.notify == nil {
		return
	}
	s.mu.Lock()
	already := s.lastAlertMonthID == status.Month
	if !already {
		s.lastAlertMonthID = status.Month
	}
	s.mu.Unlock()
	if already {
		return
	}
	s.notify(status)
}

// NotifyInterviewBudgetToDiscord は面接の月次上限アラートをDiscordへ送る。
//
// 既存の企業検索アラートと同じWebhookを使う。通知先を増やすより、
// 費用の話が1か所に集まる方が気づかれやすい。
func NotifyInterviewBudgetToDiscord(status InterviewBudgetStatus) {
	webhook := strings.TrimSpace(os.Getenv("COMPANY_SEARCH_ALERT_DISCORD_WEBHOOK_URL"))
	if webhook == "" {
		webhook = strings.TrimSpace(os.Getenv("OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL"))
	}
	if webhook == "" {
		log.Printf("[InterviewBudget] ALERT month=%s count=%d/%d enforce=%t (webhook未設定)",
			status.Month, status.Count, status.Limit, status.Enforce)
		return
	}

	action := "**上限に達しても面接は止めません**（監視のみ）。止めるには INTERVIEW_BUDGET_ENFORCE=true を設定してください。"
	if status.Enforce {
		action = "**上限に達すると面接を開始できなくなります。**"
	}
	text := fmt.Sprintf("AI面接の月次利用が閾値に達しました\n%s: %d / %d 回（残り %d）\n%s",
		status.Month, status.Count, status.Limit, status.Remaining, action)

	if err := postCompanySearchDiscordAlert(webhook, text); err != nil {
		log.Printf("[InterviewBudget] discord alert failed: %v", err)
	}
}
