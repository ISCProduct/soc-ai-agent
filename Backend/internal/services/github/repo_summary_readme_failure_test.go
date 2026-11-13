package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"Backend/internal/openai"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// README の取得結果ごとの扱いを固定する（#2226）。
// 404 は「README なし」で要約し、それ以外の失敗では要約を作らず（OpenAI を呼ばず）、保存もしない。
func TestSummarizeRepo_READMEFetchResult(t *testing.T) {
	// 暗号化キーが手元の環境にあると平文のトークンの復号に失敗するため、空にして平文として扱わせる
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	cases := []struct {
		name string
		// README の応答
		status  int
		headers map[string]string
		// README の応答の本文。空なら {"message":"error"}
		body string
		// リポジトリ自体（/repos/{owner}/{repo}）の応答。0 なら 200（README が 404 のときだけ呼ばれる）
		repoStatus int
		// 期待
		wantSummary bool
		// 要約するとき、プロンプトに含まれるべき文字列
		wantPromptPart string
		// 空でなければ GitHubReauthRequiredError を期待し、その文言と比べる
		wantReauthReason string
		wantErrPart      string
	}{
		{
			name:           "README が取れれば、その README で要約する",
			status:         http.StatusOK,
			body:           `{"encoding":"base64","content":"` + base64.StdEncoding.EncodeToString([]byte("# CLI ツール")) + `"}`,
			wantSummary:    true,
			wantPromptPart: "# CLI ツール",
		},
		{name: "README が 404 でリポジトリにはアクセスできれば、README なしで要約する", status: http.StatusNotFound, wantSummary: true, wantPromptPart: "READMEなし"},
		// #2231 のレビュー：README の 404 は、リポジトリが削除・非公開化されたときも返る
		{name: "README もリポジトリも 404 なら要約を作らずエラー", status: http.StatusNotFound, repoStatus: http.StatusNotFound, wantErrPart: "repository not accessible"},
		{name: "README が 404 でリポジトリが 401 なら再連携を促す", status: http.StatusNotFound, repoStatus: http.StatusUnauthorized, wantReauthReason: reauthMessageTokenInvalid},
		{name: "README が 404 でリポジトリが 403 なら権限の確認と再連携を促す", status: http.StatusNotFound, repoStatus: http.StatusForbidden, wantReauthReason: reauthMessageForbidden},
		{name: "500 は要約を作らずエラー", status: http.StatusInternalServerError, wantErrPart: "failed to fetch README"},
		// #2231 のレビュー（CodeRabbit）：本文の文字列ではなくステータスで判定するので、404 扱いにしない
		{
			name:        "本文に status 404 を含む 500 も 404 扱いにせず、要約を作らずエラー",
			status:      http.StatusInternalServerError,
			body:        `{"message":"github api error: status 404"}`,
			wantErrPart: "failed to fetch README",
		},
		{name: "502 も要約を作らずエラー", status: http.StatusBadGateway, wantErrPart: "failed to fetch README"},
		{
			name:        "レート制限は要約を作らずエラー",
			status:      http.StatusForbidden,
			headers:     map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1893456000"},
			wantErrPart: "failed to fetch README",
		},
		// 二次レート制限は残り回数が 0 でなくても 403 + Retry-After で返る。権限不足と取り違えて再連携を促さない
		{
			name:        "二次レート制限（403 + Retry-After）は再連携を促さず、要約を作らずエラー",
			status:      http.StatusForbidden,
			headers:     map[string]string{"X-RateLimit-Remaining": "4000", "Retry-After": "60"},
			wantErrPart: "failed to fetch README",
		},
		{name: "401 は再連携を促す", status: http.StatusUnauthorized, wantReauthReason: reauthMessageTokenInvalid},
		// #2231 のレビュー：SSO 未承認・権限不足の 403 は 500 にせず、権限の確認と再連携を促す
		{
			name:             "レート制限以外の 403 は権限の確認と再連携を促す",
			status:           http.StatusForbidden,
			headers:          map[string]string{"X-RateLimit-Remaining": "4000"},
			wantReauthReason: reauthMessageForbidden,
		},
		// #2231 のレビュー：解析の失敗は取得の失敗と文言を分ける
		{name: "README の応答が JSON でなければ、解析の失敗として要約を作らない", status: http.StatusOK, body: `not json`, wantErrPart: "failed to parse README"},
		{name: "README の base64 が壊れていれば、解析の失敗として要約を作らない", status: http.StatusOK, body: `{"encoding":"base64","content":"!!!"}`, wantErrPart: "failed to parse README"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var repoRequests atomic.Int32
			githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/repos/owner/repo" {
					repoRequests.Add(1)
					status := tc.repoStatus
					if status == 0 {
						status = http.StatusOK
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"full_name":"owner/repo"}`))
					return
				}
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.status)
				body := tc.body
				if body == "" {
					body = `{"message":"error"}`
				}
				_, _ = w.Write([]byte(body))
			}))
			defer githubServer.Close()

			var openaiCalls atomic.Int32
			var userPrompt string
			openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				openaiCalls.Add(1)
				body, _ := io.ReadAll(r.Body)
				userPrompt = string(body)
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary_text\":\"小さな CLI\",\"tech_reason\":\"\",\"challenge\":\"\",\"achievement\":\"\"}"}}]}`))
			}))
			defer openaiServer.Close()

			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), SkipDefaultTransaction: true})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery("SELECT \\* FROM `git_hub_profiles`").
				WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "git_hub_login", "access_token"}).
					AddRow(1, 7, "me", "plain-token"))
			if tc.wantSummary {
				mock.ExpectExec("INSERT INTO `git_hub_repo_summaries`").
					WillReturnResult(sqlmock.NewResult(1, 1))
			}

			s := &GitHubService{
				githubRepo:   repositories.NewGitHubRepository(db),
				apiBaseURL:   githubServer.URL,
				openaiClient: openai.NewWithBaseURL(openaiServer.URL, "gpt-4o-mini"),
			}
			got, err := s.SummarizeRepo(context.Background(), 7, "owner/repo", true, "")

			switch {
			case tc.wantSummary:
				if err != nil {
					t.Fatalf("SummarizeRepo error: %v", err)
				}
				if got.SummaryText != "小さな CLI" {
					t.Fatalf("要約が想定と違う: %+v", got)
				}
				if !strings.Contains(userPrompt, tc.wantPromptPart) {
					t.Fatalf("プロンプトに %q が含まれていない:\n%s", tc.wantPromptPart, userPrompt)
				}
			case tc.wantReauthReason != "":
				var reauth *GitHubReauthRequiredError
				if !errors.As(err, &reauth) {
					t.Fatalf("err = %v, want GitHubReauthRequiredError", err)
				}
				if reauth.Reason != tc.wantReauthReason {
					t.Fatalf("文言 = %q, want %q", reauth.Reason, tc.wantReauthReason)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Fatalf("err = %v, want %q を含むエラー", err, tc.wantErrPart)
				}
				var reauth *GitHubReauthRequiredError
				if errors.As(err, &reauth) {
					t.Fatalf("再連携のエラーにしない: %v", err)
				}
			}
			if !tc.wantSummary {
				if got != nil {
					t.Fatalf("要約を返した: %+v", got)
				}
				if n := openaiCalls.Load(); n != 0 {
					t.Fatalf("要約を作らないはずが OpenAI を %d 回呼んだ", n)
				}
			}
			// リポジトリ自体の確認は、README が 404 のときだけ行う
			wantRepoRequests := int32(0)
			if tc.status == http.StatusNotFound {
				wantRepoRequests = 1
			}
			if n := repoRequests.Load(); n != wantRepoRequests {
				t.Fatalf("リポジトリの確認の回数 = %d, want %d", n, wantRepoRequests)
			}
			// 失敗のときは INSERT の期待を置いていないので、保存すれば sqlmock が失敗する
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// GitHub API のエラーの判定は、エラーの文字列ではなく GitHubAPIError のステータスで行う（#2231 のレビュー）。
// 文字列が同じでも型が違えば一致させず、%w で包まれていても判定できることを固定する。
func TestGitHubAPIErrorPredicates(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		wantNotFound  bool
		wantAuth      bool
		wantForbidden bool
	}{
		{name: "404", err: &GitHubAPIError{StatusCode: http.StatusNotFound}, wantNotFound: true},
		{name: "包まれた 404", err: fmt.Errorf("fetch: %w", &GitHubAPIError{StatusCode: http.StatusNotFound}), wantNotFound: true},
		{name: "401", err: &GitHubAPIError{StatusCode: http.StatusUnauthorized}, wantAuth: true},
		// 同期は fetchRepositories が "fetch repositories: %w" で包んだエラーを isGitHubAuthError に渡す
		{name: "同期と同じく包まれた 401", err: fmt.Errorf("fetch repositories: %w", &GitHubAPIError{StatusCode: http.StatusUnauthorized}), wantAuth: true},
		{name: "403", err: &GitHubAPIError{StatusCode: http.StatusForbidden}, wantForbidden: true},
		{name: "500", err: &GitHubAPIError{StatusCode: http.StatusInternalServerError}},
		{name: "400", err: &GitHubAPIError{StatusCode: http.StatusBadRequest}},
		{name: "同じ文字列でも GitHubAPIError でなければ 404 にしない", err: errors.New("github api error: status 404")},
		{name: "同じ文字列でも GitHubAPIError でなければ 401 にしない", err: errors.New("github api error: status 401: bad credentials")},
		{name: "レート制限は 403 扱いにしない", err: &rateLimitError{}},
		{name: "nil", err: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGitHubNotFoundError(tc.err); got != tc.wantNotFound {
				t.Errorf("isGitHubNotFoundError = %v, want %v", got, tc.wantNotFound)
			}
			if got := isGitHubAuthError(tc.err); got != tc.wantAuth {
				t.Errorf("isGitHubAuthError = %v, want %v", got, tc.wantAuth)
			}
			if got := isGitHubForbiddenError(tc.err); got != tc.wantForbidden {
				t.Errorf("isGitHubForbiddenError = %v, want %v", got, tc.wantForbidden)
			}
		})
	}
}

