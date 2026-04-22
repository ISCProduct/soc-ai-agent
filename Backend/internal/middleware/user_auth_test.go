package middleware_test

// ユーザー認証ミドルウェアのセキュリティテスト
// 実行: cd Backend && go test ./internal/middleware/... -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/middleware"
	"Backend/internal/repositories"
)

// userRepoColumns は sqlmock が返すカラム一覧（GORM が要求する最低限）
var userRepoColumns = []string{"id", "email", "name", "is_admin", "is_guest"}

func newTestUserRepo(t *testing.T) (*repositories.UserRepository, sqlmock.Sqlmock) {
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
	return repositories.NewUserRepository(db), mock
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// TestUserAuth_SecretNotConfigured は userSecret が空のとき 503 を返すことを検証する（フェイルクローズ）
func TestUserAuth_SecretNotConfigured(t *testing.T) {
	repo, _ := newTestUserRepo(t)
	h := middleware.UserAuthFunc(repo, "", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("シークレット未設定: got %d, want 503", rr.Code)
	}
}

// TestUserAuth_MissingHeaders はヘッダーが欠けているとき 401 を返すことを検証する
func TestUserAuth_MissingHeaders(t *testing.T) {
	secret := "test-secret"
	repo, _ := newTestUserRepo(t)
	h := middleware.UserAuthFunc(repo, secret, okHandler)

	tests := []struct {
		name    string
		userID  string
		token   string
		wantCode int
	}{
		{"X-User-ID のみ欠落", "", "sometoken", http.StatusUnauthorized},
		{"X-User-Token のみ欠落", "1", "", http.StatusUnauthorized},
		{"両ヘッダー欠落", "", "", http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.userID != "" {
				req.Header.Set("X-User-ID", tc.userID)
			}
			if tc.token != "" {
				req.Header.Set("X-User-Token", tc.token)
			}
			rr := httptest.NewRecorder()
			h(rr, req)
			if rr.Code != tc.wantCode {
				t.Errorf("%s: got %d, want %d", tc.name, rr.Code, tc.wantCode)
			}
		})
	}
}

// TestUserAuth_InvalidUserID は X-User-ID が数値でないとき 401 を返すことを検証する
func TestUserAuth_InvalidUserID(t *testing.T) {
	repo, _ := newTestUserRepo(t)
	h := middleware.UserAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "notanumber")
	req.Header.Set("X-User-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("不正なユーザーID: got %d, want 401", rr.Code)
	}
}

// TestUserAuth_UserNotFound はDBにユーザーが存在しないとき 401 を返すことを検証する
func TestUserAuth_UserNotFound(t *testing.T) {
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(99), 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns))

	h := middleware.UserAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "99")
	req.Header.Set("X-User-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ユーザー未存在: got %d, want 401", rr.Code)
	}
}

// TestUserAuth_InvalidToken は正しいユーザーIDで誤ったトークンのとき 403 を返すことを検証する
func TestUserAuth_InvalidToken(t *testing.T) {
	secret := "correct-secret"
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(1, "user@example.com", "テストユーザー", false, false))

	h := middleware.UserAuthFunc(repo, secret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Token", "wrong-token")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("不正トークン: got %d, want 403", rr.Code)
	}
}

// TestUserAuth_ValidToken は正しいヘッダーで次のハンドラが呼ばれることを検証する
func TestUserAuth_ValidToken(t *testing.T) {
	const (
		secret = "correct-secret"
		userID = uint(1)
		email  = "user@example.com"
	)
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(userID, email, "テストユーザー", false, false))

	validToken := middleware.GenerateUserToken(userID, email, secret)
	h := middleware.UserAuthFunc(repo, secret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Token", validToken)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("正規トークン: got %d, want 200", rr.Code)
	}
}

// TestUserAuth_AdminTokenCannotBeUsedAsUserToken は管理者トークンをユーザートークンとして流用できないことを検証する（C-4修正の担保）
func TestUserAuth_AdminTokenCannotBeUsedAsUserToken(t *testing.T) {
	const (
		secret = "shared-secret"
		userID = uint(1)
		email  = "admin@example.com"
	)
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(userID, email, "管理者", true, false))

	// 管理者トークン（ペイロード: "{id}:{email}"）をユーザー認証に使う
	adminToken := middleware.GenerateAdminToken(userID, email, secret)
	h := middleware.UserAuthFunc(repo, secret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Token", adminToken)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("管理者トークンをユーザー認証に流用できてはならない: got %d, want 403", rr.Code)
	}
}

// TestGenerateUserToken_Deterministic は同一入力で同一トークンを生成することを検証する
func TestGenerateUserToken_Deterministic(t *testing.T) {
	tok1 := middleware.GenerateUserToken(1, "user@example.com", "secret")
	tok2 := middleware.GenerateUserToken(1, "user@example.com", "secret")
	if tok1 != tok2 {
		t.Error("同一入力で異なるトークンが生成された")
	}
}

// TestGenerateUserToken_DifferentFromAdminToken はユーザートークンと管理者トークンが異なることを検証する
func TestGenerateUserToken_DifferentFromAdminToken(t *testing.T) {
	const (
		userID = uint(1)
		email  = "user@example.com"
		secret = "secret"
	)
	userToken := middleware.GenerateUserToken(userID, email, secret)
	adminToken := middleware.GenerateAdminToken(userID, email, secret)
	if userToken == adminToken {
		t.Error("ユーザートークンと管理者トークンが同一になってはならない（トークン流用防止）")
	}
}

// TestVerifyUserToken_WrongUserID は別ユーザーIDのトークンを拒否することを検証する
func TestVerifyUserToken_WrongUserID(t *testing.T) {
	token := middleware.GenerateUserToken(1, "user@example.com", "secret")
	// 異なるユーザーIDで検証
	if middleware.VerifyUserToken(token, 2, "user@example.com", "secret") {
		t.Error("別ユーザーIDのトークンを受け入れてはならない")
	}
}

// TestVerifyUserToken_WrongEmail は別メールのトークンを拒否することを検証する
func TestVerifyUserToken_WrongEmail(t *testing.T) {
	token := middleware.GenerateUserToken(1, "user@example.com", "secret")
	if middleware.VerifyUserToken(token, 1, "attacker@example.com", "secret") {
		t.Error("別メールアドレスのトークンを受け入れてはならない")
	}
}

// TestVerifyUserToken_WrongSecret は別シークレットのトークンを拒否することを検証する
func TestVerifyUserToken_WrongSecret(t *testing.T) {
	token := middleware.GenerateUserToken(1, "user@example.com", "correct-secret")
	if middleware.VerifyUserToken(token, 1, "user@example.com", "wrong-secret") {
		t.Error("別シークレットのトークンを受け入れてはならない")
	}
}
