package resume

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/openai"
	"Backend/internal/usagectx"
)

// #1993: 履歴書レビューの AI 呼び出しに、リクエストの ctx（実行主体）と機能名が届くこと。
// 以前は context.Background() から始めていたので、機能名は unknown、実行主体は空になり、
// 利用者単位の予算ブレーカー（#1776）が履歴書レビューを数えられなかった。
func TestRequestReviewJSON_実行主体と機能名が届く(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","output_text":"{\"items\":[]}","usage":{"input_tokens":100,"output_tokens":20}}`))
	}))
	defer server.Close()

	var got []openai.Usage
	client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
	client.OnUsage = func(u openai.Usage) { got = append(got, u) }
	s := &ResumeService{aiClient: client}

	ctx := usagectx.WithActor(context.Background(), 42, 5)
	if _, err := s.requestReviewJSON(ctx, "system", "user", ""); err != nil {
		t.Fatalf("requestReviewJSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("利用量の記録 = %d件, want 1", len(got))
	}
	u := got[0]
	if u.Feature != usagectx.FeatureResumeReview {
		t.Fatalf("feature = %q, want %q", u.Feature, usagectx.FeatureResumeReview)
	}
	if u.UserID == nil || *u.UserID != 42 || u.OrganizationID == nil || *u.OrganizationID != 5 {
		t.Fatalf("実行主体が届いていない: user=%v org=%v", u.UserID, u.OrganizationID)
	}
}

// クライアントが切断したら AI 呼び出しも止まる（#1779 の残り）。
func TestRequestReviewJSON_切断したら呼ばない(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"output_text":"{}"}`))
	}))
	defer server.Close()
	s := &ResumeService{aiClient: openai.NewWithBaseURL(server.URL, "gpt-4o-mini")}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.requestReviewJSON(ctx, "system", "user", ""); err == nil {
		t.Fatal("切断済みの ctx でもエラーにならない")
	}
	if called {
		t.Fatal("切断済みなのに AI を呼んだ")
	}
}
