package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gorm.io/gorm/schema"
)

// 退会(物理削除)で、ユーザーに紐づく行が取りこぼされないことを検査する。
//
// 既存のテストは、テーブルごとに SQL を確認する作りで、「user_id を持つテーブルが
// 全て削除対象か」は誰も見ていなかった。テーブルを足したときに退会の処理を直し忘れると、
// ユーザーを物理削除したあとも個人情報が残る(個人情報保護法第28条)。
//
// ユーザーを指す列(user_id など)を持つテーブルは、次のどれかでなければならない。
//  1. hardDeleteUserData が削除する
//  2. users への外部キーが ON DELETE CASCADE(親テーブルへの CASCADE を介した連鎖でもよい)
//  3. userRowsKeptAfterPurge に、残す(または未対応の)理由つきで載っている
//
// 実 DB は使わず、ソースとマイグレーションを静的に解析する。

// userRowsKeptAfterPurge は、退会時の扱いを、テーブルごとに別に決めているテーブル。
// 理由を書くこと。ここに足す前に、hardDeleteUserData に追加できないかを先に検討すること。
var userRowsKeptAfterPurge = map[string]string{
	// 退会の記録そのもの(メールのハッシュ・マスク済みメール・実行者・S3 キー)。
	// purged_at を付けてパージ済みの印として残す設計。
	"withdrawn_users": "退会の記録。パージ済みの印として残す",

	// 以下は、退会時の扱いを方針決定中。
	"admin_school_memberships":  "未対応(方針決定待ち)",
	"api_call_logs":             "未対応(方針決定待ち)",
	"company_ownerships":        "未対応(方針決定待ち)",
	"diagnosis_quality_reports": "未対応(方針決定待ち)",
}

// ユーザーを指す列。user_id のほかに、教員案内が使う 2 列を数える。
var userRefColumns = []string{"user_id", "student_user_id", "teacher_user_id"}

type userTableInfo struct {
	cols    map[string]bool // ユーザーを指す列
	cascade map[string]bool // その列の users への外部キーが ON DELETE CASCADE
}

// cascadeParents は、テーブル → ON DELETE CASCADE で消える元の親テーブル(users 以外も含む)。
type cascadeParents map[string]map[string]bool

func TestUserDeletion_EveryUserLinkedTableIsHandled(t *testing.T) {
	purged := purgedTableNames(t)
	tables, parents := userLinkedTables(t)

	// handled(T): 物理削除される / users への CASCADE / 親が handled で、親への FK が CASCADE
	var handled func(name string, seen map[string]bool) bool
	handled = func(name string, seen map[string]bool) bool {
		if purged[name] {
			return true
		}
		if seen[name] {
			return false
		}
		seen[name] = true
		if info := tables[name]; info != nil && len(info.cols) > 0 {
			all := true
			for col := range info.cols {
				if !info.cascade[col] {
					all = false
				}
			}
			if all {
				return true
			}
		}
		for parent := range parents[name] {
			if handled(parent, seen) {
				return true
			}
		}
		return false
	}

	var unhandled []string
	for name := range tables {
		if handled(name, map[string]bool{}) {
			continue
		}
		if _, ok := userRowsKeptAfterPurge[name]; ok {
			continue
		}
		unhandled = append(unhandled, name)
	}
	sort.Strings(unhandled)
	if len(unhandled) > 0 {
		t.Errorf("退会の物理削除で取りこぼされる可能性があるテーブル(ユーザーを指す列があり、削除対象でも CASCADE でもない):\n  %s\n"+
			"hardDeleteUserData に追加するか、残す理由を userRowsKeptAfterPurge に書くこと。",
			strings.Join(unhandled, "\n  "))
	}

	// 載せたのに消えた・対応済みになったテーブルは、リストから外させる(リストを腐らせない)。
	for name, reason := range userRowsKeptAfterPurge {
		if tables[name] == nil {
			t.Errorf("userRowsKeptAfterPurge の %q は、ユーザーを指す列を持つテーブルとして見つからない(%s)。リストから消すこと", name, reason)
			continue
		}
		if handled(name, map[string]bool{}) {
			t.Errorf("userRowsKeptAfterPurge の %q は、削除または CASCADE で消えるようになっている。リストから消すこと", name)
		}
	}

	if len(purged) < 30 {
		t.Fatalf("hardDeleteUserData から削除対象を %d 件しか読めなかった。解析が壊れている", len(purged))
	}
}

