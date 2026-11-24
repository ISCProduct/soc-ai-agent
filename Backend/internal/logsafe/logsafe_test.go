package logsafe

import "testing"

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{
		"taro@example.com": "t***@example.com", "a@example.com": "a***@example.com", "invalid": "***", "": "***", " taro@example.com ": "t***@example.com",
	} {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestMaskEmailsAndChars(t *testing.T) {
	got := MaskEmails([]string{"taro@example.com", "hanako@example.org"})
	if len(got) != 2 || got[0] != "t***@example.com" || got[1] != "h***@example.org" {
		t.Fatalf("got %v", got)
	}
	if Chars("山田太郎abc") != 7 {
		t.Fatalf("Chars=%d", Chars("山田太郎abc"))
	}
}
