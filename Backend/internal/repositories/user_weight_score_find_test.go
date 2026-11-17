package repositories_test

// UserWeightScoreRepository の Find 系が models.UserWeightScore をそのまま返すことの確認。
// 移行前は mapper で entity に変換していたので、行の値の取りこぼしと、0 件のときの形を押さえる。

import (
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

var scoreCols = []string{"id", "organization_id", "user_id", "session_id", "source", "weight_category", "score", "calculation_version", "prompt_version", "created_at", "updated_at"}

func TestFindByUserAndSession_行の値をそのまま返す(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT .* FROM `user_weight_scores` WHERE user_id = \\? AND session_id = \\?").
		WithArgs(uint(4), "s1").
		WillReturnRows(sqlmock.NewRows(scoreCols).
			AddRow(1, 2, 4, "s1", "assessment", "技術志向", 77, "v1", "p3", at, at.Add(time.Minute)))

	got, err := repo.FindByUserAndSession(4, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("件数: %d", len(got))
	}
	r := got[0]
	if r.ID != 1 || r.OrganizationID != 2 || r.UserID != 4 || r.SessionID != "s1" || r.Source != "assessment" ||
		r.WeightCategory != "技術志向" || r.Score != 77 || r.CalculationVersion != "v1" || r.PromptVersion != "p3" ||
		!r.CreatedAt.Equal(at) || !r.UpdatedAt.Equal(at.Add(time.Minute)) {
		t.Fatalf("値が欠けている: %+v", r)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 0 件でも nil にしない（応答では [] になる。nil だと null になって形が変わる）。
func TestFindByUserAndSession_0件は空のスライス(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `user_weight_scores`").WillReturnRows(sqlmock.NewRows(scoreCols))
	got, err := repo.FindByUserAndSession(4, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("0 件で nil を返した")
	}
}

func TestFindTopCategories_スコア降順で件数を絞る(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `user_weight_scores` WHERE user_id = \\? AND session_id = \\? ORDER BY score DESC LIMIT \\?").
		WithArgs(uint(4), "s1", 3).
		WillReturnRows(sqlmock.NewRows(scoreCols))
	got, err := repo.FindTopCategories(4, "s1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("0 件で nil を返した")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindByUserSessionAndCategory_見つからなければ未検出エラー(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `user_weight_scores` WHERE user_id = \\? AND session_id = \\? AND weight_category = \\?").
		WithArgs(uint(4), "s1", "技術志向", 1).
		WillReturnRows(sqlmock.NewRows(scoreCols))
	got, err := repo.FindByUserSessionAndCategory(4, "s1", "技術志向")
	if !errors.Is(err, gorm.ErrRecordNotFound) || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestFindByUserSessionAndCategory_行を返す(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `user_weight_scores` WHERE user_id = \\? AND session_id = \\? AND weight_category = \\?").
		WithArgs(uint(4), "s1", "技術志向", 1).
		WillReturnRows(sqlmock.NewRows(scoreCols).AddRow(9, 1, 4, "s1", "chat", "技術志向", 55, "v1", "", time.Now(), time.Now()))
	got, err := repo.FindByUserSessionAndCategory(4, "s1", "技術志向")
	if err != nil || got == nil || got.ID != 9 || got.Score != 55 || got.Source != "chat" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
