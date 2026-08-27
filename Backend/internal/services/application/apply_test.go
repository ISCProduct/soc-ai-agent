package application

// Apply の重複判定・match所有権検証のテスト（#1017）
//
// 実行: cd Backend && go test ./internal/services/application/... -run Apply -v

import (
	"errors"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/services/shared"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// stubApplyMatchRepo は Apply のテスト用最小モック（ToggleFavorite用スタブと同じパターン）。
type stubApplyMatchRepo struct {
	findByIDResult *entity.UserCompanyMatch
	findByIDErr    error
}

func (s *stubApplyMatchRepo) CreateOrUpdate(*entity.UserCompanyMatch) error { return nil }
func (s *stubApplyMatchRepo) CreateOrUpdateBatch([]*entity.UserCompanyMatch) (int, error) {
	return 0, nil
}
func (s *stubApplyMatchRepo) FindTopMatchesByUserAndSession(uint, string, int) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
func (s *stubApplyMatchRepo) FindByID(uint) (*entity.UserCompanyMatch, error) {
	return s.findByIDResult, s.findByIDErr
}
func (s *stubApplyMatchRepo) MarkAsViewed(uint) error   { return nil }
func (s *stubApplyMatchRepo) ToggleFavorite(uint) error { return nil }
func (s *stubApplyMatchRepo) MarkAsApplied(uint) error  { return nil }
func (s *stubApplyMatchRepo) FindFavoritesByUser(uint, string) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
func (s *stubApplyMatchRepo) GetMatchStatistics(uint, string) (map[string]any, error) {
	return nil, nil
}

// newSQLMockDB はトランザクションを含むApplyのテスト用にmysql+sqlmockのgorm.DBを生成する。
func newSQLMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func TestApply_NotFoundOnMissingMatch(t *testing.T) {
	repo := &stubApplyMatchRepo{findByIDErr: gorm.ErrRecordNotFound}
	s := &ApplicationService{matchRepo: repo}

	_, err := s.Apply(1, 10, 999)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("got %v want shared.ErrNotFound", err)
	}
}

func TestApply_ForbiddenOnOtherUsersMatch(t *testing.T) {
	repo := &stubApplyMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 5, UserID: 2, CompanyID: 10}}
	s := &ApplicationService{matchRepo: repo}

	// userID=1 だが match の所有者は userID=2
	_, err := s.Apply(1, 10, 5)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want shared.ErrForbidden", err)
	}
}

func TestApply_BadRequestOnCompanyMismatch(t *testing.T) {
	repo := &stubApplyMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 5, UserID: 1, CompanyID: 10}}
	s := &ApplicationService{matchRepo: repo}

	// body の company_id(999) が match.CompanyID(10) と不一致
	_, err := s.Apply(1, 999, 5)
	if err == nil {
		t.Fatal("company_id 不一致はエラーになるべき")
	}
	if errors.Is(err, shared.ErrNotFound) || errors.Is(err, shared.ErrForbidden) || errors.Is(err, shared.ErrDuplicateActiveApplication) {
		t.Fatalf("company_id 不一致は他のエラー種別と区別されるべき: %v", err)
	}
}

func TestApply_DuplicateActiveApplication(t *testing.T) {
	db, mock := newSQLMockDB(t)
	repo := &stubApplyMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 5, UserID: 1, CompanyID: 10}}
	s := &ApplicationService{matchRepo: repo, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `user_application_statuses` WHERE user_id = \\? AND company_id = \\? AND status NOT IN \\(\\?,\\?,\\?\\)").
		WithArgs(uint(1), uint(10), "accepted", "withdrawn", "rejected", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "company_id", "status"}).
			AddRow(1, 1, 10, "document_screening"))
	mock.ExpectRollback()

	_, err := s.Apply(1, 10, 5)
	if !errors.Is(err, shared.ErrDuplicateActiveApplication) {
		t.Fatalf("got %v want shared.ErrDuplicateActiveApplication", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestApply_SucceedsAndMarksApplied(t *testing.T) {
	db, mock := newSQLMockDB(t)
	repo := &stubApplyMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 5, UserID: 1, CompanyID: 10}}
	s := &ApplicationService{matchRepo: repo, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `user_application_statuses` WHERE user_id = \\? AND company_id = \\? AND status NOT IN \\(\\?,\\?,\\?\\)").
		WithArgs(uint(1), uint(10), "accepted", "withdrawn", "rejected", sqlmock.AnyArg()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec("INSERT INTO `user_application_statuses`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec("UPDATE `user_company_matches` SET `is_applied`=\\?").
		WithArgs(true, sqlmock.AnyArg(), uint(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	app, err := s.Apply(1, 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != "applied" {
		t.Fatalf("got status %q want applied", app.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

// MarkAsApplied相当のUPDATEが失敗した場合、応募のCreateごとロールバックされ、
// 中途半端な状態（応募だけ作られてis_appliedが更新されない）が残らないことを確認する。
func TestApply_RollsBackWhenMarkAppliedFails(t *testing.T) {
	db, mock := newSQLMockDB(t)
	repo := &stubApplyMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 5, UserID: 1, CompanyID: 10}}
	s := &ApplicationService{matchRepo: repo, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `user_application_statuses` WHERE user_id = \\? AND company_id = \\? AND status NOT IN \\(\\?,\\?,\\?\\)").
		WithArgs(uint(1), uint(10), "accepted", "withdrawn", "rejected", sqlmock.AnyArg()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec("INSERT INTO `user_application_statuses`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec("UPDATE `user_company_matches` SET `is_applied`=\\?").
		WithArgs(true, sqlmock.AnyArg(), uint(5)).
		WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	_, err := s.Apply(1, 10, 5)
	if err == nil {
		t.Fatal("MarkAsApplied相当の更新が失敗した場合はエラーになるべき")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ロールバックされていない可能性がある: %v", err)
	}
}

// terminalStatusList は Apply の重複チェッククエリに使う終了状態一覧。
// terminalStatuses マップ（UpdateStatus が参照）と内容が一致していることを保証する
// （テーブル駆動: 全ステータス × 終了状態判定の一致確認）。
func TestTerminalStatusListMatchesTerminalStatusesMap(t *testing.T) {
	if len(terminalStatusList) != len(terminalStatuses) {
		t.Fatalf("terminalStatusList(%v) と terminalStatuses(%v) の件数が不一致", terminalStatusList, terminalStatuses)
	}
	for _, status := range terminalStatusList {
		if !terminalStatuses[status] {
			t.Errorf("terminalStatusList の %s が terminalStatuses に無い", status)
		}
	}

	for _, status := range ValidStatuses {
		wantTerminal := terminalStatuses[status]
		gotInList := false
		for _, s := range terminalStatusList {
			if s == status {
				gotInList = true
				break
			}
		}
		if wantTerminal != gotInList {
			t.Errorf("%s: terminalStatuses=%v だが terminalStatusList 含有=%v", status, wantTerminal, gotInList)
		}
	}
}