// doGet は 4xx・5xx をステータスを持つ GitHubAPIError で返し、応答の本文はエラーに入れない（ログに本文を出さないため）。
// 成功したときは本文をそのまま返す。
func TestDoGet_StatusError(t *testing.T) {
	const secretBody = `{"message":"Resource protected by organization SAML enforcement","org":"secret-org"}`
	cases := []struct {
		name       string
		status     int
		wantStatus int // 0 ならエラーにならない
	}{
		{name: "200 は本文を返す", status: http.StatusOK},
		{name: "399 はエラーにしない", status: 399},
		{name: "400 はエラー", status: http.StatusBadRequest, wantStatus: http.StatusBadRequest},
		{name: "403 はエラー", status: http.StatusForbidden, wantStatus: http.StatusForbidden},
		{name: "404 はエラー", status: http.StatusNotFound, wantStatus: http.StatusNotFound},
		{name: "500 はエラー", status: http.StatusInternalServerError, wantStatus: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(secretBody))
			}))
			defer server.Close()

			s := &GitHubService{}
			body, err := s.doGet(context.Background(), server.Client(), "token", server.URL+"/x")
			if tc.wantStatus == 0 {
				if err != nil {
					t.Fatalf("doGet error: %v", err)
				}
				if string(body) != secretBody {
					t.Fatalf("本文 = %q, want %q", body, secretBody)
				}
				return
			}
			var apiErr *GitHubAPIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.wantStatus {
				t.Fatalf("err = %v, want GitHubAPIError{StatusCode: %d}", err, tc.wantStatus)
			}
			if strings.Contains(err.Error(), "secret-org") || strings.Contains(err.Error(), "SAML") {
				t.Fatalf("エラーに応答の本文が入っている: %v", err)
			}
		})
	}
}

