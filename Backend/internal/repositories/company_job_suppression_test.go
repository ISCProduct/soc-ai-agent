package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestListJobPositions_SchoolExcludesSuppressed は学校指定時に個別停止された求人を
// 除外する JOIN 条件が入ることを検証する（#1508）。
// これが崩れると、キャリア担当が止めた求人が学生に出続ける。
func TestListJobPositions_SchoolExcludesSuppressed(t *testing.T) {
	repo, mock := newCompanyRepoTestDB(t)
	schoolID := uint(3)

	mock.ExpectQuery("LEFT JOIN school_job_suppressions ON school_job_suppressions.job_position_id = company_job_positions.id AND school_job_suppressions.school_id = \\?.*WHERE school_job_suppressions.id IS NULL").
		WithArgs(schoolID, schoolID, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id"}))

	if _, err := repo.ListJobPositions(nil, &schoolID, 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestListJobPositions_NoSchoolNoSuppression は学校未指定なら停止JOINを付けないことを検証する。
func TestListJobPositions_NoSchoolNoSuppression(t *testing.T) {
	repo, mock := newCompanyRepoTestDB(t)
	companyID := uint(7)

	mock.ExpectQuery("SELECT \\* FROM `company_job_positions` WHERE company_id = \\?").
		WithArgs(companyID, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id"}))

	if _, err := repo.ListJobPositions(&companyID, nil, 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
