package interview

import "testing"

func TestApplyCompanyReadingForTTS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		text           string
		companyName    string
		companyReading string
		want           string
	}{
		{
			name:           "replaces company name with reading",
			text:           "味の素株式会社様の面接を始めます。味の素株式会社での経験を教えてください。",
			companyName:    "味の素株式会社",
			companyReading: "アジノモト",
			want:           "アジノモト様の面接を始めます。アジノモトでの経験を教えてください。",
		},
		{
			name:           "no reading leaves text unchanged",
			text:           "味の素株式会社様の面接を始めます。",
			companyName:    "味の素株式会社",
			companyReading: "",
			want:           "味の素株式会社様の面接を始めます。",
		},
		{
			name:           "no company name leaves text unchanged",
			text:           "本日はよろしくお願いします。",
			companyName:    "",
			companyReading: "アジノモト",
			want:           "本日はよろしくお願いします。",
		},
		{
			name:           "reading equal to name leaves text unchanged",
			text:           "ABC株式会社の面接です。",
			companyName:    "ABC株式会社",
			companyReading: "ABC株式会社",
			want:           "ABC株式会社の面接です。",
		},
		{
			name:           "company name not present in text leaves text unchanged",
			text:           "本日はよろしくお願いします。",
			companyName:    "味の素株式会社",
			companyReading: "アジノモト",
			want:           "本日はよろしくお願いします。",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := applyCompanyReadingForTTS(tt.text, tt.companyName, tt.companyReading)
			if got != tt.want {
				t.Fatalf("applyCompanyReadingForTTS(%q, %q, %q) = %q, want %q", tt.text, tt.companyName, tt.companyReading, got, tt.want)
			}
		})
	}
}
