package repositories

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #2032: 学生向けの進路実績は学校で絞り、卒業生の氏名・学科・メモを読まない。

// newGraduateOutcomeMock は、実際に発行された SQL をすべて控える sqlmock を返す。
// 期待する SQL の照合は、各テストで「含むべき条件」を見て行う。
func newGraduateOutcomeMock(t *testing.T) (*GraduateOutcomeRepository, sqlmock.Sqlmock, *[]string) {
	t.Helper()
	var sqls []string
	matcher := sqlmock.QueryMatcherFunc(func(want, got string) error {
		sqls = append(sqls, got)
		for _, part := range strings.Split(want, "|") {
			if !strings.Contains(got, part) {
				return errors.New("SQL に " + part + " が無い: " + got)
			}
		}
		return nil
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	return NewGraduateOutcomeRepository(db), mock, &sqls
}

// 学生向けの集計クエリに必ず入る条件（学校・削除済み・企業の表示可否）。
const outcomeScope = "ge.school_id = ? AND ge.deleted_at IS NULL|c.is_active = true AND c.deleted_at IS NULL|c.is_guest_entry = true|company_entry_submissions"

func assertNoPII(t *testing.T, sqls []string) {
	t.Helper()
	for _, s := range sqls {
		for _, pii := range []string{"graduate_name", "department", "note"} {
			if strings.Contains(s, pii) {
				t.Errorf("卒業生の個人情報（%s）を読んでいる: %s", pii, s)
			}
		}
	}
}

func TestGraduateOutcomeRepository_CountsByCompanyYear(t *testing.T) {
	repo, mock, sqls := newGraduateOutcomeMock(t)
	// 学校 5 の ID がバインドされ、年度・業界の絞り込みが後に続く。年度が NULL の行は 0 として数える
	mock.ExpectQuery(outcomeScope+"|COALESCE(ge.graduation_year, 0) AS graduation_year|ge.graduation_year = ?|c.industry = ?").
		WithArgs(5, 2025, "製造業").
		WillReturnRows(sqlmock.NewRows([]string{"company_id", "company_name", "industry", "graduation_year", "count"}).
			AddRow(1, "A社", "製造業", 2025, 3))
	rows, err := repo.CountsByCompanyYear(context.Background(), 5, GraduateOutcomeFilter{Year: 2025, Industry: "製造業"})
	if err != nil || len(rows) != 1 || rows[0].CompanyName != "A社" || rows[0].Count != 3 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	assertNoPII(t, *sqls)
}

func TestGraduateOutcomeRepository_FilterOptions(t *testing.T) {
	repo, mock, sqls := newGraduateOutcomeMock(t)
	// 年度が未登録（0・NULL）の行は選択肢に出さない
	mock.ExpectQuery(outcomeScope + "|ge.graduation_year > 0").WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"graduation_year"}).AddRow(2025).AddRow(2024))
	mock.ExpectQuery(outcomeScope + "|c.industry <> ''").WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"industry"}).AddRow("製造業"))
	years, industries, err := repo.FilterOptions(context.Background(), 5)
	if err != nil || len(years) != 2 || len(industries) != 1 {
		t.Fatalf("years=%v industries=%v err=%v", years, industries, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	assertNoPII(t, *sqls)
}

func TestGraduateOutcomeRepository_CompanyCounts(t *testing.T) {
	repo, mock, sqls := newGraduateOutcomeMock(t)
	mock.ExpectQuery(outcomeScope+"|ge.company_id = ?").WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{"company_id", "company_name", "industry", "graduation_year", "count"}).
			AddRow(1, "A社", "", 2025, 3))
	// 職種はその企業のものだけ結ぶ。同数は職種名順で並びを決める
	mock.ExpectQuery(outcomeScope+"|jp.company_id = ge.company_id|ORDER BY count DESC, title").WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{"title", "count"}).AddRow("SE", 3))
	byYear, byPosition, err := repo.CompanyCounts(context.Background(), 5, 1)
	if err != nil || len(byYear) != 1 || len(byPosition) != 1 || byPosition[0].Title != "SE" {
		t.Fatalf("byYear=%+v byPosition=%+v err=%v", byYear, byPosition, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	assertNoPII(t, *sqls)
}

func TestGraduateOutcomeRepository_CompanyVisible(t *testing.T) {
	repo, mock, _ := newGraduateOutcomeMock(t)
	mock.ExpectQuery("c.id = ?|c.is_active = true AND c.deleted_at IS NULL|c.is_guest_entry = true").WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	ok, err := repo.CompanyVisible(context.Background(), 9)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 学校は学園と照合し、合わなければ実績を出さない（fail-close）。学生が存在しなければエラー。
func TestGraduateOutcomeRepository_StudentSchoolID(t *testing.T) {
	user := func(m sqlmock.Sqlmock, school any, org uint) {
		m.ExpectQuery("FROM `users`|withdrawn_at IS NULL").
			WillReturnRows(sqlmock.NewRows([]string{"school_id", "organization_id"}).AddRow(school, org))
	}
	school := func(m sqlmock.Sqlmock, org uint) {
		m.ExpectQuery("FROM `schools`").WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(org))
	}

	t.Run("学園が一致すれば学校を返す", func(t *testing.T) {
		repo, mock, _ := newGraduateOutcomeMock(t)
		user(mock, 5, 2)
		school(mock, 2)
		got, err := repo.StudentSchoolID(context.Background(), 7)
		if err != nil || got == nil || *got != 5 {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("学園が違えば出さない", func(t *testing.T) {
		repo, mock, _ := newGraduateOutcomeMock(t)
		user(mock, 5, 2)
		school(mock, 3)
		if got, err := repo.StudentSchoolID(context.Background(), 7); err != nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("学校が消えていれば出さない", func(t *testing.T) {
		repo, mock, _ := newGraduateOutcomeMock(t)
		user(mock, 5, 2)
		mock.ExpectQuery("FROM `schools`").WillReturnRows(sqlmock.NewRows([]string{"organization_id"}))
		if got, err := repo.StudentSchoolID(context.Background(), 7); err != nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("学校が未登録", func(t *testing.T) {
		repo, mock, _ := newGraduateOutcomeMock(t)
		user(mock, nil, 2)
		if got, err := repo.StudentSchoolID(context.Background(), 7); err != nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("学生が存在しない", func(t *testing.T) {
		repo, mock, _ := newGraduateOutcomeMock(t)
		mock.ExpectQuery("FROM `users`").WillReturnRows(sqlmock.NewRows([]string{"school_id", "organization_id"}))
		if _, err := repo.StudentSchoolID(context.Background(), 7); !errors.Is(err, ErrGraduateOutcomeUserNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
}
