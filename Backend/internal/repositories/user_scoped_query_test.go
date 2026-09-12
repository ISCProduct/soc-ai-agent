package repositories

import (
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
func TestUserScopedQueries_IncludeUserID(t *testing.T) {
	const userID = uint(7)

	tests := []struct {
		name         string
		expectQuery  string
		rows         *sqlmock.Rows
		call         func(db *gorm.DB) error
		wantContains []string
	}{
		{
			name:        "チャット履歴",
			expectQuery: "SELECT \\* FROM `chat_messages`",
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
			mock.ExpectQuery(tt.expectQuery).WillReturnRows(tt.rows)

			if err := tt.call(db); err != nil {
				t.Fatalf("unexpected error: %v", err)
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
		})
	}
}

// TestExistsBySessionID_DoesNotScopeByUser は所有者判定の第2段（セッションの存在確認）が
// 意図的に user_id でスコープされないことを固定する（#1156）。
//
// ここを user_id でスコープすると「新規セッション」と「他人のセッション」の区別が
// できなくなり、他人のセッションIDでの開始を許してしまう。
func TestExistsBySessionID_DoesNotScopeByUser(t *testing.T) {
	db, mock, captured := newScopedQueryDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chat_messages`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := NewChatMessageRepository(db).ExistsBySessionID("s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("メッセージが存在するのに false")
	}
	sql := (*captured)[0]
	if !strings.Contains(sql, "session_id = ?") {
		t.Errorf("session_id 条件が無い: %s", sql)
	}
	if strings.Contains(sql, "user_id") {
		t.Errorf("存在確認は user_id でスコープしない（新規と他人のセッションを区別できなくなる）: %s", sql)
	}
}
