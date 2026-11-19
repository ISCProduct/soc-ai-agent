package chat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	internalOpenAI "Backend/internal/openai"
)

// userRepoStub は GetUserByID だけを実装する（ほかのメソッドは呼ばれない）。
type userRepoStub struct {
	repository.UserRepository
	user *models.User
	err  error
}

func (s userRepoStub) GetUserByID(uint) (*models.User, error) { return s.user, s.err }

// 外部 AI へ送られた本文を記録する疑似サーバー。
func newChatBodyServer(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2],"index":0}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"output_text":"ok"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

// 回答に含まれる、その学生の氏名・メール・電話が、外部 AI へ送られる前に伏字になること（合成データ）。
func TestWithPIIGuard_MasksStudentAnswerBeforeSending(t *testing.T) {
	users := userRepoStub{user: &models.User{Name: "山田 太郎", Email: "taro.yamada@example.com", SchoolName: "架空情報専門学校"}}
	srv, bodies := newChatBodyServer(t)
	svc := &ChatService{userRepo: users, aiClient: internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")}

	ctx := svc.withPIIGuard(context.Background(), 7)
	answer := "山田太郎です。連絡は taro.yamada@example.com か 090-1234-5678。架空情報専門学校でチーム開発をしました。"
	if _, err := svc.aiClient.Responses(ctx, "質問: 学生時代に頑張ったことは？\n回答: "+answer); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.aiClient.Embedding(ctx, answer); err != nil {
		t.Fatal(err)
	}
	got := bodies()
	if len(got) != 2 {
		t.Fatalf("リクエスト数 = %d, want 2", len(got))
	}
	for _, b := range got {
		for _, leak := range []string{"山田", "太郎", "taro.yamada", "090-1234", "架空情報"} {
			if strings.Contains(b, leak) {
				t.Errorf("%q が外部へ送られた: %s", leak, b)
			}
		}
		if !strings.Contains(b, "チーム開発をしました") {
			t.Errorf("本文が消えた: %s", b)
		}
	}
}

// 登録情報を読めないときは、回答を外へ送らない。
func TestWithPIIGuard_FailsClosedWhenUserUnavailable(t *testing.T) {
	users := userRepoStub{err: errors.New("db down")}
	srv, bodies := newChatBodyServer(t)
	svc := &ChatService{userRepo: users, aiClient: internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")}

	_, err := svc.aiClient.Responses(svc.withPIIGuard(context.Background(), 7), "回答: 山田太郎です")
	if !errors.Is(err, internalOpenAI.ErrInputGuardFailed) {
		t.Fatalf("err = %v, want ErrInputGuardFailed", err)
	}
	if n := len(bodies()); n != 0 {
		t.Errorf("読めないのに %d 件送られた", n)
	}
}

// 未ログイン（userID==0）でも、規則（メール・電話など）での伏字はかける。
func TestWithPIIGuard_RulesOnlyWhenAnonymous(t *testing.T) {
	srv, bodies := newChatBodyServer(t)
	svc := &ChatService{aiClient: internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")}
	if _, err := svc.aiClient.Responses(svc.withPIIGuard(context.Background(), 0), "回答: 連絡は a@example.com まで"); err != nil {
		t.Fatal(err)
	}
	if b := bodies(); len(b) != 1 || strings.Contains(b[0], "a@example.com") {
		t.Errorf("メールが送られた: %v", b)
	}
}
