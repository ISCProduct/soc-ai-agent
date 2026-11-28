package repositories

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/models"
)

// #2028: 学生の質問の上限は、数えてから作るまでを1つのトランザクションで行い、学生の行をロックする。
// SQL は部分一致で見て、バインド引数は本人の ID であることを確かめる。

func newGuidanceSQLMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	matcher := sqlmock.QueryMatcherFunc(func(want, got string) error {
		for _, part := range strings.Split(want, "|") {
			if !strings.Contains(got, part) {
				return errors.New("SQL に " + part + " が無い: " + got)
			}
		}
		return nil
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

const openQuestionsSQL = "FROM teacher_student_guidances g|g.student_user_id = ? AND g.kind = ?|m.author_role = ?"

func TestGuidanceMessageRepository_CreateQuestion(t *testing.T) {
	withdrawn := time.Unix(1700000000, 0)
	for _, tt := range []struct {
		name      string
		lockRows  *sqlmock.Rows
		open      int // -1 なら件数を数えない
		wantWrite bool
		wantErr   error
	}{
		{"上限未満なら同じトランザクションで作る", sqlmock.NewRows([]string{"id", "withdrawn_at"}).AddRow(10, nil), 4, true, nil},
		{"上限なら作らない", sqlmock.NewRows([]string{"id", "withdrawn_at"}).AddRow(10, nil), 5, false, ErrOpenQuestionLimit},
		// 学校の確認のあとに退会が確定した（ロックが退会のコミットを待って、退会済みの行を返す）
		{"ロックのあとに退会済みなら作らない", sqlmock.NewRows([]string{"id", "withdrawn_at"}).AddRow(10, withdrawn), -1, false, ErrQuestionStudentGone},
		{"学生の行が無ければ作らない", sqlmock.NewRows([]string{"id", "withdrawn_at"}), -1, false, ErrQuestionStudentGone},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newGuidanceSQLMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id, withdrawn_at FROM users WHERE id = ? FOR UPDATE").WithArgs(10).WillReturnRows(tt.lockRows)
			if tt.open >= 0 {
				mock.ExpectQuery(openQuestionsSQL).WithArgs(10, models.GuidanceKindQuestion, models.GuidanceAuthorTeacher).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.open))
			}
			if tt.wantWrite {
				mock.ExpectExec("INSERT INTO `teacher_student_guidances`").WillReturnResult(sqlmock.NewResult(7, 1))
				mock.ExpectExec("INSERT INTO `guidance_messages`").WillReturnResult(sqlmock.NewResult(42, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			g, msg, err := NewGuidanceMessageRepository(db).CreateQuestion(context.Background(), 10, "質問", 5)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantWrite {
				if g.ID != 7 || g.StudentUserID != 10 || g.Kind != models.GuidanceKindQuestion {
					t.Fatalf("g = %+v", g)
				}
				// 保存した最初の発言を、実際の ID と作成日時つきで返す（読み直しに失敗したときの応答に使う）
				if msg.ID != 42 || msg.GuidanceID != 7 || msg.CreatedAt.IsZero() {
					t.Fatalf("msg = %+v", msg)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGuidanceMessageRepository_CreateMessageAndMarkRead(t *testing.T) {
	for _, tt := range []struct {
		name     string
		parent   *sqlmock.Rows
		seenIDs  []uint
		wantMark bool
		wantErr  error
	}{
		{"画面に出ていた ID だけを既読にする", sqlmock.NewRows([]string{"id"}).AddRow(1), []uint{3, 7}, true, nil},
		{"ID が無ければ既読にしない", sqlmock.NewRows([]string{"id"}).AddRow(1), nil, false, nil},
		// 退会者の削除で親が消えていたら、親の無い発言を残さない
		{"親が無ければ書かない", sqlmock.NewRows([]string{"id"}), []uint{3}, false, gorm.ErrRecordNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newGuidanceSQLMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM teacher_student_guidances WHERE id = ? FOR UPDATE").WithArgs(1).WillReturnRows(tt.parent)
			if tt.wantErr == nil {
				mock.ExpectExec("INSERT INTO `guidance_messages`").WillReturnResult(sqlmock.NewResult(9, 1))
				mock.ExpectExec("UPDATE `teacher_student_guidances`|`dismissed_at`=?").WillReturnResult(sqlmock.NewResult(0, 1))
				if tt.wantMark {
					mock.ExpectExec("UPDATE `guidance_messages`|author_role <> ?|read_at IS NULL|id IN (?,?)").
						WithArgs(sqlmock.AnyArg(), 1, models.GuidanceAuthorTeacher, 3, 7).
						WillReturnResult(sqlmock.NewResult(0, 2))
				}
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			m := &models.GuidanceMessage{GuidanceID: 1, AuthorUserID: 20, AuthorRole: models.GuidanceAuthorTeacher, Body: "答えます"}
			err := NewGuidanceMessageRepository(db).CreateMessageAndMarkRead(context.Background(), m, tt.seenIDs, time.Now())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 学生が自分で送った質問は、ホームの「先生から」の案内（10件の枠）に出さない。
func TestTeacherStudentGuidanceRepository_ListActiveByStudent_質問を除く(t *testing.T) {
	db, mock := newGuidanceSQLMock(t)
	mock.ExpectQuery("student_user_id = ? AND dismissed_at IS NULL AND kind <> ?|ORDER BY created_at DESC, id DESC|LIMIT ?").
		WithArgs(10, models.GuidanceKindQuestion, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := NewTeacherStudentGuidanceRepository(db).ListActiveByStudent(10, 10); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
