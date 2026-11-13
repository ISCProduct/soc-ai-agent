package github

import (
	"context"
	"errors"
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
		// 期待
		wantSummary bool
		wantReauth  bool
		wantErrPart string
	}{
		{name: "404 は README なしで要約する", status: http.StatusNotFound, wantSummary: true},
		{name: "500 は要約を作らずエラー", status: http.StatusInternalServerError, wantErrPart: "failed to fetch README"},
		{name: "502 も要約を作らずエラー", status: http.StatusBadGateway, wantErrPart: "failed to fetch README"},
		{
			name:        "レート制限は要約を作らずエラー",
			status:      http.StatusForbidden,
			headers:     map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1893456000"},
			wantErrPart: "failed to fetch README",
		},
		{name: "401 は再連携を促す", status: http.StatusUnauthorized, wantReauth: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"error"}`))
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
				if !strings.Contains(userPrompt, "READMEなし") {
					t.Fatalf("README なしとしてプロンプトに書かれていない:\n%s", userPrompt)
				}
			case tc.wantReauth:
				var reauth *GitHubReauthRequiredError
				if !errors.As(err, &reauth) {
					t.Fatalf("err = %v, want GitHubReauthRequiredError", err)
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
			// 失敗のときは INSERT の期待を置いていないので、保存すれば sqlmock が失敗する
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIsGitHubNotFoundError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("github api error: status 404: {\"message\":\"Not Found\"}"), true},
		{errors.New("github api error: status 401: bad credentials"), false},
		{errors.New("github api error: status 500: 404 page"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := isGitHubNotFoundError(tc.err); got != tc.want {
			t.Errorf("isGitHubNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
