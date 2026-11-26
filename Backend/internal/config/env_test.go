package config

import "testing"

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"staging", false},
		{"", false},
		{" production", false}, // 元の実装どおり空白は除かない
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.env)
			if got := IsProduction(); got != tt.want {
				t.Errorf("APP_ENV=%q: IsProduction = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestStringGetters(t *testing.T) {
	tests := []struct {
		name string
		key  string
		get  func() string
		val  string
		want string
	}{
		{"RAG URL は空白を除く", "RAG_REVIEW_URL", RAGReviewURL, "  http://rag:9000 \n", "http://rag:9000"},
		{"RAG URL 未設定は空", "RAG_REVIEW_URL", RAGReviewURL, "", ""},
		{"FRONTEND_URL は空白を除く", "FRONTEND_URL", FrontendURL, " http://fe\n", "http://fe"},
		// 検証側 LoadConfig と同じ値で署名するため、秘密鍵は空白を除かない
		{"USER_SECRET はそのまま", "USER_SECRET", UserSecret, " u\n", " u\n"},
		{"ADMIN_SECRET はそのまま", "ADMIN_SECRET", AdminSecret, " a\n", " a\n"},
		{"TOKEN_ENCRYPTION_KEY はそのまま", "TOKEN_ENCRYPTION_KEY", TokenEncryptionKey, " k\n", " k\n"},
		{"USER_SECRET 未設定は空", "USER_SECRET", UserSecret, "", ""},
		// Webhook は空白を除く。キー名の取り違えを検出するため getter ごとに確認する
		{"Realtime Slack", "REALTIME_ALERT_SLACK_WEBHOOK_URL", RealtimeAlertSlackWebhookURL, " http://rt\n", "http://rt"},
		{"OpenAI Discord", "OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL", OpenAICostAlertDiscordWebhookURL, " http://od ", "http://od"},
		{"企業検索 Slack", "COMPANY_SEARCH_ALERT_SLACK_WEBHOOK_URL", CompanySearchAlertSlackWebhookURL, " http://cs ", "http://cs"},
		{"企業検索 Discord", "COMPANY_SEARCH_ALERT_DISCORD_WEBHOOK_URL", CompanySearchAlertDiscordWebhookURL, " http://cd ", "http://cd"},
		// モデル名は元の実装どおり空白を除かない
		{"Whisper モデル", "OPENAI_WHISPER_MODEL", OpenAIWhisperModel, "whisper-x", "whisper-x"},
		{"Whisper 未設定は空", "OPENAI_WHISPER_MODEL", OpenAIWhisperModel, "", ""},
		{"TTS モデル", "OPENAI_TTS_MODEL", OpenAITTSModel, " tts-x", " tts-x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.val)
			if got := tt.get(); got != tt.want {
				t.Errorf("%s=%q: got %q, want %q", tt.key, tt.val, got, tt.want)
			}
		})
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

func TestPromptEnv(t *testing.T) {
	t.Setenv("PROMPT_VERSION_ES_REVIEW", " v2 ")
	t.Setenv("PROMPT_VERSION", "v1")
	t.Setenv("PROMPT_AB_ES_REVIEW", " v1:50,v2:50 ")
	if got := PromptVersion("es_review"); got != "v2" {
		t.Errorf("PromptVersion = %q, 工程名は大文字化され空白は除かれるべき", got)
	}
	if got := GlobalPromptVersion(); got != "v1" {
		t.Errorf("GlobalPromptVersion = %q", got)
	}
	if got := PromptABSpec("es_review"); got != "v1:50,v2:50" {
		t.Errorf("PromptABSpec = %q", got)
	}
	if got := PromptVersion("other"); got != "" {
		t.Errorf("未設定の工程は空文字のはず: %q", got)
	}
}

func TestMatchingReasonUseAI(t *testing.T) {
	for val, want := range map[string]bool{"": false, "true": true, "TRUE": true, "1": true, " yes ": true, "0": false, "no": false} {
		t.Setenv("MATCHING_REASON_USE_AI", val)
		if got := MatchingReasonUseAI(); got != want {
			t.Errorf("MATCHING_REASON_USE_AI=%q: got %v, want %v", val, got, want)
		}
	}
}
