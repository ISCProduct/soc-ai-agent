package routes_test

// EchoAdminAuth（管理APIの実経路の認証）のテスト。
//
// 未使用だった middleware.AdminAuth を削除した際（#1750）、そのテストを
// 実際に使われている EchoAdminAuth へ移した。判定は is_admin ではなく
// CanAccessAdminArea（管理者または職員）なので、職員が通ることと、
// 後段へ渡す principal（ユーザーID・管理者かどうか）も固定する。
// 実行: cd Backend && go test ./internal/routes/ -run TestEchoAdminAuth -v

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"Backend/internal/routes"
)

// adminAuthUserColumns は sqlmock が返すカラム一覧。職員判定のため role を含める。
var adminAuthUserColumns = []string{"id", "email", "name", "is_admin", "is_guest", "role"}

const selectUserByEmail = "SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?"

func newAdminAuthUserRepo(t *testing.T) (*repositories.UserRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return repositories.NewUserRepository(db), mock
}

// expectAdminAuthUser は GetUserByEmail が1行返すことを期待させる。
func expectAdminAuthUser(mock sqlmock.Sqlmock, id uint, email string, isAdmin bool, role string) {
	mock.ExpectQuery(selectUserByEmail).
		WithArgs(email, 1).
		WillReturnRows(sqlmock.NewRows(adminAuthUserColumns).AddRow(id, email, "テストユーザー", isAdmin, false, role))
}

type adminAuthResult struct {
	rec        *httptest.ResponseRecorder
	reached    bool
	userID     any
	isPlatform any
}

// serveAdminAuth は EchoAdminAuth を通したリクエストを1回処理し、後段に届いたかと principal を返す。
func serveAdminAuth(repo *repositories.UserRepository, secret, email, token string) adminAuthResult {
	var res adminAuthResult
	e := echo.New()
	e.GET("/x", func(c echo.Context) error {
		res.reached = true
		res.userID = c.Request().Context().Value(middleware.AdminUserIDContextKey)
		res.isPlatform = c.Request().Context().Value(middleware.AdminIsPlatformContextKey)
		return c.String(http.StatusOK, "ok")
	}, routes.EchoAdminAuth(repo, secret))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if email != "" {
		req.Header.Set("X-Admin-Email", email)
	}
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	res.rec = httptest.NewRecorder()
	e.ServeHTTP(res.rec, req)
	return res
}

// X-Admin-Email が欠落しているとき 401 を返す。
func TestEchoAdminAuth_MissingEmail(t *testing.T) {
	repo, _ := newAdminAuthUserRepo(t)
	res := serveAdminAuth(repo, "test-secret", "", "sometoken")
	if res.rec.Code != http.StatusUnauthorized || res.reached {
		t.Errorf("メール欠落: status=%d reached=%v, want 401 / false", res.rec.Code, res.reached)
	}
}

// 管理者でも職員でもないユーザー（学生）は 403 で拒否する。
func TestEchoAdminAuth_StudentForbidden(t *testing.T) {
	const email = "student@example.com"
	repo, mock := newAdminAuthUserRepo(t)
	expectAdminAuthUser(mock, 1, email, false, "student")
	res := serveAdminAuth(repo, "test-secret", email, middleware.GenerateAdminToken(1, email, "test-secret"))
	if res.rec.Code != http.StatusForbidden || res.reached {
		t.Errorf("学生: status=%d reached=%v, want 403 / false", res.rec.Code, res.reached)
	}
}

// 管理者と職員は正規の管理者トークンで通り、後段に principal が渡る。
// 職員（is_admin=false）を通すのが削除した middleware.AdminAuth との違い（#1750）。
func TestEchoAdminAuth_AdminAndStaffAllowed(t *testing.T) {
	const secret = "correct-secret"
	for _, tc := range []struct {
		name    string
		id      uint
		email   string
		isAdmin bool
		role    string
	}{
		{"管理者", 1, "admin@example.com", true, "student"},
		{"職員", 2, "staff@example.com", false, "staff"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newAdminAuthUserRepo(t)
			expectAdminAuthUser(mock, tc.id, tc.email, tc.isAdmin, tc.role)
			res := serveAdminAuth(repo, secret, tc.email, middleware.GenerateAdminToken(tc.id, tc.email, secret))
			if res.rec.Code != http.StatusOK || !res.reached {
				t.Fatalf("status=%d reached=%v, want 200 / true（body=%q）", res.rec.Code, res.reached, res.rec.Body.String())
			}
			if res.userID != tc.id {
				t.Errorf("AdminUserIDContextKey = %v, want %d", res.userID, tc.id)
			}
			if res.isPlatform != tc.isAdmin {
				t.Errorf("AdminIsPlatformContextKey = %v, want %v（職員を全校扱いにしない）", res.isPlatform, tc.isAdmin)
			}
		})
	}
}

