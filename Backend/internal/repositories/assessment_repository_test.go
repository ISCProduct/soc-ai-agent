package repositories

import (
	"database/sql/driver"
	"errors"
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAssessmentRepo(t *testing.T) (*AssessmentRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewAssessmentRepository(db), mock
}

// 受検中の一意制約（uniq_assessment_sessions_active）に負けたら、呼び出し側が読み直せる専用エラーにする。
func TestAssessmentRepository_CreateSessionDuplicateActive(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `assessment_sessions`").
		WillReturnError(&gomysql.MySQLError{Number: 1062, Message: "Duplicate entry for key 'uniq_assessment_sessions_active'"})
	mock.ExpectRollback()
	err := repo.CreateSession(&models.AssessmentSession{PublicID: "p", UserID: 7, TemplateID: 1, TemplateVersion: 1, Status: models.AssessmentStatusInProgress})
	require.ErrorIs(t, err, ErrAssessmentSessionExists)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 自動保存の再送は1行に畳む（#1678）。INSERT だけだと UNIQUE 違反で2回目が失敗する。
func TestAssessmentRepository_UpsertAnswerIsIdempotent(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	for range 2 {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO `assessment_answers`.*ON DUPLICATE KEY UPDATE `value`=VALUES\\(`value`\\),`updated_at`=VALUES\\(`updated_at`\\)").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		require.NoError(t, repo.UpsertAnswer(3, 11, 4))
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

// 提出は条件付き UPDATE で排他する。0行なら二重提出で、何も書かずにロールバックする。
func TestAssessmentRepository_SubmitTwiceRollsBack(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `assessment_sessions` SET .* WHERE id = \\? AND status = \\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repo.Submit(&models.AssessmentSession{ID: 3, UserID: 7, Status: models.AssessmentStatusInProgress}, map[string]float64{"技術志向": 50}, "assessment:x", true)
	require.True(t, errors.Is(err, ErrAssessmentAlreadySubmitted), "err = %v", err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 成功時は結果とスコアを同じトランザクションで書く。スコアは source=assessment の新しい行で、
// チャット由来の行には UPDATE も DELETE も発行しない（sqlmock は想定外の SQL で失敗する）。
func TestAssessmentRepository_SubmitWritesAssessmentRowsOnly(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `assessment_sessions` SET .* WHERE id = \\? AND status = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `assessment_results`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT .*organization_id.* FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(1))
	mock.ExpectExec("INSERT INTO `user_weight_scores` \\(`organization_id`,`user_id`,`session_id`,`source`,`weight_category`,`score`,`created_at`,`updated_at`\\)").
		WithArgs(1, 7, "assessment:x", models.ScoreSourceAssessment, "技術志向", 63, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	s := &models.AssessmentSession{ID: 3, UserID: 7, Status: models.AssessmentStatusInProgress}
	require.NoError(t, repo.Submit(s, map[string]float64{"技術志向": 62.5}, "assessment:x", true))
	require.Equal(t, models.AssessmentStatusSubmitted, s.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 正典外カテゴリは UserWeightScoreRepository で弾かれ、結果ごとロールバックされる。
func TestAssessmentRepository_SubmitRejectsUnknownCategory(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `assessment_sessions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `assessment_results`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	err := repo.Submit(&models.AssessmentSession{ID: 3, UserID: 7}, map[string]float64{"やる気": 50}, "assessment:x", true)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 未受検なら nil（合流しない）。エラーにしない。
func TestAssessmentRepository_LatestStandardScoresNone(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectQuery("FROM assessment_results AS r JOIN assessment_sessions s .* JOIN assessment_templates t .* WHERE r.user_id = \\? AND t.is_standard = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	got, err := repo.LatestStandardScores(7)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestAssessmentRepository_LatestStandardScoresParsesJSON(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectQuery("FROM assessment_results AS r").
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id", "user_id", "category_scores"}).AddRow(1, 3, 7, `{"技術志向":62.5}`))
	got, err := repo.LatestStandardScores(7)
	require.NoError(t, err)
	require.Equal(t, 62.5, got["技術志向"])
}

// 「最新のチャット診断」を探す経路は検査の行を候補にしない（#1678）。
// 候補にすると、検査の直後にチャット完了扱い・マッチングの土台が検査に置き換わる。
func TestUserWeightScoreRepository_LatestQueriesExcludeAssessment(t *testing.T) {
	repo, mock := newWeightScoreBatchRepo(t)

	mock.ExpectQuery("WHERE user_id = \\? AND session_id NOT LIKE \\? AND source <> \\?").
		WithArgs(7, "interview-%", models.ScoreSourceAssessment, 1).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("session_chat"))
	got, err := repo.FindLatestDiagnosisSessionID(7)
	require.NoError(t, err)
	require.Equal(t, "session_chat", got)

	mock.ExpectQuery("WHERE user_id IN \\(\\?\\) AND source <> 'assessment'").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "weight_category", "score"}))
	_, err = repo.FindLatestScoresByUsers([]uint{7})
	require.NoError(t, err)

	mock.ExpectQuery("WHERE user_id = \\? AND source <> \\?").
		WithArgs(7, models.ScoreSourceAssessment, 1).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}))
	_, err = repo.FindLatestByUser(7)
	require.Error(t, err) // 行なし
	require.NoError(t, mock.ExpectationsWereMet())
}

// 学校独自テンプレートの提出は結果だけ残し、user_weight_scores へは何も書かない（#1675）。
// sqlmock は想定外の SQL（user_weight_scores への INSERT）が来ると失敗する。
func TestAssessmentRepository_SubmitWithoutMergeSkipsWeightScores(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `assessment_sessions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `assessment_results`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	s := &models.AssessmentSession{ID: 3, UserID: 7, Status: models.AssessmentStatusInProgress}
	require.NoError(t, repo.Submit(s, map[string]float64{"技術志向": 62.5}, "assessment:x", false))
	require.Equal(t, models.AssessmentStatusSubmitted, s.Status)
	require.NotNil(t, s.SubmittedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 学生に見せる検査は、運営標準 + 本人の学校だけ。学校未設定なら運営標準だけ。
func TestAssessmentRepository_ListTemplatesScopedBySchool(t *testing.T) {
	school := uint(10)
	tests := []struct {
		name     string
		schoolID *uint
		query    string
		args     []driver.Value
	}{
		{"学校あり", &school, "WHERE school_id IS NULL OR school_id = \\?", []driver.Value{10}},
		{"学校なし", nil, "WHERE school_id IS NULL ORDER BY", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newAssessmentRepo(t)
			exp := mock.ExpectQuery(tt.query)
			if tt.args != nil {
				exp.WithArgs(tt.args...)
			}
			exp.WillReturnRows(sqlmock.NewRows([]string{"id"}))
			_, err := repo.ListTemplates(tt.schoolID)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 他校の独自テンプレートは受検を始められない。条件に school_id が入っていることを固定する。
func TestAssessmentRepository_FindVisibleTemplateScopedBySchool(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectQuery("WHERE id = \\? AND \\(\\(school_id IS NULL OR school_id = \\?\\)\\)").
		WithArgs(5, 10, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	school := uint(10)
	_, err := repo.FindVisibleTemplate(5, &school)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 先生向けの学生一覧は学校 + 学園 + 学生ロール + 退会除外で絞る。
func TestAssessmentRepository_ListSchoolStudentsTenantConditions(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectQuery("FROM `users` WHERE school_id = \\? AND organization_id = \\? AND role = \\? AND withdrawn_at IS NULL").
		WithArgs(10, 3, models.UserRoleStudent).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "学生A"))
	got, err := repo.ListSchoolStudents(10, 3)
	require.NoError(t, err)
	require.Equal(t, []SchoolStudent{{ID: 1, Name: "学生A"}}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 学校独自テンプレートは is_standard=false で作り、問題を同じトランザクションで入れる。
func TestAssessmentRepository_CreateSchoolTemplateForcesNonStandard(t *testing.T) {
	repo, mock := newAssessmentRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `assessment_templates`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "s", "t", "", 1, false, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectExec("INSERT INTO `assessment_questions`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	sid, org := uint(10), uint(3)
	tpl := &models.AssessmentTemplate{SchoolID: &sid, OrganizationID: &org, Slug: "s", Title: "t", Version: 1, IsStandard: true}
	err := repo.CreateSchoolTemplate(tpl, []models.AssessmentQuestion{{Position: 1, Body: "q", WeightCategory: "技術志向"}})
	require.NoError(t, err)
	require.False(t, tpl.IsStandard, "呼び出し側が true を渡しても標準にはしない")
	require.NoError(t, mock.ExpectationsWereMet())
}

// FindUserSchoolID は users.school_id をそのまま信用しない（#1675）。
//
// users.school_id は本人が送ったプロフィールの学校名から更新され（auth_profile.go）、
// 解決に使う SchoolRepository.FindByName は学園で絞っていない。そのため別学園の学生でも
// 学校名を対象校と同じにすれば、その学校の school_id を自分に付けられる。
// school_id を返してしまうと、それだけで他学園の学校独自テンプレート（問題文・受検）に届く。
func TestAssessmentRepository_FindUserSchoolIDChecksOrganization(t *testing.T) {
	tests := []struct {
		name       string
		userOrg    uint
		schoolID   any // nil なら学校未設定
		schoolOrg  *uint
		wantSchool *uint
	}{
		{name: "学園が一致すれば所属校を返す", userOrg: 1, schoolID: int64(5), schoolOrg: ptrUint(1), wantSchool: ptrUint(5)},
		{name: "別学園の学校IDは返さない", userOrg: 2, schoolID: int64(5), schoolOrg: ptrUint(1), wantSchool: nil},
		{name: "学校未設定は運営標準だけ", userOrg: 1, schoolID: nil, schoolOrg: nil, wantSchool: nil},
		{name: "学校が存在しなければ運営標準だけ", userOrg: 1, schoolID: int64(5), schoolOrg: nil, wantSchool: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newAssessmentRepo(t)
			userRows := sqlmock.NewRows([]string{"school_id", "organization_id"}).AddRow(tt.schoolID, int64(tt.userOrg))
			mock.ExpectQuery("SELECT .*`users`").WillReturnRows(userRows)
			if tt.schoolID != nil {
				if tt.schoolOrg == nil {
					mock.ExpectQuery("SELECT .*`schools`").WillReturnError(gorm.ErrRecordNotFound)
				} else {
					mock.ExpectQuery("SELECT .*`schools`").
						WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id"}).AddRow(int64(5), int64(*tt.schoolOrg)))
				}
			}
			got, err := repo.FindUserSchoolID(9)
			require.NoError(t, err)
			if tt.wantSchool == nil {
				require.Nil(t, got, "他学園・未設定では school_id を返してはいけない")
			} else {
				require.NotNil(t, got)
				require.Equal(t, *tt.wantSchool, *got)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func ptrUint(v uint) *uint { return &v }
