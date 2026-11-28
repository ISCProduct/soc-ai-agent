package repositories

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ListRetentionDue の SQL が、保持期間の仕様を表す条件を持つことを確かめる。
// sqlmock は MySQL で SQL を実行しないので、ここでは「条件が消えたら落ちる」ことを守る。
// 実際の日付計算(MySQL 上の評価)はここでは検証しない。purge_test が削除フローを担保する。
func TestListRetentionDue_SQLSpec(t *testing.T) {
	db, mock := newCompanyOwnerTestDB(t)
	repo := NewInterviewVideoRepository(db)
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(".*").
		WithArgs(now, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "object_key"}).AddRow(7, "videos/7.webm").AddRow(9, "videos/9.webm"))

	got, err := repo.ListRetentionDue(context.Background(), now, 0)
	if err != nil {
		t.Fatalf("ListRetentionDue: %v", err)
	}
	if len(got) != 2 || got[0].ID != 7 || got[0].ObjectKey != "videos/7.webm" || got[1].ID != 9 {
		t.Fatalf("結果が想定と違う: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListRetentionDue_SQLConditions(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"退会者は対象外(退会の削除に任せる)", "INNER JOIN users AS u ON u.id = v.user_id AND u.withdrawn_at IS NULL"},
		{"期限なしの学校(日数 NULL)は対象外", "s.interview_video_retention_days IS NOT NULL"},
		{"学校が無い学生も対象に含める", "u.school_id IS NULL OR s.id IS NULL"},
		{"学校が無い学生は 90 日", "IF(u.school_id IS NULL OR s.id IS NULL, 90, s.interview_video_retention_days)"},
		{"境界は「ちょうど期限」を含めない(厳密な <)", "< DATE_SUB(?, INTERVAL"},
		{"公開日が無ければ作成日で数える", "COALESCE(v.uploaded_at, v.created_at)"},
		{"古い順に取り出す", "ORDER BY v.id ASC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newCompanyOwnerTestDB(t)
			repo := NewInterviewVideoRepository(db)
			now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
			// QueryMatcher は部分一致の正規表現なので、期待する断片をそのまま照合する。
			mock.MatchExpectationsInOrder(true)
			mock.ExpectQuery(regexp.QuoteMeta(tt.want)).
				WithArgs(now, 5).
				WillReturnRows(sqlmock.NewRows([]string{"id", "object_key"}))

			if _, err := repo.ListRetentionDue(context.Background(), now, 5); err != nil {
				t.Fatalf("ListRetentionDue: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("SQL に条件が無い: %v", err)
			}
		})
	}
}

func TestListRetentionDue_ReturnsDBError(t *testing.T) {
	db, mock := newCompanyOwnerTestDB(t)
	repo := NewInterviewVideoRepository(db)
	boom := errors.New("db down")
	mock.ExpectQuery("FROM interview_videos").WillReturnError(boom)

	if _, err := repo.ListRetentionDue(context.Background(), time.Now(), 10); !errors.Is(err, boom) {
		t.Fatalf("DB エラーを握りつぶしている: %v", err)
	}
}