// Retry-After の無い二次レート制限は、本文の message で判別してレート制限として扱う（#2266 のレビュー）。
// 権限不足の 403 と取り違えると、利用者に解決できない再連携を促してしまう。
func TestDoGet_SecondaryRateLimitWithoutRetryAfter(t *testing.T) {
	cases := []struct {
		name          string
		status        int
		body          string
		wantRateLimit bool
	}{
		{"403 で本文が二次レート制限", http.StatusForbidden, `{"message":"You have exceeded a secondary rate limit. Please wait a few minutes before you try again."}`, true},
		{"大文字小文字を問わない", http.StatusForbidden, `{"message":"You Have Exceeded A Secondary Rate Limit"}`, true},
		{"403 で権限不足の本文は GitHubAPIError のまま", http.StatusForbidden, `{"message":"Resource protected by organization SAML enforcement"}`, false},
		{"404 の本文に同じ語があってもレート制限にしない", http.StatusNotFound, `{"message":"secondary rate limit"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := (&GitHubService{}).doGet(context.Background(), server.Client(), "token", server.URL+"/x")
			if got := IsRateLimitError(err); got != tc.wantRateLimit {
				t.Fatalf("IsRateLimitError = %v, want %v（err = %v）", got, tc.wantRateLimit, err)
			}
			if !tc.wantRateLimit && githubAPIStatus(err) != tc.status {
				t.Fatalf("err = %v, want GitHubAPIError{%d}", err, tc.status)
			}
			if strings.Contains(err.Error(), "message") {
				t.Fatalf("エラーに本文が入っている: %v", err)
			}
		})
	}
}

// trackingBody は、読み切られたか（EOF まで読まれたか）を記録する本文。
type trackingBody struct {
	r       *strings.Reader
	drained bool
}

func (b *trackingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if err == io.EOF {
		b.drained = true
	}
	return n, err
}
func (b *trackingBody) Close() error { return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// エラー応答の本文は、上限付きで読み切ってから捨てる（HTTP/1.x で接続を再利用するため。#2266 のレビュー）。
func TestDoGet_DrainsErrorBody(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		headers map[string]string
	}{
		{"404", http.StatusNotFound, nil},
		{"500", http.StatusInternalServerError, nil},
		{"レート制限（CheckRateLimit で返る経路）", http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackingBody{r: strings.NewReader(strings.Repeat("x", 10_000))}
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				h := http.Header{}
				for k, v := range tc.headers {
					h.Set(k, v)
				}
				return &http.Response{StatusCode: tc.status, Header: h, Body: body, Request: r}, nil
			})}
			if _, err := (&GitHubService{}).doGet(context.Background(), client, "token", "https://example.test/x"); err == nil {
				t.Fatal("エラーにならなかった")
			}
			if !body.drained {
				t.Fatal("エラー応答の本文を読み切らずに返した")
			}
		})
	}
}
