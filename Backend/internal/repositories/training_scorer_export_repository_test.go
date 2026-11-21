package repositories

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"Backend/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trainingGuards は「学習に使ってよい学生」の条件（#2168 追記2）。どれか1つでも欠けたら、
// 同意していない・退会した（フラグは 1 のまま）・学生でない・ゲストの面接が書き出される。
// 引数（true, "student", false）も WithArgs で固定し、条件の値の取り違えでも落ちるようにする。
var trainingGuards = regexp.QuoteMeta("u.allow_training_use = ? AND u.withdrawn_at IS NULL AND u.role = ? AND u.is_guest = ?")

var trainingGuardArgs = []driver.Value{true, models.UserRoleStudent, false}

func withGuardArgs(extra ...driver.Value) []driver.Value {
	return append(append([]driver.Value{}, trainingGuardArgs...), extra...)
}

func TestTrainingScorerExportRepository_IsTrainingEligible(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  bool
	}{
		{name: "条件に合う学生", count: 1, want: true},
		{name: "条件に合わない学生（同意なし・退会など）", count: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newSearchMockDB(t)
			mock.ExpectQuery(`SELECT count\(\*\) FROM users AS u WHERE ` + trainingGuards + regexp.QuoteMeta(" AND u.id = ?")).
				WithArgs(withGuardArgs(7)...).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.count))

			got, err := NewTrainingScorerExportRepository(db).IsTrainingEligible(context.Background(), 7)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTrainingScorerExportRepository_IsTrainingEligible_Error(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(trainingGuards).WillReturnError(errors.New("db down"))
	got, err := NewTrainingScorerExportRepository(db).IsTrainingEligible(context.Background(), 7)
	require.Error(t, err)
	assert.False(t, got, "エラーのときに「使ってよい」を返さない")
}

