package repositories

import (
	"context"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRollupSQLKeepsRecomputeTotalsWithoutUser(t *testing.T) {
	for _, banned := range []string{"user_id", "latency_ms", "cache_hit"} {
		if strings.Contains(apiCallLogRollupInsert, banned) {
			t.Errorf("日次合計に %s がある", banned)
		}
	}
	for _, want := range []string{
		"audio_seconds", "characters", "web_search_calls", "price_basis",
		"prompt_stage", "prompt_version", "prompt_ab_group", "via_fallback",
		"call_count", "IFNULL(school_id, 0)",
	} {
		if !strings.Contains(apiCallLogRollupInsert, want) {
			t.Errorf("日次合計に %s が無い", want)
		}
	}
}

func TestRollupBatch_RollsUpThenDeletes(t *testing.T) {
	repo, mock, _ := newMockRepo(t)
	cutoff := time.Date(2024, 11, 19, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(apiCallLogRetentionLock).
		WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM api_call_logs").
		WithArgs(cutoff, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9).AddRow(10))
	mock.ExpectExec("INSERT INTO api_call_log_daily").
		WithArgs(uint(9), uint(10)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM api_call_logs").
		WithArgs(uint(9), uint(10)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(apiCallLogRetentionLock).
		WillReturnResult(sqlmock.NewResult(0, 0))

	n, ran, err := repo.RollupBatch(context.Background(), cutoff, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !ran || n != 2 {
		t.Fatalf("ran=%v deleted=%d", ran, n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRollupBatch_SkipsWhenLockIsHeld(t *testing.T) {
	repo, mock, _ := newMockRepo(t)
	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(apiCallLogRetentionLock).
		WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK"}).AddRow(0))

	n, ran, err := repo.RollupBatch(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if ran || n != 0 {
		t.Fatalf("ran=%v deleted=%d", ran, n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRollupBatch_NoRowsDoesNotInsert(t *testing.T) {
	repo, mock, _ := newMockRepo(t)
	cutoff := time.Date(2024, 11, 19, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(apiCallLogRetentionLock).
		WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM api_call_logs").
		WithArgs(cutoff, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(apiCallLogRetentionLock).
		WillReturnResult(sqlmock.NewResult(0, 0))

	n, ran, err := repo.RollupBatch(context.Background(), cutoff, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !ran || n != 0 {
		t.Fatalf("ran=%v deleted=%d", ran, n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
