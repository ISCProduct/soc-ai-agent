package repositories

import (
	"database/sql/driver"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newScopedQueryDB は発行SQLを記録する sqlmock 付きの *gorm.DB を返す。
func newScopedQueryDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *[]string) {
	t.Helper()
	captured := &[]string{}
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		*captured = append(*captured, actualSQL)
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
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
	return db, mock, captured
}

// TestUserScopedQueries_IncludeUserID は「ForUser」系メソッドが発行するSQLに
// user_id 条件が必ず入ることを検証する（#1156）。
//
// 呼び出し側の所有者比較を消しても、クエリがスコープされていれば他人のデータは返らない。
// 逆にここから user_id が落ちると、その多層防御が無言で失われる。
//
// SQL 文字列だけでなく束縛値も検証する。文字列一致だけだと
// `Where("session_id = ? OR user_id = ?", ...)` や引数の取り違え
// （FindDocumentByIDForUser(userID, id) のような位置引数の逆転）が素通りする。
func TestUserScopedQueries_IncludeUserID(t *testing.T) {
	const userID = uint(7)

	tests := []struct {
		name         string
		expectQuery  string
		args         []driver.Value
		rows         *sqlmock.Rows
		call         func(db *gorm.DB) error
		wantContains []string
	}{
		{
			name:        "チャット履歴",
			expectQuery: "SELECT \\* FROM `chat_messages`",
			args:        []driver.Value{"s1", userID},
			rows:        sqlmock.NewRows([]string{"id", "session_id", "user_id"}),
			call: func(db *gorm.DB) error {
				_, err := NewChatMessageRepository(db).FindBySessionIDForUser("s1", userID)
				return err
			},
			wantContains: []string{"session_id = ?", "user_id = ?"},
		},
		{
			name:        "チャット履歴(最新N件・LLMコンテキスト)",
			expectQuery: "SELECT \\* FROM `chat_messages`",
			args:        []driver.Value{"s1", userID, 100},
			rows:        sqlmock.NewRows([]string{"id", "session_id", "user_id"}),
			call: func(db *gorm.DB) error {
				_, err := NewChatMessageRepository(db).FindRecentBySessionIDForUser("s1", userID, 100)
				return err
			},
			wantContains: []string{"session_id = ?", "user_id = ?", "LIMIT"},
		},
		{
			name:        "職務経歴書",
			expectQuery: "SELECT \\* FROM `resume_documents`",
			args:        []driver.Value{1, userID, 1},
			rows:        sqlmock.NewRows([]string{"id", "user_id"}).AddRow(1, userID),
			call: func(db *gorm.DB) error {
				_, err := NewResumeRepository(db).FindDocumentByIDForUser(1, userID)
				return err
			},
			wantContains: []string{"id = ?", "user_id = ?"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, captured := newScopedQueryDB(t)
			mock.ExpectQuery(tt.expectQuery).WithArgs(tt.args...).WillReturnRows(tt.rows)

			if err := tt.call(db); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("期待したクエリ・束縛値と一致しない: %v", err)
			}
			if len(*captured) == 0 {
				t.Fatal("クエリが発行されていない")
			}
			sql := (*captured)[0]
			for _, want := range tt.wantContains {
				if !strings.Contains(sql, want) {
					t.Errorf("SQL に %q が含まれていない: %s", want, sql)
				}
			}
			if strings.Contains(sql, " OR ") {
				t.Errorf("OR 条件が入るとスコープが崩れる: %s", sql)
			}
		})
	}
}

// TestExistsBySessionIDForOtherUser は所有者判定が「自分以外のメッセージの有無」で
// 行われることを固定する（#1156）。
//
// user_id = ? でスコープすると1段目と同義になり、他人のセッションIDでの開始を
// 許してしまう。逆にスコープを外すと自分のセッションでも常に拒否になる。
// user_id <> ? であることが両方を満たす唯一の形。
func TestExistsBySessionIDForOtherUser(t *testing.T) {
	const userID = uint(7)
	db, mock, captured := newScopedQueryDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chat_messages`").
		WithArgs("s1", userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := NewChatMessageRepository(db).ExistsBySessionIDForOtherUser("s1", userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("他人のメッセージが存在するのに false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("期待したクエリ・束縛値と一致しない: %v", err)
	}
	sql := (*captured)[0]
	if !strings.Contains(sql, "session_id = ?") {
		t.Errorf("session_id 条件が無い: %s", sql)
	}
	if !strings.Contains(sql, "user_id <> ?") {
		t.Errorf("他人判定は user_id <> ? である必要がある: %s", sql)
	}
}