func TestTrainingScorerExportRepository_ListTrainingSubjects(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(`SELECT u\.id AS user_id, .* FROM users AS u WHERE ` + trainingGuards + ` ORDER BY u\.id`).
		WithArgs(trainingGuardArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "email", "school_name"}).
			AddRow(3, "山田 太郎", "taro@example.com", "東京情報専門学校").
			AddRow(5, "", "b@example.com", ""))

	got, err := NewTrainingScorerExportRepository(db).ListTrainingSubjects(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []TrainingSubject{
		{UserID: 3, Name: "山田 太郎", Email: "taro@example.com", SchoolName: "東京情報専門学校"},
		{UserID: 5, Name: "", Email: "b@example.com", SchoolName: ""},
	}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestTrainingScorerExportRepository_ListTrainingSessions は、面接の取得にも学生の条件が副問い合わせで付き、
// 完了した・削除されていない・採点のある面接だけを取ることを確かめる。
func TestTrainingScorerExportRepository_ListTrainingSessions(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(`FROM interview_sessions AS s JOIN interview_reports AS ir ON ir\.session_id = s\.id ` +
		regexp.QuoteMeta("WHERE s.user_id = ? AND s.user_id IN (SELECT u.id FROM users AS u WHERE ") + trainingGuards + `\) ` +
		regexp.QuoteMeta("AND (s.status = ? AND s.deleted_at IS NULL) AND (ir.scores_json IS NOT NULL AND ir.scores_json <> '') ORDER BY s.id")).
		WithArgs(append([]driver.Value{uint(7)}, append(trainingGuardArgs, "finished")...)...).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "company_name", "template_version", "scores_json"}).
			AddRow(100, "株式会社サンプル", "v1", `{"logic":3}`))

	got, err := NewTrainingScorerExportRepository(db).ListTrainingSessions(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, []TrainingSession{{SessionID: 100, CompanyName: "株式会社サンプル", TemplateVersion: "v1", ScoresJSON: `{"logic":3}`}}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrainingScorerExportRepository_ListTrainingUtterances(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT session_id, role, text FROM `interview_utterances` WHERE session_id = ? ORDER BY created_at, id")).
		WithArgs(uint(100)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "role", "text"}).
			AddRow(100, "ai", "質問").AddRow(100, "user", "回答"))

	got, err := NewTrainingScorerExportRepository(db).ListTrainingUtterances(context.Background(), 100)
	require.NoError(t, err)
	assert.Equal(t, []TrainingUtterance{{SessionID: 100, Role: "ai", Text: "質問"}, {SessionID: 100, Role: "user", Text: "回答"}}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrainingScorerExportRepository_ListTrainingMaskingTerms(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(`SELECT name FROM schools .* UNION SELECT DISTINCT school_name FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("東京情報専門学校").AddRow("横浜デジタル専門学校"))
	mock.ExpectQuery(`SELECT name FROM companies .* UNION SELECT name_reading FROM companies .* UNION SELECT DISTINCT company_name FROM interview_sessions`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("株式会社サンプル").AddRow("サンプル"))

	got, err := NewTrainingScorerExportRepository(db).ListTrainingMaskingTerms(context.Background())
	require.NoError(t, err)
	assert.Equal(t, TrainingMaskingTerms{
		Schools:   []string{"東京情報専門学校", "横浜デジタル専門学校"},
		Companies: []string{"株式会社サンプル", "サンプル"},
	}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrainingScorerExportRepository_ListTrainingMaskingTerms_Error(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(`FROM schools`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("東京情報専門学校"))
	mock.ExpectQuery(`FROM companies`).WillReturnError(errors.New("db down"))

	got, err := NewTrainingScorerExportRepository(db).ListTrainingMaskingTerms(context.Background())
	require.Error(t, err)
	assert.Equal(t, TrainingMaskingTerms{}, got, "途中で失敗したら、取れた分だけの辞書を返さない")
}

func TestTrainingScorerExportRepository_StartAndFinishRun(t *testing.T) {
	db, mock := newSearchMockDB(t)
	started := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	finished := started.Add(time.Minute)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `training_scorer_export_runs`").
		WithArgs(models.TrainingScorerRunKindExport, models.TrainingScorerExportRunning, started, nil, 0, 0, 0, "s3://b/p/runs/x/", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `training_scorer_export_runs` SET `error_message`=?,`finished_at`=?,`session_count`=?,`status`=?,`student_count`=?,`updated_at`=? WHERE id = ?")).
		WithArgs("s3 put: denied", finished, 4, models.TrainingScorerExportFailed, 2, sqlmock.AnyArg(), uint(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewTrainingScorerExportRepository(db)
	id, err := repo.StartTrainingExportRun(context.Background(), "s3://b/p/runs/x/", started)
	require.NoError(t, err)
	assert.Equal(t, uint(9), id)
	require.NoError(t, repo.FinishTrainingExportRun(context.Background(), id, models.TrainingScorerExportFailed, 2, 4, "s3 put: denied", finished))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrainingScorerExportRepository_StartRun_Error(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `training_scorer_export_runs`").WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	id, err := NewTrainingScorerExportRepository(db).StartTrainingExportRun(context.Background(), "s3://b/p/", time.Now())
	require.Error(t, err)
	assert.Zero(t, id)
}

// TestTrainingScorerExportRepository_ListTrainingEligibleUserIDs は、削除（Sweep）が「残す学生」の一覧に使う ID が、
// 「学習に使ってよい学生」の4つの条件をすべて通ることを確かめる。条件が欠けると、取り消した・退会した学生の
// ファイルを残してしまう。
func TestTrainingScorerExportRepository_ListTrainingEligibleUserIDs(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(trainingGuards + ".*ORDER BY u.id").
		WithArgs(trainingGuardArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3).AddRow(8))

	ids, err := NewTrainingScorerExportRepository(db).ListTrainingEligibleUserIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uint{3, 8}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrainingScorerExportRepository_ListTrainingEligibleUserIDs_Error(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectQuery(trainingGuards).WillReturnError(errors.New("db down"))

	ids, err := NewTrainingScorerExportRepository(db).ListTrainingEligibleUserIDs(context.Background())
	require.Error(t, err)
	assert.Empty(t, ids)
}

// TestTrainingScorerExportRepository_SweepRun は、削除の監査ログを kind=sweep で作り、消したファイルの数を
// sweep の行にだけ書くことを確かめる（書き出しの行を書き換えない）。
func TestTrainingScorerExportRepository_SweepRun(t *testing.T) {
	db, mock := newSearchMockDB(t)
	started := time.Date(2026, 11, 17, 7, 0, 0, 0, time.UTC)
	finished := started.Add(time.Minute)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `training_scorer_export_runs`").
		WithArgs(models.TrainingScorerRunKindSweep, models.TrainingScorerExportRunning, started, nil, 0, 0, 0, "s3://b/p/runs/", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `training_scorer_export_runs` SET `deleted_file_count`=?,`error_message`=?,`finished_at`=?,`status`=?,`updated_at`=? WHERE id = ? AND kind = ?")).
		WithArgs(3, "", finished, models.TrainingScorerExportSucceeded, sqlmock.AnyArg(), uint(11), models.TrainingScorerRunKindSweep).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewTrainingScorerExportRepository(db)
	id, err := repo.StartTrainingSweepRun(context.Background(), "s3://b/p/runs/", started)
	require.NoError(t, err)
	assert.Equal(t, uint(11), id)
	require.NoError(t, repo.FinishTrainingSweepRun(context.Background(), id, models.TrainingScorerExportSucceeded, 3, "", finished))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrainingScorerExportRepository_StartSweepRun_Error(t *testing.T) {
	db, mock := newSearchMockDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `training_scorer_export_runs`").WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	id, err := NewTrainingScorerExportRepository(db).StartTrainingSweepRun(context.Background(), "s3://b/p/runs/", time.Now())
	require.Error(t, err)
	assert.Zero(t, id)
}
