package repositories

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 確認（行ロックつきの SELECT）と加算を1つのトランザクションで行い、上限に達していたら加算しない（SOCAIAGENT-724）。
func TestResearchChatQuotaRepository_Reserve(t *testing.T) {
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		used int
		want bool
	}{
		{"上限未満なら1つ増やす", 29, true},
		{"上限に達していたら増やさない", 30, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)

			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO `company_research_chat_quotas`").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("SELECT \\* FROM `company_research_chat_quotas` WHERE .* FOR UPDATE").
				WillReturnRows(sqlmock.NewRows([]string{"user_id", "quota_date", "used", "updated_at"}).AddRow(7, day, tt.used, day))
			if tt.want {
				mock.ExpectExec("UPDATE `company_research_chat_quotas` SET `updated_at`=.*,`used`=used \\+ 1").
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectCommit()

			got, err := NewResearchChatQuotaRepository(db).Reserve(context.Background(), 7, day, 30)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
