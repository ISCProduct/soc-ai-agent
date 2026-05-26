package integration_test

// 認証フローの統合テスト
// controller → service → repository の実コードを sqlmock DB で繋いで検証する
//
// 実行: cd Backend && go test ./test/integration/... -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/controllers"
	"Backend/internal/repositories"
	"Backend/internal/services"
)

func newIntegrationDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, mock
}

func newAuthController(t *testing.T, db *gorm.DB) *controllers.AuthController {
	t.Helper()
	userRepo := repositories.NewUserRepository(db)
	pendingRepo := repositories.NewPendingRegistrationRepository(db)
	emailService := services.NewEmailService() // SMTP未設定時はログ出力のみ
	authService := services.NewAuthService(userRepo, pendingRepo, emailService)
	return controllers.NewAuthController(authService)
}

// TestLogin_Integration は POST /api/auth/login の完全な統合フローを検証する
// controller → AuthService → UserRepository → sqlmock の全スタックを通る
func TestLogin_Integration(t *testing.T) {
	db, mock := newIntegrationDB(t)
	authController := newAuthController(t, db)

	// bcryptCost=12 で生成しないとログイン時に再ハッシュが走り余分なUPDATEが発生する
	hashedPw, err := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	if err != nil {
		t.Fatalf("bcryptハッシュ生成失敗: %v", err)
	}
	now := time.Now()

	userColumns := []string{
		"id", "email", "password", "name", "is_guest", "is_admin",
		"role", "target_level", "school_name",
		"oauth_provider", "oauth_id", "avatar_url",
		"certifications_acquired", "certifications_in_progress",
		"email_verified_at", "email_verification_token", "email_verification_expires",
		"last_login_at", "password_reset_token", "password_reset_expires_at",
		"allow_collective_insight", "created_at", "updated_at",
	}

	// GetUserByEmail: SELECT * FROM users WHERE email = ? LIMIT 1
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE").
		WithArgs("test@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
			1, "test@example.com", string(hashedPw), "テストユーザー", false, false,
			"student", "新卒", "テスト大学",
			"", "", "",
			"", "",
			now, "", nil,
			nil, "", nil,
			true, now, now,
		))

	// UpdateUser: LastLoginAt 更新
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)

	if handlerErr := authController.Login(c); handlerErr != nil {
		e.HTTPErrorHandler(handlerErr, c)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSONパース失敗: %v", err)
	}
	if resp["email"] != "test@example.com" {
		t.Errorf("email mismatch: got %v", resp["email"])
	}
	if resp["is_guest"] != false {
		t.Errorf("is_guest should be false, got %v", resp["is_guest"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock未処理の期待値あり: %v", err)
	}
}

// TestLogin_InvalidPassword_Integration は誤パスワードで 401 が返ることを検証する
func TestLogin_InvalidPassword_Integration(t *testing.T) {
	db, mock := newIntegrationDB(t)
	authController := newAuthController(t, db)

	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), 12)
	now := time.Now()

	userColumns := []string{
		"id", "email", "password", "name", "is_guest", "is_admin",
		"role", "target_level", "school_name",
		"oauth_provider", "oauth_id", "avatar_url",
		"certifications_acquired", "certifications_in_progress",
		"email_verified_at", "email_verification_token", "email_verification_expires",
		"last_login_at", "password_reset_token", "password_reset_expires_at",
		"allow_collective_insight", "created_at", "updated_at",
	}

	mock.ExpectQuery("SELECT \\* FROM `users` WHERE").
		WithArgs("test@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
			1, "test@example.com", string(hashedPw), "テストユーザー", false, false,
			"student", "新卒", "テスト大学",
			"", "", "",
			"", "",
			now, "", nil,
			nil, "", nil,
			true, now, now,
		))

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "wrong_password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)

	if handlerErr := authController.Login(c); handlerErr != nil {
		e.HTTPErrorHandler(handlerErr, c)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock未処理の期待値あり: %v", err)
	}
}

// TestCreateGuest_Integration は POST /api/auth/guest の完全な統合フローを検証する
func TestCreateGuest_Integration(t *testing.T) {
	db, mock := newIntegrationDB(t)
	authController := newAuthController(t, db)

	// CreateUser: INSERT INTO users
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `users`").WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectCommit()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest", nil)
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)

	if handlerErr := authController.CreateGuest(c); handlerErr != nil {
		e.HTTPErrorHandler(handlerErr, c)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSONパース失敗: %v", err)
	}
	if resp["is_guest"] != true {
		t.Errorf("is_guest should be true, got %v", resp["is_guest"])
	}
	if resp["email"] == nil || resp["email"] == "" {
		t.Errorf("email が空: %v", resp["email"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock未処理の期待値あり: %v", err)
	}
}

// TestLogin_UserNotFound_Integration はユーザーが存在しない場合に 401 が返ることを検証する
func TestLogin_UserNotFound_Integration(t *testing.T) {
	db, mock := newIntegrationDB(t)
	authController := newAuthController(t, db)

	// ユーザーが見つからない場合
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE").
		WithArgs("notfound@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	body, _ := json.Marshal(map[string]string{
		"email":    "notfound@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e := echo.New()
	c := e.NewContext(req, rec)

	if handlerErr := authController.Login(c); handlerErr != nil {
		e.HTTPErrorHandler(handlerErr, c)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock未処理の期待値あり: %v", err)
	}
}