// purgedTableNames は hardDeleteUserData が触るモデルのテーブル名を返す。
func purgedTableNames(t *testing.T) map[string]bool {
	t.Helper()
	overrides := tableNameOverrides(t)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join("..", "..", "repositories", "user_withdrawal_repository.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	naming := schema.NamingStrategy{}
	out := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "hardDeleteUserData" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "models" {
				return true
			}
			name := sel.Sel.Name
			if table, ok := overrides[name]; ok {
				out[table] = true
			} else {
				out[naming.TableName(name)] = true
			}
			return true
		})
	}
	return out
}

// tableNameOverrides は models の TableName() が返す名前を集める。
func tableNameOverrides(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "models", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("models が読めない: %v", err)
	}
	out := map[string]string{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "TableName" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			recv := fn.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			id, ok := recv.(*ast.Ident)
			if !ok || fn.Body == nil || len(fn.Body.List) != 1 {
				continue
			}
			ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			if lit, ok := ret.Results[0].(*ast.BasicLit); ok {
				out[id.Name] = strings.Trim(lit.Value, "\"`")
			}
		}
	}
	return out
}

var (
	reCreateTable = regexp.MustCompile("(?is)CREATE TABLE(?: IF NOT EXISTS)?\\s+`?(\\w+)`?\\s*\\((.*?)\\)\\s*ENGINE")
	reAlterTable  = regexp.MustCompile("(?is)ALTER TABLE\\s+`?(\\w+)`?(.*?);")
	reColumnDef   = regexp.MustCompile("(?i)(?:^|ADD COLUMN\\s+|,\\s*)`?(\\w+)`?\\s+(?:BIGINT|INT|VARCHAR|CHAR)\\b")
	reFKAny       = regexp.MustCompile("(?is)FOREIGN KEY\\s*\\([^)]*\\)\\s*REFERENCES\\s+`?(\\w+)`?\\s*\\([^)]*\\)([^,;)]*)")
	reFKUsers     = regexp.MustCompile("(?is)FOREIGN KEY\\s*\\(\\s*`?(\\w+)`?\\s*\\)\\s*REFERENCES\\s+`?users`?\\s*\\(\\s*`?id`?\\s*\\)([^,;)]*)")
)

// userLinkedTables は、マイグレーションから「ユーザーを指す列を持つテーブル」を集める。
func userLinkedTables(t *testing.T) (map[string]*userTableInfo, cascadeParents) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations が読めない: %v", err)
	}
	sort.Strings(files)

	isRef := map[string]bool{}
	for _, c := range userRefColumns {
		isRef[c] = true
	}
	tables := map[string]*userTableInfo{}
	parents := cascadeParents{}
	get := func(name string) *userTableInfo {
		if tables[name] == nil {
			tables[name] = &userTableInfo{cols: map[string]bool{}, cascade: map[string]bool{}}
		}
		return tables[name]
	}
	scan := func(table, body string) {
		for _, m := range reFKAny.FindAllStringSubmatch(body, -1) {
			if strings.Contains(strings.ToUpper(m[2]), "ON DELETE CASCADE") {
				if parents[table] == nil {
					parents[table] = map[string]bool{}
				}
				parents[table][m[1]] = true
			}
		}
		for _, line := range strings.Split(body, "\n") {
			if m := reColumnDef.FindStringSubmatch(strings.TrimSpace(line)); m != nil && isRef[m[1]] {
				get(table).cols[m[1]] = true
			}
		}
		for _, m := range reFKUsers.FindAllStringSubmatch(body, -1) {
			if !isRef[m[1]] {
				continue
			}
			info := get(table)
			info.cols[m[1]] = true
			if strings.Contains(strings.ToUpper(m[2]), "ON DELETE CASCADE") {
				info.cascade[m[1]] = true
			}
		}
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sql := string(raw)
		for _, m := range reCreateTable.FindAllStringSubmatch(sql, -1) {
			scan(m[1], m[2])
		}
		for _, m := range reAlterTable.FindAllStringSubmatch(sql, -1) {
			scan(m[1], m[2])
		}
	}
	return tables, parents
}
