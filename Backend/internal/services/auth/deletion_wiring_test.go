package auth

import (
	"os"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"Backend/internal/repositories"
)

// 退会サービス(UserDeletionService)は、1つのインスタンスを全員で使う。
//
// 以前は main.go と AuthService の内部で別々に作っていて、スカウト検索の同期(SetScoutIndexSyncer)を
// 設定したのは AuthService の内部のほうだけだった。そのため、管理者による退会・日次のリコンサイル
// (ReconcileScoutIndex)を担う main.go 側のインスタンスは、同期先が未設定のまま動いていた。
// インスタンスを分けると、同じ取りこぼしが再び起きる。

func TestMainWiresSingleUserDeletionService(t *testing.T) {
	raw, err := os.ReadFile("../../../cmd/server/main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)

	if n := strings.Count(src, "auth.NewUserDeletionService("); n != 1 {
		t.Errorf("auth.NewUserDeletionService は main.go で1回だけ呼ぶこと(いま %d 回)。別のインスタンスは依存の設定が届かない", n)
	}
	for _, want := range []string{
		"authService.SetDeletionService(userDeletionService)",
		"userDeletionService.SetScoutIndexSyncer(",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("main.go に %q が無い", want)
		}
	}
}

// 本人退会(AuthService.DeleteAccount)が、注入した退会サービスを使う。
// 注入したインスタンスに設定した同期が、本人退会でも呼ばれることで確かめる。
func TestDeleteAccount_UsesInjectedDeletionService(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	deletion := NewUserDeletionService(repositories.NewUserWithdrawalRepository(db), nil, nil)
	syncer := &scoutSyncerRecorder{}
	deletion.SetScoutIndexSyncer(syncer)
	svc := &AuthService{}
	svc.SetDeletionService(deletion)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(1, "user@example.com", nil))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos` WHERE user_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id", "drive_file_id"}))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE user_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_path"}))
	mock.ExpectExec("INSERT INTO `withdrawn_users`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM `pending_registrations`").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `users` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `user_refresh_tokens` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteAccount(1); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if len(syncer.userIDs) != 1 || syncer.userIDs[0] != 1 {
		t.Fatalf("本人退会で、注入した退会サービスのスカウト検索の同期が呼ばれていない: %v", syncer.userIDs)
	}
}
