package repositories

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"Backend/internal/models"
)

// 仮登録の保存と取得で、全フィールドが DB との間で落ちずに運ばれることを確かめる。
// 以前は entity との変換（mapper）がこの保証を担っていた。

func TestPendingRegistrationCreate_全フィールドをINSERTに渡す(t *testing.T) {
	db, mock := newExpiredDeleteTestDB(t)
	exp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	company, submission := uint(7), uint(9)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `pending_registrations`").
		WithArgs("tok", "a@example.com", int64(company), int64(submission), exp, created).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := NewPendingRegistrationRepository(db).Create(&models.PendingRegistration{
		Token: "tok", Email: "a@example.com", CompanyID: &company, SubmissionID: &submission,
		ExpiresAt: exp, CreatedAt: created,
	})
	if err != nil {
		t.Fatalf("Create失敗: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPendingRegistrationFindByToken_全フィールドを返す_未発見はnil(t *testing.T) {
	db, mock := newExpiredDeleteTestDB(t)
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	created := exp.Add(-24 * time.Hour)
	cols := []string{"token", "email", "company_id", "submission_id", "expires_at", "created_at"}

	mock.ExpectQuery("SELECT \\* FROM `pending_registrations` WHERE token = \\? AND expires_at > \\?").
		WithArgs("tok", sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows(cols).AddRow("tok", "a@example.com", 7, 9, exp, created))
	mock.ExpectQuery("SELECT \\* FROM `pending_registrations`").
		WithArgs("none", sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows(cols))

	repo := NewPendingRegistrationRepository(db)
	got, err := repo.FindByToken("tok")
	if err != nil || got == nil {
		t.Fatalf("取得失敗: %v %v", got, err)
	}
	if got.Token != "tok" || got.Email != "a@example.com" || got.CompanyID == nil || *got.CompanyID != 7 ||
		got.SubmissionID == nil || *got.SubmissionID != 9 || !got.ExpiresAt.Equal(exp) || !got.CreatedAt.Equal(created) {
		t.Fatalf("フィールドが落ちている: %+v", got)
	}
	if miss, err := repo.FindByToken("none"); miss != nil || err != nil {
		t.Fatalf("未発見は nil,nil のはず: %v %v", miss, err)
	}
}
