package costs

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

// モデル別内訳は、記録した根拠と記録の無い行の数を見て根拠を付ける（#2004）。
// 古い行（記録なし）と新しい行が混ざる期間に、新しい行の「公開価格」だけで表示しないこと。
func TestGetModelBreakdown_UsesRecordedBasisAndUnrecordedRows(t *testing.T) {
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

	since := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM api_call_logs").
		WithArgs(since).
		WillReturnRows(sqlmock.NewRows([]string{"model", "total_cost_usd", "total_tokens", "call_count", "recorded_price_basis", "unrecorded_calls"}).
			// 新しい行は公開価格として記録、古い行は記録なし。今の単価表では未登録 → 未登録を出す
			AddRow("totally-new-model", 1.0, 100, 3, "listed", 2).
			// すべて記録あり → 記録の値のまま
			AddRow("totally-new-model-2", 1.0, 100, 3, "listed", 0))

	got, err := NewAPICostService(repositories.NewAPICallLogRepository(db)).GetModelBreakdown(since)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(got) != 2 || got[0].PriceBasis != PriceBasisUnknown || got[1].PriceBasis != PriceBasisListed {
		t.Fatalf("got = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
