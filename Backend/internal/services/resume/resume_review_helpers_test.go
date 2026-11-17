package resume

import (
	"errors"
	"fmt"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/shared"
)

func TestPrependCompanyBrief(t *testing.T) {
	tests := []struct {
		name, brief, info, want string
	}{
		{"briefが空ならそのまま", "", "RAG", "RAG"},
		{"infoが空白ならbriefのみ", "BRIEF", "  ", "BRIEF"},
		{"briefを前に置く", "BRIEF", "RAG", "BRIEF\n\nRAG"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prependCompanyBrief(tt.brief, tt.info); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReviewScoreErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"検証エラーは本文を見せる", &shared.ValidationError{Message: "企業が見つかりません"}, "企業が見つかりません"},
		{"ラップされた検証エラーも見せる", fmt.Errorf("x: %w", &shared.ValidationError{Message: "入力不備"}), "入力不備"},
		{"AI不可は内部設定を隠す", fmt.Errorf("provider key missing: %w", openai.ErrAIUnavailable), "現在AI機能を利用できません。しばらくしてから再度お試しください。"},
		{"それ以外は内部の文言を隠す", errors.New("open /data/secret.pdf: permission denied"), "レビューを作成できませんでした。時間をおいてもう一度お試しください。"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reviewScoreErrorMessage(tt.err); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasExtractableText(t *testing.T) {
	tests := []struct {
		name   string
		blocks []models.ResumeTextBlock
		want   bool
	}{
		{"空", nil, false},
		{"空白のみ", []models.ResumeTextBlock{{Text: " \n\t"}}, false},
		{"1つでも文字があれば真", []models.ResumeTextBlock{{Text: " "}, {Text: "経歴"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasExtractableText(tt.blocks); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
