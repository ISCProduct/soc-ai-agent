package repositories

import (
	"bytes"
	"errors"
	stdlog "log"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 所属校が引けなかったときは利用ログを止めずに nil を返す。利用者がいないときは黙り、
// DB の失敗はログに残す。後者は学校の利用額から外れるので、気づけるようにする。
func TestSchoolIDForLog(t *testing.T) {
	school := uint(3)
	tests := []struct {
		name    string
		rows    *sqlmock.Rows
		err     error
		want    *uint
		wantLog bool
	}{
		{name: "所属校を返す", rows: sqlmock.NewRows([]string{"school_id"}).AddRow(3), want: &school},
		{name: "所属なしは nil", rows: sqlmock.NewRows([]string{"school_id"}).AddRow(nil)},
		{name: "利用者がいなければログを出さない", rows: sqlmock.NewRows([]string{"school_id"})},
		{name: "DB の失敗はログに残す", err: errors.New("connection lost"), wantLog: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			query := mock.ExpectQuery("SELECT `school_id` FROM `users` WHERE id = \\?")
			if tt.err != nil {
				query.WillReturnError(tt.err)
			} else {
				query.WillReturnRows(tt.rows)
			}
			var buf bytes.Buffer
			prev := stdlog.Writer()
			stdlog.SetOutput(&buf)
			defer stdlog.SetOutput(prev)

			got := schoolIDForLog(db, 7)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("school_id = %v, want %v", got, tt.want)
			}
			logged := strings.Contains(buf.String(), "school lookup failed")
			if logged != tt.wantLog {
				t.Fatalf("logged = %v, want %v (%q)", logged, tt.wantLog, buf.String())
			}
		})
	}
}
