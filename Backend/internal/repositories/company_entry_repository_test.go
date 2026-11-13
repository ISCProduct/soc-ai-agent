package repositories

import (
	"errors"
	"testing"
	"time"

	"Backend/domain/repository"
	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 掲載申請(ゲスト投稿)の DB 操作のテスト。SQL の WHERE と、書き込む値・引数まで検証する。
// 企業 ID の紐づけを外すと、企業と求人・卒業生・申請がつながらない行ができてしまう。

func newCompanyEntryRepoTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func newCompanyEntryDraftForTest() repository.CompanyEntryDraft {
	return repository.CompanyEntryDraft{
		Company:       &models.Company{Name: "株式会社テスト"},
		Submission:    &models.CompanyEntrySubmission{ContactEmail: "hr@example.com"},
		JobPositions:  []*models.CompanyJobPosition{{Title: "エンジニア"}, {Title: "営業"}},
		WeightProfile: &models.CompanyWeightProfile{TechnicalOrientation: 4},
		Graduates:     []*models.GraduateEmployment{{GraduateName: "卒業生A"}},
	}
}

func TestCompanyEntryRepository_CreateDraft_LinksEveryRowToCompany(t *testing.T) {
	db, mock := newCompanyEntryRepoTestDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("INSERT INTO `company_job_positions`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `company_job_positions`").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("company_weight_profiles").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `graduate_employments`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `company_entry_submissions`").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()

	draft := newCompanyEntryDraftForTest()
	if err := NewCompanyEntryRepository(db).CreateDraft(draft); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
	if draft.Company.ID != 77 {
		t.Fatalf("company.ID = %d, want 77", draft.Company.ID)
	}
	if draft.Submission.CompanyID != 77 || draft.Submission.ID != 9 {
		t.Errorf("申請: CompanyID=%d ID=%d, want 77, 9", draft.Submission.CompanyID, draft.Submission.ID)
	}
	for i, p := range draft.JobPositions {
		if p.CompanyID != 77 {
			t.Errorf("求人[%d].CompanyID = %d, want 77", i, p.CompanyID)
		}
	}
	if draft.WeightProfile.CompanyID != 77 {
		t.Errorf("重みプロファイル.CompanyID = %d, want 77", draft.WeightProfile.CompanyID)
	}
	if draft.Graduates[0].CompanyID != 77 {
		t.Errorf("卒業生.CompanyID = %d, want 77", draft.Graduates[0].CompanyID)
	}
}

func TestCompanyEntryRepository_CreateDraft_WithoutOptionalRows(t *testing.T) {
	db, mock := newCompanyEntryRepoTestDB(t)
	draft := repository.CompanyEntryDraft{
		Company:    &models.Company{Name: "株式会社テスト"},
		Submission: &models.CompanyEntrySubmission{ContactEmail: "hr@example.com"},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectExec("INSERT INTO `company_entry_submissions`").WillReturnResult(sqlmock.NewResult(6, 1))
	mock.ExpectCommit() // 求人・重み・卒業生の INSERT は来ない(来たら sqlmock が失敗させる)

	if err := NewCompanyEntryRepository(db).CreateDraft(draft); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

// 途中で失敗したら全体を巻き戻し、失敗した段の名前を付けて返す。後続の行は書かない。
func TestCompanyEntryRepository_CreateDraft_RollsBackOnFailure(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock, boom error)
		wantMsg string
	}{
		{"企業", func(m sqlmock.Sqlmock, boom error) {
			m.ExpectExec("INSERT INTO `companies`").WillReturnError(boom)
		}, "create company"},
		{"求人", func(m sqlmock.Sqlmock, boom error) {
			m.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `company_job_positions`").WillReturnError(boom)
		}, "create job position"},
		{"卒業生", func(m sqlmock.Sqlmock, boom error) {
			m.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `company_job_positions`").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `company_job_positions`").WillReturnResult(sqlmock.NewResult(2, 1))
			m.ExpectExec("company_weight_profiles").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `graduate_employments`").WillReturnError(boom)
		}, "create graduate employment"},
		{"申請", func(m sqlmock.Sqlmock, boom error) {
			m.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `company_job_positions`").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `company_job_positions`").WillReturnResult(sqlmock.NewResult(2, 1))
			m.ExpectExec("company_weight_profiles").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `graduate_employments`").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("INSERT INTO `company_entry_submissions`").WillReturnError(boom)
		}, "create submission"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newCompanyEntryRepoTestDB(t)
			boom := errors.New("db down")
			mock.ExpectBegin()
			tc.setup(mock, boom)
			mock.ExpectRollback()

			err := NewCompanyEntryRepository(db).CreateDraft(newCompanyEntryDraftForTest())
			if !errors.Is(err, boom) || err == nil || !containsStr(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %v, want %q を含み db down を包む", err, tc.wantMsg)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("sql: %v", err)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCompanyEntryRepository_FindSubmissionAndCompanyName(t *testing.T) {
	db, mock := newCompanyEntryRepoTestDB(t)
	repo := NewCompanyEntryRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `company_entry_submissions` WHERE `company_entry_submissions`.`id` = \\? ORDER BY").
		WithArgs(uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "contact_email"}).AddRow(7, 3, "hr@example.com"))
	sub, err := repo.FindSubmission(7)
	if err != nil || sub.ID != 7 || sub.CompanyID != 3 {
		t.Fatalf("sub=%+v err=%v", sub, err)
	}

	// name だけを読む(Select を外すと、不要な列まで読む)
	mock.ExpectQuery("SELECT `id`,`name` FROM `companies` WHERE `companies`.`id` = \\? AND `companies`.`deleted_at` IS NULL ORDER BY").
		WithArgs(uint(3), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(3, "株式会社テスト"))
	c, err := repo.FindCompanyName(3)
	if err != nil || c.Name != "株式会社テスト" {
		t.Fatalf("company=%+v err=%v", c, err)
	}

	mock.ExpectQuery("SELECT \\* FROM `company_entry_submissions`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := repo.FindSubmission(99); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("見つからないとき err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

// 送信状態の更新: 何を書くか(status・日時・エラー・試行回数)と、どの行か(id)を固定する。
func TestCompanyEntryRepository_MarkInviteEmail(t *testing.T) {
	sentAt := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)

	t.Run("送信済み", func(t *testing.T) {
		db, mock := newCompanyEntryRepoTestDB(t)
		mock.ExpectBegin()
		// SET の並びは gorm が列名の昇順に整える: email_attempts, email_last_error, email_sent_at, email_status, updated_at
		mock.ExpectExec("UPDATE `company_entry_submissions` SET `email_attempts`=email_attempts \\+ 1,`email_last_error`=\\?,`email_sent_at`=\\?,`email_status`=\\?,`updated_at`=\\? WHERE id = \\?").
			WithArgs("", sentAt, "sent", sqlmock.AnyArg(), uint(7)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		if err := NewCompanyEntryRepository(db).MarkInviteEmailSent(7, sentAt); err != nil {
			t.Fatalf("MarkInviteEmailSent: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sql: %v", err)
		}
	})

	t.Run("送信失敗", func(t *testing.T) {
		db, mock := newCompanyEntryRepoTestDB(t)
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `company_entry_submissions` SET `email_attempts`=email_attempts \\+ 1,`email_last_error`=\\?,`email_status`=\\?,`updated_at`=\\? WHERE id = \\?").
			WithArgs("smtp down", "failed", sqlmock.AnyArg(), uint(8)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		if err := NewCompanyEntryRepository(db).MarkInviteEmailFailed(8, "smtp down"); err != nil {
			t.Fatalf("MarkInviteEmailFailed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sql: %v", err)
		}
	})
}

func TestCompanyEntryRepository_ClaimOwnershipAndMarkClaimed(t *testing.T) {
	db, mock := newCompanyEntryRepoTestDB(t)
	repo := NewCompanyEntryRepository(db)

	// 新規: 所有者が居ないので、userID を owner として作る
	mock.ExpectQuery("SELECT \\* FROM `company_ownerships` WHERE company_id = \\?").
		WithArgs(uint(10), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "user_id", "role"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_ownerships`").
		WithArgs(uint(10), uint(42), "owner", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	got, err := repo.ClaimOwnership(10, 42)
	if err != nil || got.UserID != 42 || got.CompanyID != 10 || got.Role != "owner" {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	// 既に別のユーザーが所有: エラーにならず、既存の行(別の UserID)が返る
	mock.ExpectQuery("SELECT \\* FROM `company_ownerships` WHERE company_id = \\?").
		WithArgs(uint(11), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "user_id", "role"}).AddRow(1, 11, 99, "owner"))
	got, err = repo.ClaimOwnership(11, 42)
	if err != nil || got.UserID != 99 {
		t.Fatalf("既存の所有者(99)が返るはず: got=%+v err=%v", got, err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions` SET `status`=\\?,`updated_at`=\\? WHERE id = \\?").
		WithArgs("claimed", sqlmock.AnyArg(), uint(21)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.MarkSubmissionClaimed(21); err != nil {
		t.Fatalf("MarkSubmissionClaimed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}
