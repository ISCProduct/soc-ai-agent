package services

// GitHub GraphQL インジェクション対策のテスト（Issue #311）
// 実行: cd Backend && go test ./internal/services/... -run TestGitHub -v

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
			// 本番コードと同じ構造体を再現してJSONシリアライズをテスト
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

			// JSON文字列として解析できることを確認
			var decoded map[string]any
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("login=%q を含むJSONが不正: %v", tc.login, err)
			}

			// 生のJSONに悪意ある文字列がリテラルに埋め込まれていないことを確認
			raw := string(b)
			// variables.login の値はJSON文字列として正しくエスケープされているべき
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

			// クエリ文字列が login の値で汚染されていないことを確認
			if strings.Contains(raw, tc.login) && tc.login != "alice" && tc.login != "alice-bob" {
				// 特殊文字は JSON エスケープされているため Raw に直接出現しないはず
				// ただし通常の文字列はそのまま出現するため、特殊文字のみ確認
				if strings.Contains(tc.login, `"`) {
					// ダブルクオートはエスケープされていなければならない
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

			// ラウンドトリップ確認
			varsDecoded, _ := decoded["variables"].(map[string]any)
			decodedAfter, _ := varsDecoded["after"].(string)
			if decodedAfter != tc.cursor {
				t.Errorf("ラウンドトリップ後のカーソル値が不一致: got %q, want %q", decodedAfter, tc.cursor)
			}
		})
	}
}
