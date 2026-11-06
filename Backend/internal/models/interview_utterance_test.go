package models

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

// 面接詳細APIは発話をこのモデルのまま返し、フロントエンドは小文字のキーを読む（#1775）。
// タグを外すと id / role / text が ID / Role / Text に変わり、発話ログの参照が壊れる。
func TestInterviewUtteranceJSONKeys(t *testing.T) {
	clientID := "client-1"
	body, err := json.Marshal(InterviewUtterance{
		ID:                1,
		SessionID:         2,
		Role:              "user",
		Text:              "発話",
		ClientUtteranceID: &clientID,
		CreatedAt:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(decoded))
	for key := range decoded {
		got = append(got, key)
	}
	slices.Sort(got)

	want := []string{"client_utterance_id", "created_at", "id", "role", "session_id", "text"}
	if !slices.Equal(got, want) {
		t.Fatalf("JSON keys = %v, want %v", got, want)
	}
}
