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
	"unicode/utf8"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newSummaryTestDB sqlmock につないだ GORM を返す。
func newSummaryTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

// summaryTestServers README を返す GitHub と、要約を返す OpenAI の偽サーバー。呼ばれた回数を数える。
type summaryTestServers struct {
	github, openai           *httptest.Server
	githubCalls, openaiCalls atomic.Int32
}

func newSummaryTestServers(t *testing.T, openaiContent string) *summaryTestServers {
	t.Helper()
	ts := &summaryTestServers{}
	ts.github = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.githubCalls.Add(1)
		content := base64.StdEncoding.EncodeToString([]byte("# repo\nGo の API サーバー"))
		_ = json.NewEncoder(w).Encode(map[string]string{"content": content, "encoding": "base64"})
	}))
	ts.openai = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.openaiCalls.Add(1)
		resp := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": openaiContent}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(func() {
		ts.github.Close()
		ts.openai.Close()
	})
	return ts
}

var summaryColumns = []string{"id", "user_id", "full_name", "summary_text", "tech_reason", "challenge", "achievement", "prompt_version"}

const newSummaryJSON = `{"summary_text":"新しい要約","tech_reason":"理由","challenge":"課題","achievement":"成果"}`

// force_refresh=false でキャッシュがある場合。今の版なら返し、古い版・空の版なら作り直して今の版で保存する（#2185）。
func TestSummarizeRepo_CacheByPromptVersion(t *testing.T) {
	// 暗号化キーが手元の環境にあると平文のトークンの復号に失敗するため、空にして平文として扱わせる
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	tests := []struct {
		name          string
		cachedVersion string
		wantRefresh   bool
	}{
		{"今の版のキャッシュはそのまま返す", repoSummaryPromptVersion, false},
		{"古い版のキャッシュは作り直す", "v3", true},
		{"版を記録する前の行（空）は作り直す", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newSummaryTestServers(t, newSummaryJSON)
			db, mock := newSummaryTestDB(t)
			mock.ExpectQuery("SELECT \\* FROM `git_hub_repo_summaries` WHERE user_id = \\? AND full_name = \\?").
				WithArgs(7, "owner/repo", 1).
				WillReturnRows(sqlmock.NewRows(summaryColumns).
					AddRow(1, 7, "owner/repo", "古い要約", "古い理由", "古い課題", "古い成果", tc.cachedVersion))
			if tc.wantRefresh {
				mock.ExpectQuery("SELECT \\* FROM `git_hub_profiles`").
					WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "git_hub_login", "access_token"}).
						AddRow(1, 7, "me", "plain-token"))
				// 版のカラムを INSERT し、既存の行と重なったときも版を更新する
				mock.ExpectExec("INSERT INTO `git_hub_repo_summaries` .*`prompt_version`.*ON DUPLICATE KEY UPDATE .*`prompt_version`=VALUES\\(`prompt_version`\\)").
					WillReturnResult(sqlmock.NewResult(1, 1))
			}

			s := &GitHubService{
				githubRepo:   repositories.NewGitHubRepository(db),
				apiBaseURL:   ts.github.URL,
				openaiClient: openai.NewWithBaseURL(ts.openai.URL, "gpt-4o-mini"),
			}
			got, err := s.SummarizeRepo(context.Background(), 7, "owner/repo", false, "")
			if err != nil {
				t.Fatalf("SummarizeRepo error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}

			wantCalls := int32(0)
			if tc.wantRefresh {
				wantCalls = 1
			}
			if n := ts.openaiCalls.Load(); n != wantCalls {
				t.Errorf("OpenAI の呼び出し = %d 回, want %d", n, wantCalls)
			}
			if n := ts.githubCalls.Load(); n != wantCalls {
				t.Errorf("README の取得 = %d 回, want %d", n, wantCalls)
			}
			if tc.wantRefresh {
				if got.SummaryText != "新しい要約" || got.PromptVersion != repoSummaryPromptVersion || got.UserID != 7 {
					t.Errorf("作り直した要約が想定と違う: %+v", got)
				}
			} else {
				if got.SummaryText != "古い要約" || got.PromptVersion != repoSummaryPromptVersion {
					t.Errorf("キャッシュをそのまま返していない: %+v", got)
				}
			}
		})
	}
}

