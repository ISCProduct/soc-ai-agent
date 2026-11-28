package chat

// recommendations に載せる診断信頼度の読み出し（loadDiagnosisQuality）のテスト。
// 未設定・読めない・無いときはゼロ値にし、推薦の応答を失敗させないことを確かめる。

import (
	"errors"
	"regexp"
	"testing"

	"Backend/internal/repositories"
	"Backend/internal/services/diagnosis"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var selectQualityBySession = regexp.QuoteMeta("SELECT * FROM `diagnosis_quality_reports` WHERE user_id = ? AND session_id = ?")

func newChatControllerWithQuality(t *testing.T) (*ChatController, sqlmock.Sqlmock) {
	t.Helper()
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
	c := &ChatController{}
	c.SetDiagnosisQualityService(diagnosis.NewQualityService(repositories.NewDiagnosisQualityRepository(db), nil, nil, nil, nil))
	return c, mock
}

func TestLoadDiagnosisQuality(t *testing.T) {
	t.Run("未設定ならゼロ値", func(t *testing.T) {
		conf, flags, summary := (&ChatController{}).loadDiagnosisQuality(7, "sess-1")
		assert.Equal(t, 0, conf)
		assert.Nil(t, flags)
		assert.Equal(t, "", summary)
	})

	t.Run("レポートがあれば信頼度・フラグ・要約を返す", func(t *testing.T) {
		c, mock := newChatControllerWithQuality(t)
		mock.ExpectQuery(selectQualityBySession).WithArgs(uint(7), "sess-1", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "confidence", "flags_json", "summary"}).AddRow(1, 65, `["low_evidence"]`, "合成サマリー"))
		conf, flags, summary := c.loadDiagnosisQuality(7, "sess-1")
		assert.Equal(t, 65, conf)
		assert.Equal(t, []string{"low_evidence"}, flags)
		assert.Equal(t, "合成サマリー", summary)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("読めなければゼロ値", func(t *testing.T) {
		c, mock := newChatControllerWithQuality(t)
		mock.ExpectQuery(selectQualityBySession).WithArgs(uint(7), "sess-1", 1).WillReturnError(errors.New("db down"))
		conf, flags, summary := c.loadDiagnosisQuality(7, "sess-1")
		assert.Equal(t, 0, conf)
		assert.Nil(t, flags)
		assert.Equal(t, "", summary)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
