package email

import (
	"os"
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/services/analysis"
)

func TestResolveEmailProvider_AutoSelection(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "")
	t.Setenv("EMAIL_FROM", "from@example.com")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("RESEND_FROM", "")

	svc := NewEmailService()
	if got := svc.Provider(); got != "log" {
		t.Fatalf("expected log, got %s", got)
	}

	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "smtp@example.com")
	svc = NewEmailService()
	if got := svc.Provider(); got != "smtp" {
		t.Fatalf("expected smtp, got %s", got)
	}

	t.Setenv("RESEND_API_KEY", "re_test_key")
	t.Setenv("EMAIL_FROM", "resend@example.com")
	svc = NewEmailService()
	if got := svc.Provider(); got != "resend" {
		t.Fatalf("expected resend, got %s", got)
	}

	t.Setenv("EMAIL_PROVIDER", "log")
	svc = NewEmailService()
	if got := svc.Provider(); got != "log" {
		t.Fatalf("explicit log expected, got %s", got)
	}
}

func TestEmailService_SendRegistrationEmail_UsesTransport(t *testing.T) {
	rec := &recordingMailTransport{}
	svc := &EmailService{
		provider:  "recording",
		from:      "noreply@example.com",
		transport: rec,
	}
	t.Setenv("FRONTEND_URL", "http://localhost:3000")

	if err := svc.SendRegistrationEmail("user@example.com", "tok123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(rec.messages))
	}
	msg := rec.messages[0]
	if msg.Subject != "会員登録の確認" {
		t.Errorf("subject = %q", msg.Subject)
	}
	if len(msg.To) != 1 || msg.To[0] != "user@example.com" {
		t.Errorf("to = %#v", msg.To)
	}
	if !strings.Contains(msg.HTML, "/register/confirm?token=tok123") {
		t.Errorf("html missing confirm link: %s", msg.HTML)
	}
	if msg.From != "noreply@example.com" {
		t.Errorf("from = %q", msg.From)
	}
}

func TestEmailService_SendSystemAlertEmail_TextBody(t *testing.T) {
	rec := &recordingMailTransport{}
	svc := &EmailService{provider: "recording", from: "ops@example.com", transport: rec}
	if err := svc.SendSystemAlertEmail([]string{"a@example.com", "b@example.com"}, "alert", "budget exceeded"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.messages) != 1 {
		t.Fatalf("expected 1 message")
	}
	if rec.messages[0].Text != "budget exceeded" {
		t.Errorf("text = %q", rec.messages[0].Text)
	}
	if rec.messages[0].HTML != "" {
		t.Errorf("html should be empty for alert")
	}
}

func TestEmailService_SendAnalysisReport_HTML(t *testing.T) {
	rec := &recordingMailTransport{}
	svc := &EmailService{provider: "recording", from: "noreply@example.com", transport: rec}
	user := &entity.User{Name: "太郎", Email: "taro@example.com"}
	summary := &analysis.AnalysisSummary{
		Scores:   analysis.AnalysisScores{JobScore: 0.8, InterestScore: 0.7, AptitudeScore: 0.6, FutureScore: 0.5},
		Progress: analysis.AnalysisProgress{Job: 0.8, Interest: 0.7, Aptitude: 0.6, Future: 0.5},
	}
	if err := svc.SendAnalysisReport(user, summary, nil, "sess-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.messages) != 1 || !strings.Contains(rec.messages[0].HTML, "AI就活分析レポート") {
		t.Fatalf("unexpected message: %#v", rec.messages)
	}
}

func TestNewEmailService_ResendWithoutKeyFallsBackToLog(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "resend")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("EMAIL_FROM", "from@example.com")
	// clear other selectors
	_ = os.Unsetenv("SMTP_HOST")
	svc := NewEmailService()
	if svc.Provider() != "log" {
		t.Fatalf("expected log fallback, got %s", svc.Provider())
	}
}
