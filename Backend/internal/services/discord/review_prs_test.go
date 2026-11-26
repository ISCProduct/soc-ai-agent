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

func TestSelectWaiting_レビューは履歴なので最新の意見で判断する(t *testing.T) {
	tests := []struct {
		name         string
		pr           pullSnapshot
		wantListed   bool
		wantContains []string
		wantExcludes []string
	}{
		{
			name: "変更要求のあと新しいコミットで承認した人は載せない",
			pr: pullSnapshot{Number: 1, HeadSHA: "h2", Requested: []string{"r"}, Reviews: []pullReview{
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
				{Login: "a", State: "APPROVED", SHA: "h2"},
			}},
			wantListed:   true,
			wantContains: []string{"r"},
			wantExcludes: []string{"a"},
		},
		{
			name: "承認した人だけが残る PR は依頼先が無く載らない",
			pr: pullSnapshot{Number: 1, HeadSHA: "h2", Reviews: []pullReview{
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
				{Login: "a", State: "APPROVED", SHA: "h2"},
			}},
			wantListed: false,
		},
		{
			name: "同じコミットで変更要求のあとに承認しても変更要求ではない",
			pr: pullSnapshot{Number: 1, HeadSHA: "h1", Requested: []string{"r"}, Reviews: []pullReview{
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
				{Login: "A", State: "APPROVED", SHA: "h1"},
			}},
			wantListed:   true,
			wantContains: []string{"r"},
		},
		{
			name: "承認のあとに変更要求を出した人は、最新のコミットなら除外する",
			pr: pullSnapshot{Number: 1, HeadSHA: "h1", Requested: []string{"r"}, Reviews: []pullReview{
				{Login: "a", State: "APPROVED", SHA: "h1"},
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
			}},
			wantListed: false,
		},
		{
			name: "別の人の変更要求は、他の人の承認で消えない",
			pr: pullSnapshot{Number: 1, HeadSHA: "h1", Requested: []string{"r"}, Reviews: []pullReview{
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
				{Login: "b", State: "APPROVED", SHA: "h1"},
			}},
			wantListed: false,
		},
		{
			name: "コメントだけでは変更要求を打ち消さない",
			pr: pullSnapshot{Number: 1, HeadSHA: "h2", Reviews: []pullReview{
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
				{Login: "a", State: "COMMENTED", SHA: "h2"},
			}},
			wantListed:   true,
			wantContains: []string{"a"},
		},
		{
			name: "却下された変更要求は載せない",
			pr: pullSnapshot{Number: 1, HeadSHA: "h2", Requested: []string{"r"}, Reviews: []pullReview{
				{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
				{Login: "a", State: "DISMISSED", SHA: "h1"},
			}},
			wantListed:   true,
			wantContains: []string{"r"},
			wantExcludes: []string{"a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectWaiting([]pullSnapshot{tt.pr})
			if (len(got) == 1) != tt.wantListed {
				t.Fatalf("listed = %v, want %v: %+v", len(got) == 1, tt.wantListed, got)
			}
			if !tt.wantListed {
				return
			}
			has := func(login string) bool {
				for _, r := range got[0].Reviewers {
					if r == login {
						return true
					}
				}
				return false
			}
			for _, w := range tt.wantContains {
				if !has(w) {
					t.Errorf("%q が載っていない: %v", w, got[0].Reviewers)
				}
			}
			for _, w := range tt.wantExcludes {
				if has(w) {
					t.Errorf("%q を載せてはいけない: %v", w, got[0].Reviewers)
				}
			}
		})
	}
}

func TestSelectWaiting_承認済みレビュアーを載せない_HeadSHA_h2(t *testing.T) {
	got := SelectWaiting([]pullSnapshot{{
		Number: 7, HeadSHA: "h2",
		Reviews: []pullReview{
			{Login: "a", State: "CHANGES_REQUESTED", SHA: "h1"},
			{Login: "a", State: "APPROVED", SHA: "h2"},
		},
	}})
	if len(got) != 0 {
		t.Fatalf("承認済みの a を再びレビュー待ちに載せた: %+v", got)
	}
}

func TestGitHubReviewClient_最新の意見で履歴の変更要求を打ち消す(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		query = string(body)
		w.Header().Set("Content-Type", "application/json")
		// 履歴(直近10件)には a の変更要求しか残っていないが、最新の意見は新しいコミットへの承認。
		_, _ = io.WriteString(w, `{
		  "data": {"repository": {"pullRequests": {"pageInfo": {"hasNextPage": true}, "nodes": [
		    {"number": 3, "title": "承認済み", "url": "u", "isDraft": false, "headRefOid": "h2",
		     "reviewRequests": {"nodes": []},
		     "reviews": {"nodes": [{"state": "CHANGES_REQUESTED", "author": {"login": "a"}, "commit": {"oid": "h1"}}]},
		     "latestOpinionatedReviews": {"nodes": [{"state": "APPROVED", "author": {"login": "a"}, "commit": {"oid": "h2"}}]}},
		    {"number": 4, "title": "未対応", "url": "u", "isDraft": false, "headRefOid": "h4",
		     "reviewRequests": {"nodes": []},
		     "reviews": {"nodes": [{"state": "CHANGES_REQUESTED", "author": {"login": "b"}, "commit": {"oid": "h3"}}]},
		     "latestOpinionatedReviews": {"nodes": [{"state": "CHANGES_REQUESTED", "author": {"login": "b"}, "commit": {"oid": "h3"}}]}}
		  ]}}}
		}`)
	}))
	defer srv.Close()
	c := &GitHubReviewClient{token: "t", owner: "o", repo: "r", endpoint: srv.URL, http: srv.Client()}
	got, err := c.Waiting(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Number != 4 || got[0].Reviewers[0] != "b" {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(query, "latestOpinionatedReviews") {
		t.Error("最新の意見をクエリしていない")
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
