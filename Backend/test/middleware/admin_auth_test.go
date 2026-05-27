package middleware_test

// 管理者認証ミドルウェアのセキュリティテスト
// 実行: cd Backend && go test ./test/middleware/... -run TestAdminAuth -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"Backend/internal/middleware"
)

// TestAdminAuth_MissingEmail は X-Admin-Email が欠落しているとき 401 を返すことを検証する
func TestAdminAuth_MissingEmail(t *testing.T) {
	repo, _ := newTestUserRepo(t)
	h := middleware.AdminAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("メール欠落: got %d, want 401", rr.Code)
	}
}

// TestAdminAuth_NonAdminUser は is_admin=false のユーザーを 403 で拒否することを検証する
func TestAdminAuth_NonAdminUser(t *testing.T) {
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("user@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(1, "user@example.com", "一般ユーザー", false, false))

	h := middleware.AdminAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", "user@example.com")
	req.Header.Set("X-Admin-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("非管理者ユーザー: got %d, want 403", rr.Code)
	}
}

// TestAdminAuth_SecretNotConfigured は ADMIN_SECRET 未設定のとき 503 を返すことを検証する（C-3修正の担保）
func TestAdminAuth_SecretNotConfigured(t *testing.T) {
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("admin@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(1, "admin@example.com", "管理者", true, false))

	h := middleware.AdminAuthFunc(repo, "", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", "admin@example.com")
	req.Header.Set("X-Admin-Token", "any-token")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("ADMIN_SECRET未設定はフェイルクローズ（503）であるべき: got %d, want 503", rr.Code)
	}
}

// TestAdminAuth_MissingToken は X-Admin-Token が欠落しているとき 403 を返すことを検証する
func TestAdminAuth_MissingToken(t *testing.T) {
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("admin@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(1, "admin@example.com", "管理者", true, false))

	h := middleware.AdminAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", "admin@example.com")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("トークン欠落: got %d, want 403", rr.Code)
	}
}

// TestAdminAuth_InvalidToken は誤ったトークンで 403 を返すことを検証する
func TestAdminAuth_InvalidToken(t *testing.T) {
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("admin@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(1, "admin@example.com", "管理者", true, false))

	h := middleware.AdminAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", "admin@example.com")
	req.Header.Set("X-Admin-Token", "wrong-token")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("不正トークン: got %d, want 403", rr.Code)
	}
}

// TestAdminAuth_ValidToken は正規の管理者トークンで次のハンドラが呼ばれることを検証する
func TestAdminAuth_ValidToken(t *testing.T) {
	const (
		secret  = "correct-secret"
		adminID = uint(1)
		email   = "admin@example.com"
	)
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(email, 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(adminID, email, "管理者", true, false))

	validToken := middleware.GenerateAdminToken(adminID, email, secret)
	h := middleware.AdminAuthFunc(repo, secret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", email)
	req.Header.Set("X-Admin-Token", validToken)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("正規管理者トークン: got %d, want 200", rr.Code)
	}
}

// TestAdminAuth_UserTokenCannotBeUsedAsAdminToken はユーザートークンを管理者認証に流用できないことを検証する
func TestAdminAuth_UserTokenCannotBeUsedAsAdminToken(t *testing.T) {
	const (
		secret  = "shared-secret"
		adminID = uint(1)
		email   = "admin@example.com"
	)
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(email, 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns).AddRow(adminID, email, "管理者", true, false))

	userToken := middleware.GenerateUserToken(adminID, email, secret)
	h := middleware.AdminAuthFunc(repo, secret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", email)
	req.Header.Set("X-Admin-Token", userToken)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("ユーザートークンを管理者認証に流用できてはならない: got %d, want 403", rr.Code)
	}
}

// TestAdminAuth_UserEmailNotFound はDBに存在しないメールで 403 を返すことを検証する
func TestAdminAuth_UserEmailNotFound(t *testing.T) {
	repo, mock := newTestUserRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("ghost@example.com", 1).
		WillReturnRows(sqlmock.NewRows(userRepoColumns))

	h := middleware.AdminAuthFunc(repo, "test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Admin-Email", "ghost@example.com")
	req.Header.Set("X-Admin-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("存在しないメール: got %d, want 403", rr.Code)
	}
}
