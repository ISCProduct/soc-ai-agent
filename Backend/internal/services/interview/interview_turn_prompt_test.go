package interview

import (
	"strings"
	"testing"
)

// TestBuildInterviewSystemPromptToneGuideline は #910 の回帰テスト。
// AIが叱責的なトーンで応答しないよう、プロンプトに中立トーンの指示が
// 含まれていることを検証する。
func TestBuildInterviewSystemPromptToneGuideline(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)
	if !strings.Contains(prompt, "叱責") {
		t.Fatalf("prompt missing tone guideline (叱責禁止): %s", prompt)
	}
}

func TestBuildInterviewSystemPromptDeepeningMotivationCriteria(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)
	if !strings.Contains(prompt, "きっかけ") || !strings.Contains(prompt, "継続") {
		t.Fatalf("prompt missing motivation/continuity deepening criteria: %s", prompt)
	}
}

func TestIsEngineerPosition(t *testing.T) {
	tests := []struct {
		name     string
		position string
		want     bool
	}{
		{name: "日本語エンジニア", position: "バックエンドエンジニア", want: true},
		{name: "英語小文字", position: "software engineer", want: true},
		{name: "英語大文字", position: "Senior Developer", want: true},
		{name: "SRE", position: "Site Reliability Engineer (SRE)", want: true},
		{name: "非エンジニア", position: "法人営業", want: false},
		{name: "空文字", position: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEngineerPosition(tt.position); got != tt.want {
				t.Fatalf("isEngineerPosition(%q) = %v, want %v", tt.position, got, tt.want)
			}
		})
	}
}
