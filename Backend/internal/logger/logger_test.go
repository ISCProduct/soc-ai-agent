package logger_test

// 実行: cd Backend && go test ./internal/logger/... -v

import (
	"log/slog"
	"os"
	"testing"

	"Backend/internal/logger"
)

// TestSetup_DefaultIsInfo はLOG_LEVEL未設定時にInfoレベルになることを検証する
func TestSetup_DefaultIsInfo(t *testing.T) {
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LOG_FORMAT")
	logger.Setup()
	if !slog.Default().Enabled(nil, slog.LevelInfo) {
		t.Error("INFO レベルが有効になっていない")
	}
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("デフォルト設定では DEBUG レベルは無効であるべき")
	}
}

// TestSetup_DebugLevel はLOG_LEVEL=DEBUGでDebugレベルが有効になることを検証する
func TestSetup_DebugLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	os.Unsetenv("LOG_FORMAT")
	logger.Setup()
	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("DEBUG レベルが有効になっていない")
	}
}

// TestSetup_WarnLevel はLOG_LEVEL=WARNでInfoレベルが無効になることを検証する
func TestSetup_WarnLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "WARN")
	logger.Setup()
	if slog.Default().Enabled(nil, slog.LevelInfo) {
		t.Error("WARN 設定では INFO レベルは無効であるべき")
	}
	if !slog.Default().Enabled(nil, slog.LevelWarn) {
		t.Error("WARN レベルが有効になっていない")
	}
}

// TestSetup_DoesNotPanic はSetupがパニックしないことを検証する
func TestSetup_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Setup() がパニックした: %v", r)
		}
	}()
	logger.Setup()
}