// ADMIN_SECRET 未設定のときはフェイルクローズで 503 を返す。
func TestEchoAdminAuth_SecretNotConfigured(t *testing.T) {
	const email = "admin@example.com"
	repo, mock := newAdminAuthUserRepo(t)
	expectAdminAuthUser(mock, 1, email, true, "")
	res := serveAdminAuth(repo, "", email, "any-token")
	if res.rec.Code != http.StatusServiceUnavailable || res.reached {
		t.Errorf("ADMIN_SECRET未設定: status=%d reached=%v, want 503 / false", res.rec.Code, res.reached)
	}
}

// トークンの欠落・不正・ユーザートークンの流用は 403 で拒否する。
func TestEchoAdminAuth_RejectsBadToken(t *testing.T) {
	const (
		secret = "test-secret"
		email  = "admin@example.com"
	)
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"トークン欠落", ""},
		{"不正なトークン", "wrong-token"},
		{"ユーザートークンの流用", middleware.GenerateUserToken(1, email, secret)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newAdminAuthUserRepo(t)
			expectAdminAuthUser(mock, 1, email, true, "")
			res := serveAdminAuth(repo, secret, email, tc.token)
			if res.rec.Code != http.StatusForbidden || res.reached {
				t.Errorf("status=%d reached=%v, want 403 / false", res.rec.Code, res.reached)
			}
		})
	}
}

// DB に存在しないメールは 403 で拒否する。
func TestEchoAdminAuth_UserEmailNotFound(t *testing.T) {
	repo, mock := newAdminAuthUserRepo(t)
	mock.ExpectQuery(selectUserByEmail).
		WithArgs("ghost@example.com", 1).
		WillReturnRows(sqlmock.NewRows(adminAuthUserColumns))
	res := serveAdminAuth(repo, "test-secret", "ghost@example.com", "sometoken")
	if res.rec.Code != http.StatusForbidden || res.reached {
		t.Errorf("存在しないメール: status=%d reached=%v, want 403 / false", res.rec.Code, res.reached)
	}
}

// legacyAdminToken は #1155 以前の形式（発行時刻を持たない HMAC-SHA256）のトークンを作る。
// 本番パッケージにテスト専用の公開関数を足さないため、ここで組み立てる。
func legacyAdminToken(userID uint, email, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d:%s", userID, email)
	return hex.EncodeToString(mac.Sum(nil))
}

// 期限切れと無効で 403 の本文を出し分ける（#1155）。
//
// 期限切れだけ「再ログインしてください」と案内する。ただし案内を出すのは署名が
// 一致した場合だけで、形式だけで判定すると「そのメールアドレスが管理者か」を
// レスポンスの差から判別できる列挙オラクルになる。
func TestEchoAdminAuth_ExpiredTokenMessage(t *testing.T) {
	const (
		secret = "test-secret"
		email  = "admin@example.com"
	)
	for _, tc := range []struct {
		name        string
		token       string
		wantMessage string
	}{
		{"旧形式で署名が一致するなら再ログイン案内", legacyAdminToken(1, email, secret), "有効期限"},
		{"旧形式の長さでも署名が違えば Forbidden のみ", strings.Repeat("ab", 32), "Forbidden"},
		{"そもそも形式が違えば Forbidden のみ", "garbage", "Forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newAdminAuthUserRepo(t)
			expectAdminAuthUser(mock, 1, email, true, "")
			res := serveAdminAuth(repo, secret, email, tc.token)
			if res.rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", res.rec.Code)
			}
			if !strings.Contains(res.rec.Body.String(), tc.wantMessage) {
				t.Errorf("body = %q, want contains %q", res.rec.Body.String(), tc.wantMessage)
			}
		})
	}
}
