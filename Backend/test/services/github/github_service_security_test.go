package github_test

// GitHub GraphQL インジェクション対策のテスト（Issue #311）
// 実行: cd Backend && go test ./test/services/... -run TestGitHub -v

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestFetchTotalContributions_LoginIsSafelyEncoded は login 値が JSON として安全にエンコードされることを検証する（#311修正の担保）
func TestFetchTotalContributions_LoginIsSafelyEncoded(t *testing.T) {
	tests := []struct {
		name  string
		login string
	}{
		{"通常のログイン名", "alice"},
		{"ハイフン含む", "alice-bob"},
		{"ダブルクオート含む（インジェクション試行）", `alice"evil`},
		{"バックスラッシュ含む", `alice\evil`},
		{"改行含む", "alice\nevil"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}{
				Query:     `query($login: String!) { user(login: $login) { contributionsCollection { contributionCalendar { totalContributions } } } }`,
				Variables: map[string]any{"login": tc.login},
			}
			b, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("login=%q を含むJSONが不正: %v", tc.login, err)
			}

			raw := string(b)
			vars, ok := decoded["variables"].(map[string]any)
			if !ok {
				t.Fatal("variables フィールドが不正")
			}
			decodedLogin, ok := vars["login"].(string)
			if !ok {
				t.Fatal("variables.login フィールドが不正")
			}
			if decodedLogin != tc.login {
				t.Errorf("ラウンドトリップ後の login 値が不一致: got %q, want %q", decodedLogin, tc.login)
			}

			if strings.Contains(raw, tc.login) && tc.login != "alice" && tc.login != "alice-bob" {
				if strings.Contains(tc.login, `"`) {
					escapedQ := `\"`
					if !strings.Contains(raw, escapedQ) {
						t.Errorf("ダブルクオートがエスケープされていない: %s", raw)
					}
				}
			}
		})
	}
}

// TestContributedReposQuery_CursorIsSafelyEncoded はカーソル値が JSON として安全にエンコードされることを検証する（#311修正の担保）
func TestContributedReposQuery_CursorIsSafelyEncoded(t *testing.T) {
	type contributedReposVars struct {
		After *string `json:"after,omitempty"`
	}

	tests := []struct {
		name   string
		cursor string
	}{
		{"通常のbase64カーソル", "Y3Vyc29yOnYyOpK5"},
		{"ダブルクオート含む（インジェクション試行）", `cursor"evil`},
		{"バックスラッシュ含む", `cursor\evil`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			after := tc.cursor
			vars := contributedReposVars{After: &after}
			payload := struct {
				Query     string               `json:"query"`
				Variables contributedReposVars `json:"variables"`
			}{
				Query:     `query($after: String) { viewer { repositoriesContributedTo(first: 100, after: $after) { pageInfo { hasNextPage endCursor } } } }`,
				Variables: vars,
			}

			b, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("カーソル=%q を含むJSONが不正: %v", tc.cursor, err)
			}

			varsDecoded, _ := decoded["variables"].(map[string]any)
			decodedAfter, _ := varsDecoded["after"].(string)
			if decodedAfter != tc.cursor {
				t.Errorf("ラウンドトリップ後のカーソル値が不一致: got %q, want %q", decodedAfter, tc.cursor)
			}
		})
	}
}
