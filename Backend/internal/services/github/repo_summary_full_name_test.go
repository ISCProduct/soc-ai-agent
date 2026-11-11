package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// invalidRepoFullNames 形が正しくない full_name。どれも GitHub API を呼ばずに弾く（#2176）。
var invalidRepoFullNames = []struct {
	name     string
	fullName string
}{
	{"空", ""},
	{"スラッシュが無い", "ownerrepo"},
	{"スラッシュが2つ", "owner/repo/x"},
	{"上の階層へ出る", "../users/x"},
	{"リポジトリの後ろで上の階層へ戻る", "owner/repo/../../x"},
	{"クエリを付ける", "owner/repo?x=1"},
	{"フラグメントを付ける", "owner/repo#x"},
	{"オーナーが空", "/repo"},
	{"リポジトリが空", "owner/"},
	{"リポジトリが「.」", "owner/."},
	{"リポジトリが「..」", "owner/.."},
	{"オーナーが「..」", "../repo"},
	{"オーナーが40字", strings.Repeat("a", 40) + "/repo"},
	{"リポジトリが101字", "owner/" + strings.Repeat("r", 101)},
	{"オーナーの先頭がハイフン", "-owner/repo"},
	{"オーナーの末尾がハイフン", "owner-/repo"},
	{"オーナーにドット", "own.er/repo"},
	{"オーナーにアンダースコア", "own_er/repo"},
	{"パーセントエンコード", "owner/repo%2F..%2F..%2Fuser"},
	{"空白", "owner/re po"},
	{"改行", "owner/repo\n"},
	{"バックスラッシュ", "owner/..\\user"},
	{"英数字以外の文字", "owner/リポジトリ"},
}

// validRepoFullNames GitHub で使える full_name。最大長・記号入りも通す。
var validRepoFullNames = []struct {
	name     string
	fullName string
}{
	{"基本", "owner/repo"},
	{"オーナー1字・リポジトリ1字", "a/b"},
	{"オーナーにハイフン", "my-org/repo"},
	// 今は作れないが、古いアカウントには連続したハイフンのユーザー名が残っているため通す
	{"オーナーに連続したハイフン", "a--b/repo"},
	{"リポジトリにドット・ハイフン・アンダースコア", "owner/my.repo-name_2"},
	{"リポジトリがドットで始まる", "owner/.github"},
	{"リポジトリが「...」", "owner/..."},
	{"大文字と数字", "Owner01/Repo99"},
	{"オーナー39字", strings.Repeat("a", 39) + "/repo"},
	{"リポジトリ100字", "owner/" + strings.Repeat("r", 100)},
	{"どちらも最大長", strings.Repeat("a", 38) + "z/" + strings.Repeat("r", 100)},
}

func TestValidateRepoFullName(t *testing.T) {
	for _, tc := range invalidRepoFullNames {
		t.Run("拒否_"+tc.name, func(t *testing.T) {
			if err := ValidateRepoFullName(tc.fullName); !errors.Is(err, ErrInvalidRepoFullName) {
				t.Fatalf("ValidateRepoFullName(%q) = %v, want ErrInvalidRepoFullName", tc.fullName, err)
			}
		})
	}
	for _, tc := range validRepoFullNames {
		t.Run("許可_"+tc.name, func(t *testing.T) {
			if err := ValidateRepoFullName(tc.fullName); err != nil {
				t.Fatalf("ValidateRepoFullName(%q) = %v, want nil", tc.fullName, err)
			}
		})
	}
}

// 形が正しくない full_name では、キャッシュ・プロフィールの読み出しより前に止め、GitHub API へは何も送らない。
// githubRepo を nil にしているので、入口の検証より先に進めば panic で落ちる。
func TestSummarizeRepo_InvalidFullNameDoesNotCallGitHub(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := &GitHubService{apiBaseURL: server.URL}
	for _, tc := range invalidRepoFullNames {
		for _, force := range []bool{false, true} {
			got, err := s.SummarizeRepo(context.Background(), 1, tc.fullName, force, "")
			if !errors.Is(err, ErrInvalidRepoFullName) || got != nil {
				t.Errorf("%s (force=%v): got=%v err=%v, want ErrInvalidRepoFullName", tc.name, force, got, err)
			}
		}
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("GitHub API へ %d 回リクエストが送られた。0 回であるべき", n)
	}
}

// 正しい full_name では、従来どおり /repos/{owner}/{repo}/readme を読み、要約を返して保存する。
func TestSummarizeRepo_ValidFullNameReturnsSummary(t *testing.T) {
	// 暗号化キーが手元の環境にあると平文のトークンの復号に失敗するため、空にして平文として扱わせる
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	const fullName = "my-org/my.repo_name-2"
	var readmePaths []string
	var authHeader string
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		readmePaths = append(readmePaths, r.URL.Path)
		authHeader = r.Header.Get("Authorization")
		content := base64.StdEncoding.EncodeToString([]byte("# my repo\nGo で作った API サーバー"))
		_ = json.NewEncoder(w).Encode(map[string]string{"content": content, "encoding": "base64"})
	}))
	defer githubServer.Close()

	var userPrompt string
	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			if m.Role == "user" {
				userPrompt = m.Content
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary_text\":\"Go の API\",\"tech_reason\":\"速い\",\"challenge\":\"並行処理\",\"achievement\":\"応答を半分に\"}"}}]}`))
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
	mock.ExpectExec("INSERT INTO `git_hub_repo_summaries`").
		WillReturnResult(sqlmock.NewResult(1, 1))

	s := &GitHubService{
		githubRepo:   repositories.NewGitHubRepository(db),
		apiBaseURL:   githubServer.URL,
		openaiClient: openai.NewWithBaseURL(openaiServer.URL, "gpt-4o-mini"),
	}
	// forceRefresh=true でキャッシュの読み出しを飛ばし、README の取得から要約・保存までを通す
	got, err := s.SummarizeRepo(context.Background(), 7, fullName, true, "")
	if err != nil {
		t.Fatalf("SummarizeRepo error: %v", err)
	}
	if got.UserID != 7 || got.FullName != fullName || got.SummaryText != "Go の API" || got.Achievement != "応答を半分に" {
		t.Fatalf("要約が想定と違う: %+v", got)
	}
	if len(readmePaths) != 1 || readmePaths[0] != "/repos/"+fullName+"/readme" {
		t.Fatalf("README の取得先 = %v, want [/repos/%s/readme]", readmePaths, fullName)
	}
	if authHeader != "Bearer plain-token" {
		t.Fatalf("Authorization = %q", authHeader)
	}
	if !strings.Contains(userPrompt, "Go で作った API サーバー") {
		t.Fatalf("README の本文がプロンプトに入っていない:\n%s", userPrompt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
