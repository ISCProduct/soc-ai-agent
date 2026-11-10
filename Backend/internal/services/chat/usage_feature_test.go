package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"Backend/internal/openai"
	"Backend/internal/usagectx"
)

// #1993: チャットの質問生成と埋め込みの利用量に、機能名が付くこと。

func newUsageRecordingClient(t *testing.T) (*openai.Client, func() []openai.Usage) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/embeddings"):
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}],"usage":{"prompt_tokens":3,"total_tokens":3}}`))
		case strings.HasSuffix(r.URL.Path, "/responses"):
			_, _ = w.Write([]byte(`{"output_text":"次の質問です","usage":{"input_tokens":10,"output_tokens":5}}`))
		default:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
		}
	}))
	t.Cleanup(server.Close)
	client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
	var mu sync.Mutex
	var usages []openai.Usage
	client.OnUsage = func(u openai.Usage) {
		mu.Lock()
		defer mu.Unlock()
		usages = append(usages, u)
	}
	return client, func() []openai.Usage {
		mu.Lock()
		defer mu.Unlock()
		return append([]openai.Usage(nil), usages...)
	}
}

func TestAICallWithRetries_既定は質問生成で呼び出し元の機能名は残す(t *testing.T) {
	client, usages := newUsageRecordingClient(t)
	s := &ChatService{aiClient: client}

	ctx := usagectx.WithActor(context.Background(), 7, 3)
	if _, err := s.aiCallWithRetries(ctx, "質問を作ってください"); err != nil {
		t.Fatalf("aiCallWithRetries: %v", err)
	}
	if _, err := s.aiCallWithRetries(usagectx.WithFeature(ctx, usagectx.FeatureChatAnswerEval), "採点してください"); err != nil {
		t.Fatalf("aiCallWithRetries: %v", err)
	}
	got := usages()
	if len(got) != 2 {
		t.Fatalf("利用量の記録 = %d件, want 2", len(got))
	}
	if got[0].Feature != usagectx.FeatureChatQuestion || got[1].Feature != usagectx.FeatureChatAnswerEval {
		t.Fatalf("feature = %q, %q", got[0].Feature, got[1].Feature)
	}
	if got[0].UserID == nil || *got[0].UserID != 7 {
		t.Fatalf("実行主体が届いていない: %+v", got[0].UserID)
	}
}

func TestValidateAnswerRelevance_埋め込みは機能名embedding(t *testing.T) {
	client, usages := newUsageRecordingClient(t)
	s := &ChatService{aiClient: client}

	if _, err := s.validateAnswerRelevance(context.Background(), "あなたが大切にしている価値観を教えてください", "人の役に立つことを大切にしています"); err != nil {
		t.Fatalf("validateAnswerRelevance: %v", err)
	}
	got := usages()
	if len(got) == 0 {
		t.Fatal("埋め込みが呼ばれていない（文章系の質問として判定されなかった）")
	}
	for _, u := range got {
		if u.Feature != usagectx.FeatureEmbedding {
			t.Fatalf("feature = %q, want embedding", u.Feature)
		}
	}
}