// 一覧は今の版で作られた要約だけを返す。古い版の要約（虚偽の実績を含みうる）は出さない（#2185）。
func TestListRepoSummaries_OnlyCurrentPromptVersion(t *testing.T) {
	db, mock := newSummaryTestDB(t)
	mock.ExpectQuery("SELECT \\* FROM `git_hub_repo_summaries` WHERE user_id = \\? AND prompt_version = \\? ORDER BY updated_at desc").
		WithArgs(7, repoSummaryPromptVersion).
		WillReturnRows(sqlmock.NewRows(summaryColumns).
			AddRow(1, 7, "owner/repo", "要約", "", "", "", repoSummaryPromptVersion))

	s := &GitHubService{githubRepo: repositories.NewGitHubRepository(db)}
	got, err := s.ListRepoSummaries(7)
	if err != nil {
		t.Fatalf("ListRepoSummaries error: %v", err)
	}
	if len(got) != 1 || got[0].FullName != "owner/repo" {
		t.Fatalf("一覧が想定と違う: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		maxRunes int
		want     string
	}{
		{"空", "", 3, ""},
		{"上限より短い", "ab", 3, "ab"},
		{"上限ちょうどの英字", "abc", 3, "abc"},
		{"上限を超える英字", "abcd", 3, "abc"},
		{"上限ちょうどの日本語", "あいう", 3, "あいう"},
		{"上限を超える日本語", "あいうえ", 3, "あいう"},
		{"英字と日本語の混在", "aあbい", 3, "aあb"},
		{"上限0", "abc", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateRunes(tc.in, tc.maxRunes); got != tc.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tc.in, tc.maxRunes, got, tc.want)
			}
		})
	}
}

// README の切り詰めはルーン単位。4000 バイトの境界に日本語がかかっても、文字の途中で切れない（#2185）。
func TestFetchREADME_TruncatesByRunes(t *testing.T) {
	tests := []struct {
		name   string
		readme string
		want   string
	}{
		// "a" + "あ"×2000 は 6001 バイト。4000 バイト目は「あ」（3バイト）の途中にかかる。
		// ルーンでは 2001 字なので、上限 4000 字に収まり、そのまま返す。
		{"4000バイトの境界に日本語がかかる（4000字以内）", "a" + strings.Repeat("あ", 2000), "a" + strings.Repeat("あ", 2000)},
		{"日本語ちょうど4000字", strings.Repeat("あ", 4000), strings.Repeat("あ", 4000)},
		{"日本語4001字は4000字に切る", strings.Repeat("あ", 4000) + "い", strings.Repeat("あ", 4000)},
		{"英字4001字は4000字に切る", strings.Repeat("a", 4001), strings.Repeat("a", 4000)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				content := base64.StdEncoding.EncodeToString([]byte(tc.readme))
				_ = json.NewEncoder(w).Encode(map[string]string{"content": content, "encoding": "base64"})
			}))
			defer server.Close()

			s := &GitHubService{apiBaseURL: server.URL}
			got, err := s.fetchREADME(context.Background(), server.Client(), "token", "owner/repo")
			if err != nil {
				t.Fatalf("fetchREADME error: %v", err)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("UTF-8 として不正な文字列になった（文字の途中で切れている）")
			}
			if got != tc.want {
				t.Errorf("README = %d 字（%d バイト）, want %d 字（%d バイト）",
					utf8.RuneCountInString(got), len(got), utf8.RuneCountInString(tc.want), len(tc.want))
			}
		})
	}
}

