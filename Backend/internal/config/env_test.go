package config

import "testing"

func TestEnvGetters(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("RAG_REVIEW_URL", "  http://rag:9000 \n")
	t.Setenv("USER_SECRET", " u\n")
	t.Setenv("ADMIN_SECRET", " a\n")
	t.Setenv("TOKEN_ENCRYPTION_KEY", " k\n")
	t.Setenv("FRONTEND_URL", " http://fe\n")

	if !IsProduction() {
		t.Error("APP_ENV=production で IsProduction が false")
	}
	if got := RAGReviewURL(); got != "http://rag:9000" {
		t.Errorf("RAGReviewURL = %q, 前後の空白が除かれるべき", got)
	}
	for name, got := range map[string]string{
		"u": UserSecret(), "a": AdminSecret(), "k": TokenEncryptionKey(), "http://fe": FrontendURL(),
	} {
		if got != name {
			t.Errorf("getter の値 = %q, want %q", got, name)
		}
	}

	t.Setenv("APP_ENV", "staging")
	if IsProduction() {
		t.Error("APP_ENV=staging で IsProduction が true")
	}
}

func TestAlertWebhookFallback(t *testing.T) {
	t.Setenv("OPENAI_COST_ALERT_SLACK_WEBHOOK_URL", "")
	t.Setenv("REALTIME_ALERT_SLACK_WEBHOOK_URL", " http://rt ")
	if got := OpenAICostAlertSlackWebhookURL(); got != "http://rt" {
		t.Errorf("OpenAI 用が未設定なら Realtime 用に倒れるべき: %q", got)
	}
	t.Setenv("OPENAI_COST_ALERT_SLACK_WEBHOOK_URL", "http://oa")
	if got := OpenAICostAlertSlackWebhookURL(); got != "http://oa" {
		t.Errorf("OpenAI 用が優先されるべき: %q", got)
	}
}

func TestBudgetEnforce(t *testing.T) {
	tests := []struct {
		name, val string
		company   bool
		interview bool
	}{
		{"未設定は既定値", "", true, false},
		{"true", "true", true, true},
		{"false", " false ", false, false},
		{"不正値は既定値", "yes-please", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COMPANY_SEARCH_BUDGET_ENFORCE", tt.val)
			t.Setenv("INTERVIEW_BUDGET_ENFORCE", tt.val)
			if got := CompanySearchBudgetEnforce(); got != tt.company {
				t.Errorf("CompanySearchBudgetEnforce = %v, want %v", got, tt.company)
			}
			if got := InterviewBudgetEnforce(); got != tt.interview {
				t.Errorf("InterviewBudgetEnforce = %v, want %v", got, tt.interview)
			}
		})
	}
}
