package repositories

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func TestGuidanceMessageRepository_CreateQuestion_上限なら作らない(t *testing.T) {
	db, mock := newGuidanceSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM users WHERE id = ? FOR UPDATE").WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectQuery(openQuestionsSQL).WithArgs(10, models.GuidanceKindQuestion, models.GuidanceAuthorTeacher).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectRollback()

	_, err := NewGuidanceMessageRepository(db).CreateQuestion(context.Background(), 10, "質問", 5)
	if !errors.Is(err, ErrOpenQuestionLimit) {
		t.Fatalf("err = %v, want ErrOpenQuestionLimit", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuidanceMessageRepository_CreateQuestion_上限未満なら同じトランザクションで作る(t *testing.T) {
	db, mock := newGuidanceSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM users WHERE id = ? FOR UPDATE").WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectQuery(openQuestionsSQL).WithArgs(10, models.GuidanceKindQuestion, models.GuidanceAuthorTeacher).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	mock.ExpectExec("INSERT INTO `teacher_student_guidances`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO `guidance_messages`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	g, err := NewGuidanceMessageRepository(db).CreateQuestion(context.Background(), 10, "質問", 5)
	if err != nil || g.ID != 7 || g.StudentUserID != 10 || g.Kind != models.GuidanceKindQuestion {
		t.Fatalf("g = %+v err = %v", g, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
