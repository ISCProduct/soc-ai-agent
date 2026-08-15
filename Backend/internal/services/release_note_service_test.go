package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/internal/openai"
	"Backend/internal/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newReleaseNoteTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

// newSummaryStubServer は ChatCompletionJSON の呼び出しに対して固定の summary を返すスタブ。
func newSummaryStubServer(t *testing.T, summary string) (*httptest.Server, *openai.Client) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/chat/completions") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body := map[string]any{
			"title":   "テストタイトル",
			"summary": summary,
		}
		raw, _ := json.Marshal(body)
		resp := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": string(raw)},
			}},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
	return server, client
}

func TestReleaseNoteService_IngestMergedPRs_SkipsExisting(t *testing.T) {
	db, mock := newReleaseNoteTestDB(t)
	server, client := newSummaryStubServer(t, "")
	defer server.Close()
	svc := services.NewReleaseNoteService(db, client)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `release_notes` WHERE pr_number = \\?").
		WithArgs(uint(100)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	saved, err := svc.IngestMergedPRs(context.Background(), []services.ReleaseNoteSource{
		{PRNumber: 100, Title: "既存PR", Body: "本文", MergedAt: time.Now()},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved != 0 {
		t.Fatalf("expected 0 saved, got %d", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestReleaseNoteService_IngestMergedPRs_SkipsEmptySummary(t *testing.T) {
	db, mock := newReleaseNoteTestDB(t)
	server, client := newSummaryStubServer(t, "")
	defer server.Close()
	svc := services.NewReleaseNoteService(db, client)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `release_notes` WHERE pr_number = \\?").
		WithArgs(uint(200)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	saved, err := svc.IngestMergedPRs(context.Background(), []services.ReleaseNoteSource{
		{PRNumber: 200, Title: "内部リファクタリング", Body: "ユーザーに影響なし", MergedAt: time.Now()},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved != 0 {
		t.Fatalf("expected 0 saved (empty summary), got %d", saved)
	}
}

func TestReleaseNoteService_IngestMergedPRs_SavesNewEntry(t *testing.T) {
	db, mock := newReleaseNoteTestDB(t)
	server, client := newSummaryStubServer(t, "新機能を追加しました。")
	defer server.Close()
	svc := services.NewReleaseNoteService(db, client)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `release_notes` WHERE pr_number = \\?").
		WithArgs(uint(300)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `release_notes`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	saved, err := svc.IngestMergedPRs(context.Background(), []services.ReleaseNoteSource{
		{PRNumber: 300, Title: "新機能", Body: "ユーザー向けの本文", MergedAt: time.Now()},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved != 1 {
		t.Fatalf("expected 1 saved, got %d", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestReleaseNoteService_IngestMergedPRs_NilClientErrors(t *testing.T) {
	db, _ := newReleaseNoteTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)

	_, err := svc.IngestMergedPRs(context.Background(), []services.ReleaseNoteSource{{PRNumber: 1}})
	if err != services.ErrReleaseNoteLLMClientNil {
		t.Fatalf("expected ErrReleaseNoteLLMClientNil, got %v", err)
	}
}

func TestReleaseNoteService_List_OrdersByMergedAtDesc(t *testing.T) {
	db, mock := newReleaseNoteTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)

	rows := sqlmock.NewRows([]string{"id", "pr_number", "title", "summary", "merged_at", "created_at"}).
		AddRow(2, 200, "新しい方", "説明2", time.Now(), time.Now()).
		AddRow(1, 100, "古い方", "説明1", time.Now().Add(-24*time.Hour), time.Now())
	mock.ExpectQuery("SELECT \\* FROM `release_notes` ORDER BY merged_at DESC LIMIT \\?").
		WithArgs(20).
		WillReturnRows(rows)

	notes, err := svc.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 2 || notes[0].Title != "新しい方" {
		t.Fatalf("unexpected result: %+v", notes)
	}
}
