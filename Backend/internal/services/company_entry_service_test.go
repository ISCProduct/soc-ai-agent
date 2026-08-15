package services

import (
	"strings"
	"testing"

	"Backend/internal/services/email"
)

func TestCompanyEntryService_SubmitValidation(t *testing.T) {
	svc := &CompanyEntryService{}

	tests := []struct {
		name    string
		in      CompanyEntryInput
		wantErr string
	}{
		{
			name:    "企業名必須",
			in:      CompanyEntryInput{ContactEmail: "hr@example.com", PrivacyConsent: true},
			wantErr: "name is required",
		},
		{
			name:    "メール必須",
			in:      CompanyEntryInput{Name: "テスト株式会社", PrivacyConsent: true},
			wantErr: "contact_email is required",
		},
		{
			name: "メール形式不正",
			in: CompanyEntryInput{
				Name: "テスト株式会社", ContactEmail: "not-an-email", PrivacyConsent: true,
			},
			wantErr: "contact_email is invalid",
		},
		{
			name: "同意必須",
			in: CompanyEntryInput{
				Name: "テスト株式会社", ContactEmail: "hr@example.com",
			},
			wantErr: "privacy_consent is required",
		},
		{
			name: "ハニーポット拒否",
			in: CompanyEntryInput{
				Name: "テスト株式会社", ContactEmail: "hr@example.com", PrivacyConsent: true, Honeypot: "bot",
			},
			wantErr: "rejected",
		},
		{
			name: "求人件数上限",
			in: CompanyEntryInput{
				Name:           "テスト株式会社",
				ContactEmail:   "hr@example.com",
				PrivacyConsent: true,
				JobPositions:   make([]CompanyEntryJobInput, maxEntryJobPositions+1),
			},
			wantErr: "job_positions must be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Submit(tt.in)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestEmailService_SendCompanyEntryThankYouAndInvite_LogFallback(t *testing.T) {
	svc := &email.EmailService{} // host empty → log only
	if err := svc.SendCompanyEntryThankYouAndInvite("hr@example.com", "テスト株式会社", "token123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.SendCompanyEntryThankYouAndInvite("hr@example.com", "", ""); err != nil {
		t.Fatalf("unexpected error without token: %v", err)
	}
}
