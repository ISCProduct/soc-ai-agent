package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// サーバーは1ターンの user と ai を続けて保存するので、created_at が同じミリ秒になりうる。
// 同値の並びを id で固定しないと、書き起こしと面接官の履歴で ai が user より先に並ぶ（#1969）。
func TestInterviewUtteranceRepository_FindBySessionIDOrdersByID(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm作成失敗: %v", err)
	}
	mock.ExpectQuery("SELECT \\* FROM `interview_utterances` WHERE session_id = \\? ORDER BY created_at ASC, id ASC").
		WithArgs(uint(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id", "role", "text"}).
			AddRow(1, 5, "user", "回答").
			AddRow(2, 5, "ai", "質問"))

	got, err := NewInterviewUtteranceRepository(db).FindBySessionID(5)
	if err != nil {
		t.Fatalf("FindBySessionID: %v", err)
	}
	if len(got) != 2 || got[0].Role != "user" || got[1].Role != "ai" {
		t.Fatalf("got = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
