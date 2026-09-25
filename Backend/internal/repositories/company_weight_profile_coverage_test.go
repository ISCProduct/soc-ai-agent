package repositories

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestCountPublishedWithoutWeightProfile_Query は発行SQLを固定する（#1380）。
//
// 「公開中でプロファイルが無い企業」の数は、マッチング対象から外れた企業数そのもの。
// 公開条件が CountPublished / FindAllPublished とずれたり、
// job_position_id IS NULL の条件が落ちて求人単位プロファイルを拾ったりすると、
// 運用者が見る数字だけが実態と食い違う。
func TestCountPublishedWithoutWeightProfile_Query(t *testing.T) {
	db, mock, captured := newScopedQueryDB(t)
	repo := NewCompanyRepository(db)

	mock.ExpectQuery("SELECT count").
		WithArgs(true, "published").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	got, err := repo.CountPublishedWithoutWeightProfile()
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("期待した束縛値でクエリが発行されていない: %v", err)
	}

	if len(*captured) == 0 {
		t.Fatal("SQLが記録されていない")
	}
	sql := (*captured)[0]
	for _, want := range []string{
		"is_active",               // 公開判定（CountPublished と同条件）
		"data_status",             // 同上
		"NOT EXISTS",              // プロファイルを持たない企業に限定
		"company_weight_profiles", // 突き合わせ先
		"company_weight_profiles.job_position_id IS NULL", // 会社単位プロファイルのみ
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("発行SQLに %q が無い:\n%s", want, sql)
		}
	}
}
