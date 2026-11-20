package chat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"Backend/internal/openai"
)

// 質問側の埋め込みは同じ質問ならプロセス内で使い回し、API は回答側だけ呼ぶこと（合成データ）。

type embServer struct {
	mu     sync.Mutex
	inputs []string
	fail   bool
}

func (e *embServer) count(sub string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, in := range e.inputs {
		if strings.Contains(in, sub) {
			n++
		}
	}
	return n
}

func newEmbClient(t *testing.T) (*openai.Client, *embServer) {
	t.Helper()
	es := &embServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		es.mu.Lock()
		es.inputs = append(es.inputs, string(b))
		fail := es.fail
		es.mu.Unlock()
		if fail {
			http.Error(w, `{"error":{"message":"boom"}}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// 質問と回答で別のベクトルを返す（類似度が 1 にならないように）
		if strings.Contains(string(b), "QQQ") {
			_, _ = w.Write([]byte(`{"data":[{"embedding":[1,0,0],"index":0}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"embedding":[1,1,0],"index":0}]}`))
	}))
	t.Cleanup(srv.Close)
	return openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), es
}

func TestAnswerSimilarity_質問側の埋め込みはキャッシュされる(t *testing.T) {
	client, es := newEmbClient(t)
	s := &ChatService{aiClient: client}
	ctx := context.Background()

	var first float64
	for i := range 3 {
		cos, ok := s.answerSimilarity(ctx, "QQQ 質問", "回答"+string(rune('A'+i)))
		if !ok {
			t.Fatalf("ok=false (i=%d)", i)
		}
		if i == 0 {
			first = cos
		}
		// 類似度は [1,0,0] と [1,1,0] のコサイン = 1/√2
		if cos < 0.7070 || cos > 0.7072 || cos != first {
			t.Fatalf("cos = %v", cos)
		}
	}
	if got := es.count("QQQ"); got != 1 {
		t.Fatalf("質問側の API 呼び出し = %d, want 1", got)
	}
	if got := es.count("回答"); got != 3 {
		t.Fatalf("回答側の API 呼び出し = %d, want 3", got)
	}
}

func TestAnswerSimilarity_取得エラーはキャッシュしない(t *testing.T) {
	client, es := newEmbClient(t)
	s := &ChatService{aiClient: client}
	ctx := context.Background()

	es.mu.Lock()
	es.fail = true
	es.mu.Unlock()
	if _, ok := s.answerSimilarity(ctx, "QQQ 質問", "回答"); ok {
		t.Fatal("エラー時は ok=false のはず")
	}
	es.mu.Lock()
	es.fail = false
	es.mu.Unlock()
	if _, ok := s.answerSimilarity(ctx, "QQQ 質問", "回答"); !ok {
		t.Fatal("復旧後は成功するはず（エラーがキャッシュされている）")
	}
}

func TestQuestionEmbCache_件数上限を超えない(t *testing.T) {
	var c questionEmbCache
	for i := range questionEmbCacheMax * 2 {
		c.put(string(rune('a'+i%26))+strings.Repeat("x", i), []float32{1})
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > questionEmbCacheMax {
		t.Fatalf("len = %d, max %d", len(c.m), questionEmbCacheMax)
	}
}
