package diagnosis

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/piimask"
)

// 診断品質の評価は、会話の抜粋を外の LLM へ送る。その学生の氏名・電話が、送る前に伏字になること（合成データ）。
func TestEvaluateWithLLM_MasksChatExcerptBeforeSending(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"confidence\":70,\"flags\":[],\"summary\":\"ok\"}"}}]}`))
	}))
	defer srv.Close()

	svc := &QualityService{aiClient: openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")}
	svc.SetSubjectReader(piimask.SubjectReaderFunc(func(uint) (piimask.Subject, error) {
		return piimask.Subject{Name: "山田 太郎", Email: "taro.yamada@example.com"}, nil
	}))
	// RunDiagnosisQuality と同じ手順でガードを付けて評価する
	ctx := openai.WithInputGuard(context.Background(), piimask.GuardFor(svc.subjects, 7))
	msgs := []models.ChatMessage{{Role: "user", Content: "山田太郎です。090-1234-5678 まで。チーム開発をしました。"}}
	if _, err := svc.evaluateWithLLM(ctx, nil, msgs, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("リクエスト数 = %d, want 1", len(bodies))
	}
	for _, leak := range []string{"山田", "太郎", "090-1234"} {
		if strings.Contains(bodies[0], leak) {
			t.Errorf("%q が外部へ送られた: %s", leak, bodies[0])
		}
	}
	if !strings.Contains(bodies[0], "チーム開発をしました") {
		t.Errorf("本文が消えた: %s", bodies[0])
	}
}

// 登録情報を読めないときは、会話を外へ送らない。
func TestEvaluateWithLLM_FailsClosedWhenSubjectUnavailable(t *testing.T) {
	var mu sync.Mutex
	sent := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sent++
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	svc := &QualityService{aiClient: openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")}
	svc.SetSubjectReader(piimask.SubjectReaderFunc(func(uint) (piimask.Subject, error) { return piimask.Subject{}, errors.New("db down") }))
	ctx := openai.WithInputGuard(context.Background(), piimask.GuardFor(svc.subjects, 7))
	_, err := svc.evaluateWithLLM(ctx, nil, []models.ChatMessage{{Role: "user", Content: "山田太郎です"}}, nil)
	if !errors.Is(err, openai.ErrInputGuardFailed) {
		t.Fatalf("err = %v, want ErrInputGuardFailed", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if sent != 0 {
		t.Errorf("読めないのに %d 件送られた", sent)
	}
}
