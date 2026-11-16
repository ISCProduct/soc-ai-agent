package config

import "testing"

func TestEnvGetters(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("RAG_REVIEW_URL", "  http://rag:9000 \n")
	t.Setenv("USER_SECRET", "u")
	t.Setenv("ADMIN_SECRET", "a")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "k")
	t.Setenv("FRONTEND_URL", "http://fe")

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
