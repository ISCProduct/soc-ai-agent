package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newReadyTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// gorm.Open が接続確認で 1 回 ping する。
	mock.ExpectPing()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func getJSONStatus(t *testing.T, e http.Handler, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return rec.Code, body["status"]
}

func TestHealthDoesNotTouchDBAndReadyPings(t *testing.T) {
	db, mock := newReadyTestDB(t)
	mock.ExpectPing()
	e := newEchoServer(false, nil, db)

	code, status := getJSONStatus(t, e, "/health")
	if code != http.StatusOK || status != "ok" {
		t.Fatalf("/health = %d %s", code, status)
	}
	code, status = getJSONStatus(t, e, "/healthz")
	if code != http.StatusOK || status != "ok" {
		t.Fatalf("/healthz = %d %s", code, status)
	}
	code, status = getJSONStatus(t, e, "/ready")
	if code != http.StatusOK || status != "ok" {
		t.Fatalf("/ready = %d %s", code, status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReadyUnavailableWhenPingFails(t *testing.T) {
	db, mock := newReadyTestDB(t)
	mock.ExpectPing().WillReturnError(errors.New("db down"))
	e := newEchoServer(false, nil, db)

	code, status := getJSONStatus(t, e, "/ready")
	if code != http.StatusServiceUnavailable || status != "unavailable" {
		t.Fatalf("/ready = %d %s", code, status)
	}
}

func TestReadyUnavailableWithoutDB(t *testing.T) {
	e := newEchoServer(false, nil, nil)
	code, status := getJSONStatus(t, e, "/ready")
	if code != http.StatusServiceUnavailable || status != "unavailable" {
		t.Fatalf("/ready = %d %s", code, status)
	}
	code, status = getJSONStatus(t, e, "/health")
	if code != http.StatusOK || status != "ok" {
		t.Fatalf("/health = %d %s", code, status)
	}
}
