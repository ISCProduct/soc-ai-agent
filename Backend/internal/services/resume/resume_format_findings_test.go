package resume

import (
	"encoding/json"
	"strings"
	"testing"

	"Backend/internal/models"
)

func TestFormatFindingsFor_KeepsBlockIndexZero(t *testing.T) {
	fs := formatFindingsFor([]models.ResumeTextBlock{{PageNumber: 1, BlockIndex: 0, Text: "飲食店のバイトをしました。"}})
	if len(fs) != 1 {
		t.Fatalf("got %+v", fs)
	}
	raw, err := json.Marshal(fs)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{`"block_index":0`, `"page_hint":1`} {
		if !strings.Contains(s, want) {
			t.Errorf("%s が無い: %s", want, s)
		}
	}
	if n := strings.Count(s, `"block_index"`); n != 1 {
		t.Errorf("block_index が %d 回出た: %s", n, s)
	}
}

func TestFormatFindingsFor_EmptyIsArray(t *testing.T) {
	raw, _ := json.Marshal(formatFindingsFor(nil))
	if string(raw) != "[]" {
		t.Errorf("got %s", raw)
	}
}