func TestValidateRepoSummaryOutput(t *testing.T) {
	atLimit := strings.Repeat("あ", maxRepoSummaryFieldRunes)
	overLimit := strings.Repeat("あ", maxRepoSummaryFieldRunes+1)

	valid := []struct {
		name string
		in   models.GitHubRepoSummary
	}{
		{"全フィールドあり", models.GitHubRepoSummary{SummaryText: "要約", TechReason: "理由", Challenge: "課題", Achievement: "成果"}},
		{"summary_text 以外は空でよい", models.GitHubRepoSummary{SummaryText: "要約"}},
		{"全フィールドが上限ちょうど", models.GitHubRepoSummary{SummaryText: atLimit, TechReason: atLimit, Challenge: atLimit, Achievement: atLimit}},
	}
	for _, tc := range valid {
		t.Run("許可_"+tc.name, func(t *testing.T) {
			if err := validateRepoSummaryOutput(&tc.in); err != nil {
				t.Fatalf("validateRepoSummaryOutput = %v, want nil", err)
			}
		})
	}

	invalid := []struct {
		name string
		in   models.GitHubRepoSummary
	}{
		{"summary_text が空", models.GitHubRepoSummary{TechReason: "理由"}},
		{"summary_text が上限を超える", models.GitHubRepoSummary{SummaryText: overLimit}},
		{"tech_reason が上限を超える", models.GitHubRepoSummary{SummaryText: "要約", TechReason: overLimit}},
		{"challenge が上限を超える", models.GitHubRepoSummary{SummaryText: "要約", Challenge: overLimit}},
		{"achievement が上限を超える", models.GitHubRepoSummary{SummaryText: "要約", Achievement: overLimit}},
	}
	for _, tc := range invalid {
		t.Run("拒否_"+tc.name, func(t *testing.T) {
			if err := validateRepoSummaryOutput(&tc.in); !errors.Is(err, ErrInvalidRepoSummaryOutput) {
				t.Fatalf("validateRepoSummaryOutput = %v, want ErrInvalidRepoSummaryOutput", err)
			}
		})
	}
}

// モデルの出力の検証を generateRepoSummary が通していること。上限ちょうどは保存し、超える・形式が違う出力は返さない。
func TestGenerateRepoSummary_ValidatesOutput(t *testing.T) {
	atLimit := strings.Repeat("あ", maxRepoSummaryFieldRunes)
	overLimit := strings.Repeat("あ", maxRepoSummaryFieldRunes+1)
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"上限ちょうどは受け付ける", `{"summary_text":"` + atLimit + `","achievement":"` + atLimit + `"}`, false},
		{"前後の空白は取り除く", `{"summary_text":"  要約\n","achievement":" 成果 "}`, false},
		{"achievement が上限を超える", `{"summary_text":"要約","achievement":"` + overLimit + `"}`, true},
		{"summary_text が空白だけ", `{"summary_text":"  ","achievement":"成果"}`, true},
		{"null", `null`, true},
		{"空のオブジェクト", `{}`, true},
		// 配列は extractRepoSummaryJSON が「{」より前と「}」より後を落とすため、今の解釈では通る。ここでは JSON でない文を見る
		{"JSON でない文", `要約です`, true},
		{"文字列でない値", `{"summary_text":123}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": tc.content}}}}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			s := &GitHubService{openaiClient: openai.NewWithBaseURL(server.URL, "gpt-4o-mini")}
			got, err := s.generateRepoSummary(context.Background(), "owner/repo", "README", "")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("エラーにならなかった: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("generateRepoSummary error: %v", err)
			}
			if got.PromptVersion != repoSummaryPromptVersion {
				t.Errorf("PromptVersion = %q, want %q", got.PromptVersion, repoSummaryPromptVersion)
			}
			if strings.TrimSpace(got.SummaryText) != got.SummaryText || strings.TrimSpace(got.Achievement) != got.Achievement {
				t.Errorf("前後の空白が残っている: %+v", got)
			}
		})
	}
}

// 上限を超える出力は保存しない。キャッシュも書き換えないので、次の依頼で作り直される。
func TestSummarizeRepo_InvalidOutputIsNotSaved(t *testing.T) {
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	overLimit := strings.Repeat("あ", maxRepoSummaryFieldRunes+1)
	ts := newSummaryTestServers(t, `{"summary_text":"`+overLimit+`"}`)
	db, mock := newSummaryTestDB(t)
	mock.ExpectQuery("SELECT \\* FROM `git_hub_profiles`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "git_hub_login", "access_token"}).
			AddRow(1, 7, "me", "plain-token"))
	// INSERT は期待しない。呼ばれれば sqlmock がエラーにし、ExpectationsWereMet でも検出する。

	s := &GitHubService{
		githubRepo:   repositories.NewGitHubRepository(db),
		apiBaseURL:   ts.github.URL,
		openaiClient: openai.NewWithBaseURL(ts.openai.URL, "gpt-4o-mini"),
	}
	got, err := s.SummarizeRepo(context.Background(), 7, "owner/repo", true, "")
	if !errors.Is(err, ErrInvalidRepoSummaryOutput) || got != nil {
		t.Fatalf("got=%+v err=%v, want ErrInvalidRepoSummaryOutput", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
