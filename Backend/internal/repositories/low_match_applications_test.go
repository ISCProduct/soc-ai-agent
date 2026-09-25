package repositories

import (
	"database/sql/driver"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestFindLowMatchApplicationsByUsers_Query は発行SQLと束縛値を検証する（#1124）。
//
// SQL文字列だけを見ると、閾値と軸数の下限を取り違えて渡しても素通りする。
// 引数の順序が入れ替わると「軸数4未満の応募だけを低マッチとして出す」という
// 逆の挙動になるため、束縛値まで固定する。
func TestFindLowMatchApplicationsByUsers_Query(t *testing.T) {
	db, mock, captured := newScopedQueryDB(t)
	repo := NewUserApplicationStatusRepository(db)

	mock.ExpectQuery("SELECT .* FROM user_application_statuses AS a").
		WithArgs(
			driver.Value(int64(1)), driver.Value(int64(2)),
			"withdrawn", "rejected", "not_applied",
			40.0,
			4,
		).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "company_name", "match_score", "status"}).
			AddRow(2, "サンプル商事", 21.5, "applied"))

	got, err := repo.FindLowMatchApplicationsByUsers([]uint{1, 2}, 40.0, 4)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(got[2]) != 1 || got[2][0].CompanyName != "サンプル商事" {
		t.Errorf("生徒IDごとに束ねられていない: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("期待した束縛値でクエリが発行されていない: %v", err)
	}

	if len(*captured) == 0 {
		t.Fatal("SQLが記録されていない")
	}
	sql := (*captured)[0]
	for _, want := range []string{
		"match_score",        // 閾値による絞り込み
		"matched_axis_count", // 算出軸が少ない行の除外（#1124）
		"user_id",            // 生徒スコープ
		"status",             // 終了済み応募の除外
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("発行SQLに %q が無い:\n%s", want, sql)
		}
	}
	// 境界値ちょうどを含めると学生側の needsLowMatchConfirm と食い違う。
	if !strings.Contains(sql, "match_score` < ?") && !strings.Contains(sql, "match_score < ?") {
		t.Errorf("閾値が < ではない（<= だと学生側と食い違う）:\n%s", sql)
	}
	if !strings.Contains(sql, "matched_axis_count` >= ?") && !strings.Contains(sql, "matched_axis_count >= ?") {
		t.Errorf("軸数の下限が >= ではない:\n%s", sql)
	}
}

// 生徒が0人ならクエリを投げない（空の IN ? は全件条件になり得る）。
func TestFindLowMatchApplicationsByUsers_NoUsers(t *testing.T) {
	db, mock, captured := newScopedQueryDB(t)
	repo := NewUserApplicationStatusRepository(db)

	got, err := repo.FindLowMatchApplicationsByUsers(nil, 40.0, 4)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("空を返すこと: %+v", got)
	}
	if len(*captured) != 0 {
		t.Errorf("生徒0人でクエリを投げている: %v", *captured)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("想定外のクエリ: %v", err)
	}
}
