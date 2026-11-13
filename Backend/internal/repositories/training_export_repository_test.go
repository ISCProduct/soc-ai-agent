package repositories

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 学習データのエクスポートの SQL を、WHERE 条件と引数まで検証する。
// 特に「AI 発話を混ぜない」(role = user)は、混ざると蒸留にあたる(プロジェクトの必須条件)ので、
// 発話を数える・取り出す SQL のすべてで固定する。

func newTrainingExportTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("sqlmock: %v", err)
		}
		_ = sqlDB.Close()
	})
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

var outcomeStatuses = []string{"rejected", "offered", "accepted"}

func TestTrainingExportRepository_CountOutcomeApplications(t *testing.T) {
	db, mock := newTrainingExportTestDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_application_statuses` WHERE status IN \\(\\?,\\?,\\?\\)").
		WithArgs("rejected", "offered", "accepted").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	n, err := NewTrainingExportRepository(db).CountOutcomeApplications(context.Background(), outcomeStatuses)
	if err != nil || n != 7 {
		t.Fatalf("n=%d err=%v, want 7", n, err)
	}
}

func TestTrainingExportRepository_CountSessionsByStatus(t *testing.T) {
	db, mock := newTrainingExportTestDB(t)
	mock.ExpectQuery("COUNT\\(DISTINCT s.id\\) AS n.*JOIN interview_sessions AS s ON s.user_id = a.user_id AND s.company_id = a.company_id.*WHERE a.status IN \\(\\?,\\?,\\?\\) GROUP BY .a.\\..status.").
		WithArgs("rejected", "offered", "accepted").
		WillReturnRows(sqlmock.NewRows([]string{"status", "n"}).AddRow("offered", 2).AddRow("rejected", 3))

	got, err := NewTrainingExportRepository(db).CountSessionsByStatus(context.Background(), outcomeStatuses)
	if err != nil || len(got) != 2 || got["offered"] != 2 || got["rejected"] != 3 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestTrainingExportRepository_CountExportableSessions_OnlyUserUtterances(t *testing.T) {
	db, mock := newTrainingExportTestDB(t)
	// JOIN の u.role = ? に "user" が先、その後ろの WHERE に statuses が来る
	mock.ExpectQuery("JOIN interview_utterances AS u ON u.session_id = s.id AND u.role = \\?.*WHERE a.status IN \\(\\?,\\?,\\?\\)").
		WithArgs("user", "rejected", "offered", "accepted").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))

	n, err := NewTrainingExportRepository(db).CountExportableSessions(context.Background(), outcomeStatuses)
	if err != nil || n != 4 {
		t.Fatalf("n=%d err=%v, want 4", n, err)
	}
}

func TestTrainingExportRepository_CountUserUtterances_OnlyUserUtterances(t *testing.T) {
	db, mock := newTrainingExportTestDB(t)
	mock.ExpectQuery("WHERE a.status IN \\(\\?,\\?,\\?\\) AND u.role = \\?").
		WithArgs("rejected", "offered", "accepted", "user").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(30))

	n, err := NewTrainingExportRepository(db).CountUserUtterances(context.Background(), outcomeStatuses)
	if err != nil || n != 30 {
		t.Fatalf("n=%d err=%v, want 30", n, err)
	}
}

func TestTrainingExportRepository_ListOutcomeSessions(t *testing.T) {
	t.Run("limit なしは全件、id 順", func(t *testing.T) {
		db, mock := newTrainingExportTestDB(t)
		mock.ExpectQuery("SELECT s.id AS session_id, a.status AS status FROM .*JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id WHERE a.status IN \\(\\?,\\?,\\?\\) ORDER BY s.id$").
			WithArgs("rejected", "offered", "accepted").
			WillReturnRows(sqlmock.NewRows([]string{"session_id", "status"}).AddRow(1, "offered").AddRow(2, "rejected"))

		got, err := NewTrainingExportRepository(db).ListOutcomeSessions(context.Background(), outcomeStatuses, 0)
		if err != nil || len(got) != 2 || got[0].SessionID != 1 || got[0].Status != "offered" || got[1].SessionID != 2 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("limit つきは LIMIT を付ける", func(t *testing.T) {
		db, mock := newTrainingExportTestDB(t)
		mock.ExpectQuery("ORDER BY s.id LIMIT \\?").
			WithArgs("rejected", "offered", "accepted", 5).
			WillReturnRows(sqlmock.NewRows([]string{"session_id", "status"}))

		got, err := NewTrainingExportRepository(db).ListOutcomeSessions(context.Background(), outcomeStatuses, 5)
		if err != nil || len(got) != 0 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
}

func TestTrainingExportRepository_ListUserUtterances_OnlyUserRole(t *testing.T) {
	db, mock := newTrainingExportTestDB(t)
	mock.ExpectQuery("SELECT session_id, text FROM `interview_utterances` WHERE session_id IN \\(\\?,\\?\\) AND role = \\? ORDER BY session_id, created_at, id").
		WithArgs(uint(1), uint(2), "user").
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "text"}).AddRow(1, "志望動機は…").AddRow(2, "強みは…"))

	got, err := NewTrainingExportRepository(db).ListUserUtterances(context.Background(), []uint{1, 2})
	if err != nil || len(got) != 2 || got[0].SessionID != 1 || got[0].Text != "志望動機は…" || got[1].SessionID != 2 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
