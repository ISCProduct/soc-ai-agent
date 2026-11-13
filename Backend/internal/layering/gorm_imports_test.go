package layering

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Controller と Service は、GORM を直接 import しない。
//
// DB は Repository の内側に閉じる。not found の判定は repository.ErrNotFound を使う
// (domain/repository/not_found.go)。Controller や Service が gorm を知っていると、
// テストに DB が要り、ORM を替えるときの影響が全体に広がる。
//
// allowedServiceFiles は、今も *gorm.DB を直接持っている Service。新しく足してはいけない。
// Repository へ移したら、ここから消すこと(消し忘れはログに出る。失敗にはならない)。
var allowedServiceFiles = []string{
	"internal/services/application/application_service.go",
	"internal/services/auth/auth_deletion.go",
	"internal/services/auth/auth_service.go",
	"internal/services/auth/user_deletion_service.go",
	"internal/services/company/company_audit_apply.go",
	"internal/services/company/company_entry_service.go",
	"internal/services/companyauth/company_auth_service.go",
	"internal/services/hr/student_analysis_service.go",
	"internal/services/release/release_note_service.go",
	"internal/services/shared/company_owner.go",
	"internal/services/training/export_service.go",
}

func TestControllersAndServicesDoNotImportGorm(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	var found []string
	for _, dir := range []string{"internal/controllers", "internal/services", "internal/middleware", "internal/routes"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.Contains(rel, "/mocks/") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			// 検索文字列を分けて書く。そのまま書くとこのファイル自身に一致してしまう。
			if strings.Contains(string(src), `"gorm.io/`+`gorm"`) {
				found = append(found, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, f := range found {
		if !slices.Contains(allowedServiceFiles, f) {
			t.Errorf("%s が gorm を import している。DB は Repository に閉じ、not found は repository.ErrNotFound で判定すること", f)
		}
	}
	// 許可リストの縮小は、失敗にはしない(Repository へ移す PR が別々に入っても、
	// 後から入るほうの CI が赤くならないようにするため)。消し忘れはログに出す。
	for _, f := range allowedServiceFiles {
		if !slices.Contains(found, f) {
			t.Logf("%s は gorm を import しなくなった。allowedServiceFiles から消してよい", f)
		}
	}
}
