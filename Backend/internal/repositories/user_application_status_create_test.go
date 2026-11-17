package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"Backend/internal/models"
)

// TestCreate_DoesNotWriteAssociations は、Company を持つ応募を保存しても
// 応募の行だけが書かれ、企業の行へは触れないことを検証する。
// 関連を保存対象から外し忘れると、企業への INSERT が余計に発行されて落ちる。
func TestCreate_DoesNotWriteAssociations(t *testing.T) {
	repo, mock, _ := newAppStatusRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_application_statuses`").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectCommit()

	app := &models.UserApplicationStatus{
		UserID:    1,
		CompanyID: 7,
		MatchID:   3,
		Status:    "applied",
		Company:   models.Company{ID: 7, Name: "テスト企業"},
	}
	if err := repo.Create(app); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.ID != 42 {
		t.Fatalf("ID が採番値で埋まっていない: %d", app.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestFindByUserID_NoRowsIsEmptyNotNil は、該当なしのとき nil ではなく空スライスを返すことを検証する。
func TestFindByUserID_NoRowsIsEmptyNotNil(t *testing.T) {
	repo, mock, _ := newAppStatusRepo(t)
	cols := []string{"id", "user_id", "company_id", "status"}

	mock.ExpectQuery("SELECT .* FROM `user_application_statuses`").
		WillReturnRows(sqlmock.NewRows(cols))
	got, err := repo.FindByUserID(1)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("FindByUserID: 該当なしで nil が返った")
	}

	mock.ExpectQuery("SELECT .* FROM `user_application_statuses`").
		WillReturnRows(sqlmock.NewRows(cols))
	got, err = repo.FindAll(0, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("FindAll: 該当なしで nil が返った")
	}
}
