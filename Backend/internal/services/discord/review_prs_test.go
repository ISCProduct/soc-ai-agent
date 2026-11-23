package discord

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSelectWaiting(t *testing.T) {
	prs := []pullSnapshot{
		{Number: 1, Title: "draft", Draft: true, Requested: []string{"a"}},
		{Number: 2, Title: "changes", HeadSHA: "aaa", Requested: []string{"a"}, Reviews: []pullReview{{Login: "a", State: "CHANGES_REQUESTED", SHA: "aaa"}}},
		{Number: 3, Title: "updated", HeadSHA: "bbb", Reviews: []pullReview{{Login: "b", State: "CHANGES_REQUESTED", SHA: "aaa"}}},
		{Number: 4, Title: "waiting", HeadSHA: "ccc", Requested: []string{"c", "c"}},
		{Number: 5, Title: "nobody", HeadSHA: "ddd"},
	}
	got := SelectWaiting(prs)
	if len(got) != 2 || got[0].Number != 3 || got[1].Number != 4 {
		t.Fatalf("got %+v", got)
	}
	if len(got[1].Reviewers) != 1 || got[1].Reviewers[0] != "c" {
		t.Fatalf("reviewers = %v", got[1].Reviewers)
	}
	if got[0].Reviewers[0] != "b" {
		t.Fatalf("更新後の依頼先 = %v", got[0].Reviewers)
	}
}

func TestGitHubReviewClient_Waiting(t *testing.T) {
	const token = "ghp_test_token_not_real"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), token) {
			t.Error("トークンが本文に入っている")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
		  "data": {"repository": {"pullRequests": {"nodes": [
		    {"number": 9, "title": "見る", "url": "https://github.com/o/r/pull/9", "isDraft": false, "headRefOid": "h",
		     "reviewRequests": {"nodes": [{"requestedReviewer": {"login": "ohashi"}}]},
		     "reviews": {"nodes": []}}
		  ]}}}
		}`)
	}))
	defer srv.Close()
	c := &GitHubReviewClient{token: token, owner: "o", repo: "r", endpoint: srv.URL, http: srv.Client()}
	got, err := c.Waiting(context.Background())
	if err != nil || len(got) != 1 || got[0].Number != 9 || got[0].Reviewers[0] != "ohashi" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestGitHubReviewClient_失敗にトークンを含めない(t *testing.T) {
	const token = "ghp_secret_value"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope "+token, http.StatusForbidden)
	}))
	defer srv.Close()
	c := &GitHubReviewClient{token: token, owner: "o", repo: "r", endpoint: srv.URL, http: srv.Client()}
	_, err := c.Waiting(context.Background())
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("err = %v", err)
	}
}

func TestNewGitHubReviewClientFromEnv_配信用トークンは使わない(t *testing.T) {
	t.Setenv("GITHUB_PR_READ_TOKEN", "same-token")
	t.Setenv("GITHUB_DISPATCH_TOKEN", "same-token")
	t.Setenv("GITHUB_PR_READ_REPO", "ISCProduct/soc-ai-agent")
	if NewGitHubReviewClientFromEnv() != nil {
		t.Fatal("配信用トークンを読み取りに使った")
	}
	t.Setenv("GITHUB_PR_READ_TOKEN", "")
	if NewGitHubReviewClientFromEnv() != nil {
		t.Fatal("トークンが無いのにクライアントを作った")
	}
}

func TestNormalizeGitHubLogin(t *testing.T) {
	got, err := NormalizeGitHubLogin("  @Ohashi-Kazuyuki ")
	if err != nil || got != "Ohashi-Kazuyuki" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := NormalizeGitHubLogin("-bad"); err == nil {
		t.Fatal("先頭ハイフンを通した")
	}
	got, err = NormalizeGitHubLogin("  ")
	if err != nil || got != "" {
		t.Fatalf("空 = %q err %v", got, err)
	}
}
