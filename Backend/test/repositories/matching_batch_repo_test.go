package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/domain/entity"
	"Backend/internal/repositories"
)

func TestGetWeightProfilesByCompanyIDs(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewCompanyRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `company_weight_profiles` WHERE company_id IN \\(\\?,\\?\\) AND job_position_id IS NULL").
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "technical_orientation"}).
			AddRow(10, 1, 80).
			AddRow(11, 2, 40))

	got, err := repo.GetWeightProfilesByCompanyIDs([]uint{1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[1] == nil || got[1].TechnicalOrientation != 80 {
		t.Fatalf("unexpected profiles: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetWeightProfilesByCompanyIDs_Empty(t *testing.T) {
	repo := repositories.NewCompanyRepository(nil)
	got, err := repo.GetWeightProfilesByCompanyIDs(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty ids should return empty map, got %v err %v", got, err)
	}
}

func TestCreateOrUpdateBatch_CreatesAndUpdates(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewUserCompanyMatchRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `user_company_matches` WHERE user_id = \\? AND session_id = \\?").
		WithArgs(uint(1), "s1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "session_id", "company_id", "match_score", "is_favorited", "is_viewed", "is_applied",
		}).AddRow(99, 1, "s1", 2, 50.0, true, false, false))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_company_matches`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_company_matches`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	saved, err := repo.CreateOrUpdateBatch([]*entity.UserCompanyMatch{
		{UserID: 1, SessionID: "s1", CompanyID: 1, MatchScore: 80},
		{UserID: 1, SessionID: "s1", CompanyID: 2, MatchScore: 90},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved != 2 {
		t.Fatalf("saved=%d want 2", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
