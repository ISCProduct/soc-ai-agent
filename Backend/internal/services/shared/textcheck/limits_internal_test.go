package textcheck

import (
	"encoding/json"
	"os"
	"testing"
)

// 字数の入力と期待値は rag/testdata/char_limit_cases.json に置く。
// Go と RAG の pytest が同じファイルを読む。どちらかの数え方や許容範囲だけが変わると、
// こちらが落ちる（#1957）。RAG のソースを正規表現で読むと、コメントや改行で落ちる。
func TestCharLimit_MatchesSharedGolden(t *testing.T) {
	raw, err := os.ReadFile("../../../../../rag/testdata/char_limit_cases.json")
	if err != nil {
		t.Fatalf("共有ゴールデンが読めない: %v", err)
	}
	var doc struct {
		Cases []struct {
			Text    string `json:"text"`
			Limit   int    `json:"limit"`
			Mode    string `json:"mode"`
			Chars   int    `json:"chars"`
			Low     int    `json:"low"`
			High    int    `json:"high"`
			Verdict string `json:"verdict"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("ゴールデンの JSON が壊れている: %v", err)
	}
	if len(doc.Cases) == 0 {
		t.Fatal("ゴールデンにケースが無い")
	}

	for i, c := range doc.Cases {
		if got := CountChars(c.Text); got != c.Chars {
			t.Errorf("case %d CountChars = %d, want %d", i, got, c.Chars)
		}
		rule := CharLimit{Limit: c.Limit, Mode: c.Mode}
		low, high := rule.bounds()
		if c.Limit > 0 && (low != c.Low || high != c.High) {
			t.Errorf("case %d bounds = (%d, %d), want (%d, %d)", i, low, high, c.Low, c.High)
		}
		findings := rule.Check(c.Text)
		switch c.Verdict {
		case "ok", "unchecked":
			if len(findings) != 0 {
				t.Errorf("case %d verdict %s, got %+v", i, c.Verdict, findings)
			}
		case "too_short":
			if len(findings) != 1 || findings[0].Severity != SeverityWarning {
				t.Errorf("case %d want too_short warning, got %+v", i, findings)
			}
		case "too_long":
			if len(findings) != 1 || findings[0].Severity != SeverityCritical {
				t.Errorf("case %d want too_long critical, got %+v", i, findings)
			}
		default:
			t.Errorf("case %d unknown verdict %q", i, c.Verdict)
		}
	}
}
