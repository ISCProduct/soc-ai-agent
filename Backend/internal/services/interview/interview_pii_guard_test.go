package interview

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
	"Backend/internal/openai"
)

type piiUserRepoStub struct {
	repository.UserRepository
	user *models.User
	err  error
}

func (s piiUserRepoStub) GetUserByID(uint) (*models.User, error) { return s.user, s.err }

func newPIIBodyServer(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{}"}}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

// 面接の発話に含まれる、その学生の氏名・メール・電話が、外部 AI へ送られる前に伏字になること（合成データ）。
func TestWithPIIGuard_MasksSpokenTextBeforeSending(t *testing.T) {
	srv, bodies := newPIIBodyServer(t)
	cli := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	svc := &InterviewService{
		userRepo:     piiUserRepoStub{user: &models.User{Name: "山田 太郎", Email: "taro.yamada@example.com", SchoolName: "架空情報専門学校"}},
		openaiClient: cli,
	}
	ctx := svc.withPIIGuard(context.Background(), 7)
	if _, err := cli.ChatCompletionJSON(ctx, "sys json", "応募者発言: 山田太郎です。090-1234-5678 か taro.yamada@example.com へ。架空情報専門学校でチーム開発をしました。", 0.2, 100); err != nil {
		t.Fatal(err)
	}
	got := bodies()
	if len(got) != 1 {
		t.Fatalf("リクエスト数 = %d, want 1", len(got))
	}
	for _, leak := range []string{"山田", "太郎", "090-1234", "taro.yamada", "架空情報"} {
		if strings.Contains(got[0], leak) {
			t.Errorf("%q が外部へ送られた: %s", leak, got[0])
		}
	}
	if !strings.Contains(got[0], "チーム開発をしました") {
		t.Errorf("本文が消えた: %s", got[0])
	}
}

// 登録情報を読めないときは、発話を外へ送らない。
func TestWithPIIGuard_FailsClosedWhenUserUnavailable(t *testing.T) {
	srv, bodies := newPIIBodyServer(t)
	cli := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	svc := &InterviewService{userRepo: piiUserRepoStub{err: errors.New("db down")}, openaiClient: cli}
	_, err := cli.ChatCompletionJSON(svc.withPIIGuard(context.Background(), 7), "sys json", "応募者発言: 山田太郎です", 0.2, 100)
	if !errors.Is(err, openai.ErrInputGuardFailed) {
		t.Fatalf("err = %v, want ErrInputGuardFailed", err)
	}
	if n := len(bodies()); n != 0 {
		t.Errorf("読めないのに %d 件送られた", n)
	}
}
