package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/piimask"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// README に書かれた学生本人の氏名・メール・学校名は、外部 LLM へ送る前に伏字にする（SOCAIAGENT-697）。
// 登録情報を読めず伏字を準備できないときは、LLM を呼ばず、保存もせず、要約の失敗にする。
// 氏名・メール・学校名はすべて合成データ。
func TestSummarizeRepo_PIIGuard(t *testing.T) {
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	const (
		name   = "試験 花子"
		email  = "hanako.sample@example.com"
		school = "架空情報専門学校"
	)
	readme := "# ポートフォリオ\n作者: " + name + "\n連絡先: " + email + "\n所属: " + school + "\nGo で作った CLI です。"

	cases := []struct {
		name       string
		subjects   piimask.SubjectReader
		wantErr    bool
		wantCalled bool
	}{
		{
			name: "登録情報で辞書を作り、氏名・メール・学校名を伏字にして送る",
			subjects: piimask.SubjectReaderFunc(func(uint) (piimask.Subject, error) {
				return piimask.Subject{Name: name, Email: email, SchoolName: school}, nil
			}),
			wantCalled: true,
		},
		{
			name: "登録情報を読めなければ LLM を呼ばずに要約の失敗にする",
			subjects: piimask.SubjectReaderFunc(func(uint) (piimask.Subject, error) {
				return piimask.Subject{}, errors.New("db down")
			}),
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"encoding":"base64","content":"` + base64.StdEncoding.EncodeToString([]byte(readme)) + `"}`))
			}))
			defer githubServer.Close()

			var calls atomic.Int32
			var userPrompt string
			openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var body struct {
					Messages []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"messages"`
				}
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &body)
				for _, m := range body.Messages {
					if m.Role == "user" {
						userPrompt = m.Content
					}
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary_text\":\"CLI\"}"}}]}`))
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
			if !tc.wantErr {
				mock.ExpectExec("INSERT INTO `git_hub_repo_summaries`").WillReturnResult(sqlmock.NewResult(1, 1))
			}

			s := &GitHubService{
				githubRepo:   repositories.NewGitHubRepository(db),
				apiBaseURL:   githubServer.URL,
				openaiClient: openai.NewWithBaseURL(openaiServer.URL, "gpt-4o-mini"),
			}
			s.SetSubjectReader(tc.subjects)
			got, err := s.SummarizeRepo(context.Background(), 7, "owner/repo", true, "")

			if tc.wantErr {
				if !errors.Is(err, openai.ErrInputGuardFailed) || !strings.Contains(err.Error(), "AI summary generation failed") {
					t.Fatalf("err = %v, want 要約の失敗（ErrInputGuardFailed）", err)
				}
				if got != nil {
					t.Fatalf("要約を返した: %+v", got)
				}
			} else if err != nil {
				t.Fatalf("SummarizeRepo error: %v", err)
			}
			if n := calls.Load(); (n > 0) != tc.wantCalled {
				t.Fatalf("OpenAI の呼び出し回数 = %d, wantCalled = %v", n, tc.wantCalled)
			}
			if tc.wantCalled {
				for _, pii := range []string{"試験", "花子", email, "hanako.sample", school} {
					if strings.Contains(userPrompt, pii) {
						t.Errorf("プロンプトに伏字前の %q が残っている:\n%s", pii, userPrompt)
					}
				}
				if !strings.Contains(userPrompt, "Go で作った CLI です。") {
					t.Errorf("README の本文が送られていない:\n%s", userPrompt)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
