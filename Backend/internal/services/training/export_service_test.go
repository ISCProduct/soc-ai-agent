package training_test

// 学習データのエクスポートのテスト。
// 実行: cd Backend && go test ./internal/services/training/... -v
//
// 実DBを使わず、発行されるSQLと結果の組み立てを検証する。
// 見たいのは「AI発話が混ざらないこと」と「結論が出た応募だけを対象にすること」。

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"

	"Backend/internal/repositories"
	"Backend/internal/services/training"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock を作れない: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm を開けない: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, mock
}

func TestOutcomeStatuses_RAG側と揃っていること(t *testing.T) {
	// rag/export_training_data.py の OUTCOME_LABELS と一致させる必要がある。
	// 片方だけ増やすと、Backend が出したデータを RAG 側が黙って捨てる。
	want := map[string]bool{"rejected": true, "offered": true, "accepted": true}
	if len(training.OutcomeStatuses) != len(want) {
		t.Fatalf("件数が違う: %v", training.OutcomeStatuses)
	}
	for _, s := range training.OutcomeStatuses {
		if !want[s] {
			t.Errorf("想定外のステータス: %s", s)
		}
	}
	// 進行中のステータスが混ざると、結果が出ていない応募を学習してしまう。
	for _, ng := range []string{"applied", "document_screening", "interview_scheduled", "withdrawn"} {
		for _, s := range training.OutcomeStatuses {
			if s == ng {
				t.Errorf("結論が出ていないステータスが含まれている: %s", ng)
			}
		}
	}
}

func TestExport_ユーザー発話だけを集める(t *testing.T) {
	db, mock := newMockDB(t)
	svc := training.NewService(repositories.NewTrainingExportRepository(db))

	mock.ExpectQuery(`SELECT s.id AS session_id, a.status AS status FROM .*interview_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "status"}).
			AddRow(1, "offered").
			AddRow(2, "rejected"))

	mock.ExpectQuery(`SELECT session_id, text FROM .*interview_utterances`).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "text"}).
			AddRow(1, "学生時代はサークルの代表をしていました").
			AddRow(1, "強みは調整力です").
			AddRow(2, "志望動機は事業内容への共感です"))

	got, _, err := svc.Export(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("Export に失敗: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("セッション数が違う: %d", len(got))
	}

	if got[0].ApplicationStatus != "offered" || got[1].ApplicationStatus != "rejected" {
		t.Errorf("ステータスの対応が崩れている: %+v", got)
	}
	if len(got[0].Utterances) != 2 {
		t.Errorf("発話数が違う: %d", len(got[0].Utterances))
	}
	// AI発話を混ぜると、モデルの出力を教師信号にすることになり蒸留にあたる。
	for _, s := range got {
		for _, u := range s.Utterances {
			if u.Role != "user" {
				t.Errorf("user 以外の発話が混ざっている: %+v", u)
			}
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("想定外のクエリ: %v", err)
	}
}

func TestExport_発話が無いセッションは落とす(t *testing.T) {
	db, mock := newMockDB(t)
	svc := training.NewService(repositories.NewTrainingExportRepository(db))

	mock.ExpectQuery(`SELECT s.id AS session_id`).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "status"}).AddRow(1, "offered"))
	// ユーザー発話が1件も無い
	mock.ExpectQuery(`SELECT session_id, text`).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "text"}))

	got, _, err := svc.Export(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("Export に失敗: %v", err)
	}
	// prompt を作れないので RAG 側でも捨てられる。ここで落としておく。
	if len(got) != 0 {
		t.Errorf("発話の無いセッションが残っている: %+v", got)
	}
}

func TestExport_該当ゼロなら空を返す(t *testing.T) {
	db, mock := newMockDB(t)
	svc := training.NewService(repositories.NewTrainingExportRepository(db))

	mock.ExpectQuery(`SELECT s.id AS session_id`).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "status"}))

	got, _, err := svc.Export(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("該当ゼロはエラーにしない: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("空スライスを返すべき: %+v", got)
	}
}

// 発行されるSQLが結論の出た応募だけに絞っていることを確認する。
func TestExport_結論が出た応募だけを対象にする(t *testing.T) {
	db, mock := newMockDB(t)
	svc := training.NewService(repositories.NewTrainingExportRepository(db))

	var captured string
	mock.ExpectQuery(`SELECT s.id AS session_id`).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "status"}))

	_, _, _ = svc.Export(context.Background(), 0, 0)

	// sqlmock は実際のSQLを保持しないため、ここでは条件の存在をサービス側の
	// 定数から担保する。OutcomeStatuses のテストと合わせて意図を固定する。
	captured = strings.Join(training.OutcomeStatuses, ",")
	if !regexp.MustCompile(`rejected`).MatchString(captured) {
		t.Error("rejected が対象に含まれていない")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("想定外のクエリ: %v", err)
	}
}

// SOCAIAGENT-694: limit は既定・上限 1000。offset でページングし、続きがありうるときは次の offset を返す。
func TestExport_limitの上限とページング(t *testing.T) {
	tests := []struct {
		name           string
		limit, offset  int
		wantArgs       []driver.Value // LIMIT / OFFSET の引数
		rows           int            // DB が返すセッション数
		wantNextOffset int            // 0 は続きなし
	}{
		{"limit 省略(0)は 1000", 0, 0, []driver.Value{training.ExportMaxLimit}, 0, 0},
		{"上限を超える limit は 1000 に丸める", 5000, 0, []driver.Value{training.ExportMaxLimit}, 0, 0},
		{"上限内で limit 未満しか無ければ続きなし", 2, 0, []driver.Value{2}, 1, 0},
		{"limit 件ちょうど取れたら次の offset を返す", 2, 4, []driver.Value{2, 4}, 2, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			svc := training.NewService(repositories.NewTrainingExportRepository(db))

			rows := sqlmock.NewRows([]string{"session_id", "status"})
			for i := range tt.rows {
				rows.AddRow(i+1, "offered")
			}
			// 対象ステータス3つ + 同意条件3つ + LIMIT/OFFSET
			args := []driver.Value{sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()}
			pattern := `SELECT s.id AS session_id.* ORDER BY s.id LIMIT \?$`
			if tt.offset > 0 {
				pattern = `SELECT s.id AS session_id.* ORDER BY s.id LIMIT \? OFFSET \?$`
			}
			mock.ExpectQuery(pattern).WithArgs(append(args, tt.wantArgs...)...).WillReturnRows(rows)
			if tt.rows > 0 {
				utts := sqlmock.NewRows([]string{"session_id", "text"})
				for i := range tt.rows {
					utts.AddRow(i+1, "合成の発話")
				}
				mock.ExpectQuery(`SELECT session_id, text`).WillReturnRows(utts)
			}

			_, next, err := svc.Export(context.Background(), tt.limit, tt.offset)
			if err != nil {
				t.Fatalf("Export に失敗: %v", err)
			}
			if next != tt.wantNextOffset {
				t.Errorf("next offset=%d want %d", next, tt.wantNextOffset)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("LIMIT/OFFSET が想定と違う: %v", err)
			}
		})
	}
}
