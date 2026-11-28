package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/openai"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// expectOwnRepos 本人が作成者・共同編集者の同期済みリポジトリの読み出しを期待する（SOCAIAGENT-698）。
func expectOwnRepos(mock sqlmock.Sqlmock, userID uint, names ...string) {
	rows := sqlmock.NewRows([]string{"full_name"})
	for _, n := range names {
		rows.AddRow(n)
	}
	mock.ExpectQuery("SELECT `full_name` FROM `git_hub_repos` WHERE user_id = \\? AND is_owner_or_collaborator = \\?").
		WithArgs(userID, true).
		WillReturnRows(rows)
}

// 要約の対象は、本人が作成者または共同編集者として同期済みのリポジトリだけ（SOCAIAGENT-698）。
// 対象外では、キャッシュ・プロフィールの読み出しも、README の取得も、AI の呼び出しもしない。
func TestSummarizeRepo_OnlyOwnSyncedRepos(t *testing.T) {
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	tests := []struct {
		name     string
		ownRepos []string // 同期済みで is_owner_or_collaborator=1 のもの
		fullName string
		wantErr  error
		// 対象のとき、キャッシュを引く full_name（同期済みの表記にそろえる）
		wantCacheKey string
	}{
		{name: "本人のリポジトリは要約できる", ownRepos: []string{"me/app", "me/cli"}, fullName: "me/cli", wantCacheKey: "me/cli"},
		{name: "共同編集者のリポジトリも要約できる", ownRepos: []string{"me/app", "friend/shared"}, fullName: "friend/shared", wantCacheKey: "friend/shared"},
		{name: "大文字小文字が違っても同期済みの表記で扱う", ownRepos: []string{"Me/App"}, fullName: "me/app", wantCacheKey: "Me/App"},
		{name: "他人の公開リポジトリは対象外", ownRepos: []string{"me/app"}, fullName: "stranger/popular", wantErr: ErrRepoNotSummarizable},
		// 所属組織の他人のリポジトリは同期されていても is_owner_or_collaborator=0 なので、一覧に出てこない
		{name: "所属組織の他人のリポジトリは対象外", ownRepos: []string{"me/app"}, fullName: "my-org/teammate-repo", wantErr: ErrRepoNotSummarizable},
		{name: "未同期（一覧が空）は対象外", ownRepos: nil, fullName: "me/app", wantErr: ErrRepoNotSummarizable},
		{name: "前方一致では通さない", ownRepos: []string{"me/app"}, fullName: "me/ap", wantErr: ErrRepoNotSummarizable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newSummaryTestServers(t, newSummaryJSON)
			db, mock := newSummaryTestDB(t)
			expectOwnRepos(mock, 7, tc.ownRepos...)
			if tc.wantErr == nil {
				// 今の版のキャッシュを返し、README・AI は呼ばずに終わらせる
				mock.ExpectQuery("SELECT \\* FROM `git_hub_repo_summaries` WHERE user_id = \\? AND full_name = \\?").
					WithArgs(7, tc.wantCacheKey, 1).
					WillReturnRows(sqlmock.NewRows(summaryColumns).
						AddRow(1, 7, tc.wantCacheKey, "要約", "", "", "", repoSummaryPromptVersion))
			}
			// 対象外のときはこれ以上のクエリを期待しない。呼ばれれば sqlmock がエラーにする。

			s := &GitHubService{
				githubRepo:   repositories.NewGitHubRepository(db),
				apiBaseURL:   ts.github.URL,
				openaiClient: openai.NewWithBaseURL(ts.openai.URL, "gpt-4o-mini"),
			}
			got, err := s.SummarizeRepo(context.Background(), 7, tc.fullName, false, "")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != nil {
					t.Fatalf("got=%v err=%v, want %v", got, err, tc.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("SummarizeRepo error: %v", err)
				}
				if got.FullName != tc.wantCacheKey {
					t.Errorf("FullName = %q, want %q", got.FullName, tc.wantCacheKey)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			if n := ts.githubCalls.Load(); n != 0 {
				t.Errorf("README の取得 = %d 回, want 0", n)
			}
			if n := ts.openaiCalls.Load(); n != 0 {
				t.Errorf("AI の呼び出し = %d 回, want 0", n)
			}
		})
	}
}

// 強制再生成（forceRefresh=true）でも対象外なら README・AI を呼ばない。
func TestSummarizeRepo_NotOwnedForceRefreshDoesNotCallGitHubOrAI(t *testing.T) {
	ts := newSummaryTestServers(t, newSummaryJSON)
	db, mock := newSummaryTestDB(t)
	expectOwnRepos(mock, 7, "me/app")

	s := &GitHubService{
		githubRepo:   repositories.NewGitHubRepository(db),
		apiBaseURL:   ts.github.URL,
		openaiClient: openai.NewWithBaseURL(ts.openai.URL, "gpt-4o-mini"),
	}
	if _, err := s.SummarizeRepo(context.Background(), 7, "stranger/popular", true, ""); !errors.Is(err, ErrRepoNotSummarizable) {
		t.Fatalf("err = %v, want ErrRepoNotSummarizable", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if ts.githubCalls.Load() != 0 || ts.openaiCalls.Load() != 0 {
		t.Fatalf("GitHub=%d AI=%d, want 0/0", ts.githubCalls.Load(), ts.openaiCalls.Load())
	}
}

// 一覧は、本人が作成者・共同編集者として同期済みのリポジトリの要約だけを返す。対象外の要約は消さずに隠す。
func TestListRepoSummaries_HidesNotOwnedRepos(t *testing.T) {
	tests := []struct {
		name     string
		ownRepos []string
		stored   []string // 保存済みの要約の full_name（今の版）
		want     []string
	}{
		{name: "対象のものだけ残す", ownRepos: []string{"me/app"}, stored: []string{"me/app", "stranger/popular", "my-org/teammate-repo"}, want: []string{"me/app"}},
		{name: "大文字小文字の違いは同じリポジトリとみなす", ownRepos: []string{"Me/App"}, stored: []string{"me/app"}, want: []string{"me/app"}},
		{name: "未同期ならすべて隠す", ownRepos: nil, stored: []string{"me/app"}, want: []string{}},
		{name: "要約が無ければ空", ownRepos: []string{"me/app"}, stored: nil, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSummaryTestDB(t)
			rows := sqlmock.NewRows(summaryColumns)
			for i, n := range tc.stored {
				rows.AddRow(i+1, 7, n, "要約", "", "", "", repoSummaryPromptVersion)
			}
			mock.ExpectQuery("SELECT \\* FROM `git_hub_repo_summaries` WHERE user_id = \\? AND prompt_version = \\? ORDER BY updated_at desc").
				WithArgs(7, repoSummaryPromptVersion).
				WillReturnRows(rows)
			expectOwnRepos(mock, 7, tc.ownRepos...)
			// DELETE / UPDATE は期待しない（隠すだけで消さない）

			s := &GitHubService{githubRepo: repositories.NewGitHubRepository(db)}
			got, err := s.ListRepoSummaries(context.Background(), 7)
			if err != nil {
				t.Fatalf("ListRepoSummaries error: %v", err)
			}
			names := []string{}
			for _, g := range got {
				names = append(names, g.FullName)
			}
			if len(names) != len(tc.want) {
				t.Fatalf("一覧 = %v, want %v", names, tc.want)
			}
			for i := range names {
				if names[i] != tc.want[i] {
					t.Fatalf("一覧 = %v, want %v", names, tc.want)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 同期では、本人が作成者・共同編集者のリポジトリにだけ印を付ける。組織メンバーとして見えるもの・
// 組織のリポジトリ・コントリビュートしただけのものには付けない（SOCAIAGENT-698）。
func TestFetchRepositories_MarksOwnerOrCollaborator(t *testing.T) {
	repo := func(fullName string) map[string]any {
		return map[string]any{"name": fullName, "full_name": fullName}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "repo, read:org")
		switch {
		case r.URL.Path == "/graphql":
			_, _ = w.Write([]byte(`{"data":{"viewer":{"repositoriesContributedTo":{"nodes":[{"name":"oss","nameWithOwner":"stranger/oss"},{"name":"app","nameWithOwner":"me/app"}],"pageInfo":{"hasNextPage":false}}}}}`))
		case r.URL.Path == "/user/orgs":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"login": "my-org"}})
		case r.URL.Path == "/orgs/my-org/repos":
			_ = json.NewEncoder(w).Encode([]any{repo("my-org/teammate-repo"), repo("my-org/org-only")})
		case r.URL.Path == "/user/repos" && r.URL.Query().Get("affiliation") == "owner,collaborator":
			_ = json.NewEncoder(w).Encode([]any{repo("me/app"), repo("friend/shared")})
		case r.URL.Path == "/user/repos" && r.URL.Query().Get("affiliation") == "organization_member":
			_ = json.NewEncoder(w).Encode([]any{repo("my-org/teammate-repo"), repo("me/app")})
		default:
			t.Errorf("想定外のリクエスト: %s", r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	s := &GitHubService{apiBaseURL: server.URL, graphQLURL: server.URL + "/graphql"}
	repos, err := s.fetchRepositories(context.Background(), server.Client(), "token")
	if err != nil {
		t.Fatalf("fetchRepositories error: %v", err)
	}
	want := map[string]bool{
		"me/app":               true,
		"friend/shared":        true,
		"my-org/teammate-repo": false,
		"my-org/org-only":      false,
		"stranger/oss":         false,
	}
	got := map[string]bool{}
	for _, r := range repos {
		if _, dup := got[r.FullName]; dup {
			t.Errorf("%s が重複している", r.FullName)
		}
		got[r.FullName] = r.IsOwnerOrCollaborator
	}
	if len(got) != len(want) {
		t.Fatalf("取得したリポジトリ = %v, want %v", got, want)
	}
	for name, w := range want {
		if g, ok := got[name]; !ok || g != w {
			t.Errorf("%s: IsOwnerOrCollaborator = %v (ok=%v), want %v", name, g, ok, w)
		}
	}
}
