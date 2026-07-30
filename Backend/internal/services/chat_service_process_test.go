package services

import (
	"Backend/internal/models"
	"testing"
)

func TestMergeAskedTexts(t *testing.T) {
	cases := []struct {
		name        string
		history     []models.ChatMessage
		aiQuestions []models.AIGeneratedQuestion
		wantKeys    []string
		wantCount   int
	}{
		{
			name:        "empty inputs",
			history:     nil,
			aiQuestions: nil,
			wantKeys:    nil,
			wantCount:   0,
		},
		{
			name: "assistant history only",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "最近頑張ったことは？"},
				{Role: "user", Content: "ハッカソンに参加しました"},
			},
			wantKeys:  []string{"最近頑張ったことは？"},
			wantCount: 1,
		},
		{
			name: "ai questions with hint suffix stripped",
			aiQuestions: []models.AIGeneratedQuestion{
				{QuestionText: "チームで協力した経験は？\n\n💡 具体例を教えてください"},
			},
			wantKeys:  []string{"チームで協力した経験は？"},
			wantCount: 1,
		},
		{
			name: "deduplicates normalized text from both sources",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "  最近頑張ったことは？  "},
			},
			aiQuestions: []models.AIGeneratedQuestion{
				{QuestionText: "最近頑張ったことは？"},
			},
			wantKeys:  []string{"最近頑張ったことは？"},
			wantCount: 1,
		},
		{
			name: "ignores empty and user messages",
			history: []models.ChatMessage{
				{Role: "user", Content: "回答"},
				{Role: "assistant", Content: "   "},
			},
			aiQuestions: []models.AIGeneratedQuestion{
				{QuestionText: ""},
			},
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeAskedTexts(tc.history, tc.aiQuestions)
			if len(got) != tc.wantCount {
				t.Fatalf("len=%d want %d", len(got), tc.wantCount)
			}
			for _, key := range tc.wantKeys {
				if !got[key] {
					t.Fatalf("missing key %q in %v", key, got)
				}
			}
		})
	}
}
