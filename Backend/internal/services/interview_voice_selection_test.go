package services

import "testing"

func TestTTSVoiceForGenderAndLang(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gender string
		lang   string
		want   string
	}{
		{name: "male ja uses onyx", gender: "male", lang: "ja", want: "onyx"},
		{name: "male ko uses onyx", gender: "male", lang: "ko", want: "onyx"},
		{name: "male en uses echo", gender: "male", lang: "en", want: "echo"},
		{name: "male uppercase lang uses echo", gender: "male", lang: "EN", want: "echo"},
		{name: "female ja uses shimmer", gender: "female", lang: "ja", want: "shimmer"},
		{name: "invalid gender falls back female", gender: "unknown", lang: "en", want: "shimmer"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ttsVoiceForGenderAndLang(tt.gender, tt.lang)
			if got != tt.want {
				t.Fatalf("ttsVoiceForGenderAndLang(%q, %q) = %q, want %q", tt.gender, tt.lang, got, tt.want)
			}
		})
	}
}

func TestRealtimeVoiceForLangAndGender_EnvOverride(t *testing.T) {
	t.Setenv("OPENAI_REALTIME_VOICE", "alloy")
	if got := realtimeVoiceForLangAndGender("en", "male"); got != "alloy" {
		t.Fatalf("realtimeVoiceForLangAndGender should respect OPENAI_REALTIME_VOICE, got %q", got)
	}
}

func TestRealtimeVoiceForLangAndGender_DefaultSelection(t *testing.T) {
	t.Setenv("OPENAI_REALTIME_VOICE", "")
	if got := realtimeVoiceForLangAndGender("en", "male"); got != "echo" {
		t.Fatalf("realtimeVoiceForLangAndGender(en,male) = %q, want echo", got)
	}
}
