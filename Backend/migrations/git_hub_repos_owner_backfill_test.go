package migrations

import (
	"strings"
	"testing"
)

// SOCAIAGENT-698 のオーナー決定（2026-11-22）: 列を足したあと、既存の git_hub_repos の行を本人のもの（1）として扱う。
// これが無いと既定値 0 のままになり、既存の利用者は同期し直すまで AI 要約を作れず、保存済みの要約も一覧から隠れる
// （ListOwnRepoFullNames が is_owner_or_collaborator = 1 の行だけを返すため）。
// 実際の MySQL での適用は PR で確かめる。ここでは SQL に移行が入っていて、列の追加より後にあることだけを見る。
func TestGitHubReposOwnerBackfill_MarksExistingRows(t *testing.T) {
	data, err := migrationFiles.ReadFile("20261122211106_git_hub_repos_add_is_owner_or_collaborator.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	// コメントを除いた SQL 本体で確かめる（コメントの中の文字列で通らないように）
	var body []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		body = append(body, line)
	}
	sql := strings.Join(body, "\n")

	alter := strings.Index(sql, "ADD COLUMN is_owner_or_collaborator")
	update := strings.Index(sql, "UPDATE git_hub_repos SET is_owner_or_collaborator = 1;")
	if alter < 0 {
		t.Fatal("up に列の追加が無い")
	}
	if update < 0 {
		t.Fatal("up に既存の行を 1 にする UPDATE が無い")
	}
	if update < alter {
		t.Error("UPDATE が列の追加より前にある")
	}
}
