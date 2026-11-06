package interview

import (
	"strings"
	"testing"
)

func TestParseCompanyQuestionHints(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "省略", raw: "  "},
		{name: "有効な候補", raw: `["質問1","質問2"]`, want: []string{"質問1", "質問2"}},
		{name: "JSON以外", raw: `"質問"`, wantErr: true},
		{name: "候補数超過", raw: `["1","2","3","4","5","6"]`, wantErr: true},
		{name: "文字数超過", raw: `["` + strings.Repeat("あ", 301) + `"]`, wantErr: true},
		{name: "payload超過", raw: `"` + strings.Repeat("x", 16*1024) + `"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompanyQuestionHints(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCompanyQuestionHints() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseCompanyQuestionHints() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseCompanyQuestionHints()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
