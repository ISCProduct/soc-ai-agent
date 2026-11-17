package repositories

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"Backend/internal/models"
)

// 分析フェーズ・進捗の Repository が、models をそのまま返す・保存することを確かめる。
// 以前は entity との変換(mapper)を挟んでいたので、その性質(Phase を書き込まない、
// 全項目を返す)をここで固定する。

var (
	phaseCols    = []string{"id", "phase_name", "display_name", "phase_order", "description", "min_questions", "max_questions", "created_at", "updated_at"}
	progressCols = []string{"id", "user_id", "session_id", "phase_id", "questions_asked", "valid_answers", "invalid_answers", "completion_score", "is_completed", "completed_at", "created_at", "updated_at"}
)

func TestAnalysisPhaseFindAll_順序順に全項目を返す(t *testing.T) {
	db, mock := newExpiredDeleteTestDB(t)
	now := time.Now().Truncate(time.Second)
	mock.ExpectQuery("SELECT \\* FROM `analysis_phases` ORDER BY phase_order ASC").
		WillReturnRows(sqlmock.NewRows(phaseCols).
			AddRow(1, "job_analysis", "職種", 1, "説明", 3, 5, now, now).
			AddRow(2, "interest_analysis", "興味", 2, "", 2, 0, now, now))

	got, err := NewAnalysisPhaseRepository(db).FindAll()
	if err != nil || len(got) != 2 {
		t.Fatalf("FindAll: %v %v", got, err)
	}
	if got[0].ID != 1 || got[0].PhaseName != "job_analysis" || got[0].DisplayName != "職種" || got[0].PhaseOrder != 1 ||
		got[0].Description != "説明" || got[0].MinQuestions != 3 || got[0].MaxQuestions != 5 || got[1].PhaseName != "interest_analysis" {
		t.Fatalf("フィールドが落ちている: %+v", got)
	}
}

func TestAnalysisPhaseFindByName_名前で引く_未発見はエラー(t *testing.T) {
	db, mock := newExpiredDeleteTestDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `analysis_phases` WHERE phase_name = \\?").
		WithArgs("job_analysis", 1).
		WillReturnRows(sqlmock.NewRows(phaseCols).AddRow(1, "job_analysis", "職種", 1, "", 3, 5, now, now))
	mock.ExpectQuery("SELECT \\* FROM `analysis_phases` WHERE phase_name = \\?").
		WithArgs("none", 1).
		WillReturnRows(sqlmock.NewRows(phaseCols))

	repo := NewAnalysisPhaseRepository(db)
	if p, err := repo.FindByName("job_analysis"); err != nil || p == nil || p.ID != 1 {
		t.Fatalf("FindByName: %v %v", p, err)
	}
	if p, err := repo.FindByName("none"); err == nil || p != nil {
		t.Fatalf("未発見はエラーのはず: %v %v", p, err)
	}
}

func TestUserAnalysisProgressFindByUserAndSession_Phaseを読み込んで返す(t *testing.T) {
	db, mock := newExpiredDeleteTestDB(t)
	now := time.Now().Truncate(time.Second)
	mock.ExpectQuery("SELECT \\* FROM `user_analysis_progress` WHERE user_id = \\? AND session_id = \\? ORDER BY phase_id ASC").
		WithArgs(5, "s1").
		WillReturnRows(sqlmock.NewRows(progressCols).
			AddRow(10, 5, "s1", 1, 4, 3, 1, 60.0, false, nil, now, now))
	mock.ExpectQuery("SELECT \\* FROM `analysis_phases` WHERE `analysis_phases`.`id` = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(phaseCols).AddRow(1, "job_analysis", "職種", 1, "", 3, 5, now, now))

	got, err := NewUserAnalysisProgressRepository(db).FindByUserAndSession(5, "s1")
	if err != nil || len(got) != 1 {
		t.Fatalf("取得失敗: %v %v", got, err)
	}
	p := got[0]
	if p.ID != 10 || p.UserID != 5 || p.SessionID != "s1" || p.PhaseID != 1 || p.QuestionsAsked != 4 ||
		p.ValidAnswers != 3 || p.InvalidAnswers != 1 || p.CompletionScore != 60 || p.IsCompleted {
		t.Fatalf("フィールドが落ちている: %+v", p)
	}
	if p.Phase == nil || p.Phase.PhaseName != "job_analysis" {
		t.Fatalf("Phase が読み込まれていない: %+v", p.Phase)
	}
}

// Update は進捗の列だけを更新し、Preload 済みの Phase を書き込まない。
func TestUserAnalysisProgressUpdate_Phaseを書き込まない(t *testing.T) {
	db, mock := newExpiredDeleteTestDB(t)
	now := time.Now()
	done := now.Add(time.Minute)
	mock.ExpectBegin()
	// Phase の INSERT(upsert)が来たら、この Exec の照合に合わず失敗する。
	mock.ExpectExec("UPDATE `user_analysis_progress` SET `user_id`=\\?,`session_id`=\\?,`phase_id`=\\?,`questions_asked`=\\?,`valid_answers`=\\?,`invalid_answers`=\\?,`completion_score`=\\?,`is_completed`=\\?,`completed_at`=\\?,`created_at`=\\?,`updated_at`=\\? WHERE `id` = \\?").
		WithArgs(5, "s1", 1, 5, 4, 1, 80.0, true, done, now, sqlmock.AnyArg(), 10).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := NewUserAnalysisProgressRepository(db).Update(&models.UserAnalysisProgress{
		ID: 10, UserID: 5, SessionID: "s1", PhaseID: 1,
		Phase:          &models.AnalysisPhase{ID: 1, PhaseName: "job_analysis", DisplayName: "職種", MinQuestions: 3, MaxQuestions: 5},
		QuestionsAsked: 5, ValidAnswers: 4, InvalidAnswers: 1, CompletionScore: 80,
		IsCompleted: true, CompletedAt: &done, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Update失敗: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
