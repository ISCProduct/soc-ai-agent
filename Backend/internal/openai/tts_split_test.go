package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSplitForTTS(t *testing.T) {
	tests := []struct {
		name string
		text string
		max  int
		want []string
	}{
		{"1文はそのまま", "自己紹介をお願いします。", 3, []string{"自己紹介をお願いします。"}},
		{"区切りが無ければそのまま", "よろしくお願いします", 3, []string{"よろしくお願いします"}},
		{"3文は3つ", "ありがとうございます。よく分かりました。次に志望動機を教えてください。", 3,
			[]string{"ありがとうございます。", "よく分かりました。", "次に志望動機を教えてください。"}},
		{"上限を超える文はまとめる", "はい。なるほど。分かりました。では。次に。", 2,
			[]string{"はい。なるほど。分かりました。", "では。次に。"}},
		{"末尾の区切り無しの文も残す", "なるほど。では続けてください", 3, []string{"なるほど。", "では続けてください"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitForTTS(tt.text, tt.max)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("splitForTTS = %q, want %q", got, tt.want)
			}
			if strings.Join(got, "") != tt.text {
				t.Errorf("つなげると元の文にならない: %q", got)
			}
		})
	}
}

// 文ごとの読み上げは同時に作り、元の順番でつなげる。
func TestTTS_SynthesizesChunksConcurrentlyInOrder(t *testing.T) {
	var inFlight, maxInFlight atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		var body struct {
			Input string `json:"input"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("[" + body.Input + "]"))
	}))
	defer server.Close()

	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	got, err := cli.TTS(context.Background(), "ありがとうございます。よく分かりました。次の質問です。", "alloy")
	if err != nil {
		t.Fatalf("TTS error: %v", err)
	}
	if string(got) != "[ありがとうございます。][よく分かりました。][次の質問です。]" {
		t.Errorf("つなげた音声の順番が違う: %s", got)
	}
	if maxInFlight.Load() < 2 {
		t.Errorf("同時に作っていない（最大同時数 %d）", maxInFlight.Load())
	}
}

// 1つでも失敗したら全体を失敗にする（途中が欠けた音声を返さない。呼び出し元はローカルへ縮退する）。
func TestTTS_FailsWhenAnyChunkFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "分かりました") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	if got, err := cli.TTS(context.Background(), "はい。分かりました。", "alloy"); err == nil {
		t.Fatalf("途中の失敗を無視した: %q", got)
	}
}
