package controllers

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/services/auth"
	"Backend/internal/services/interfaces"
	"Backend/internal/services/refreshtoken"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// tenantOrgID はリクエストcontextからテナント解決済み組織ID(0の場合はテナント制約なし)を取り出す。
func tenantOrgID(ctx echo.Context) uint {
	id, _ := middleware.TenantOrganizationIDFromContext(ctx.Request().Context())
	return id
}

// isProduction は APP_ENV=production のときのみ true を返す。
func isProduction() bool { return os.Getenv("APP_ENV") == "production" }

type AuthController struct {
	authService interfaces.AuthService
	userSecret  string
}

func NewAuthController(authService interfaces.AuthService, userSecret string) *AuthController {
	return &AuthController{authService: authService, userSecret: userSecret}
}

// guestUserIDForPromotion は昇格対象のゲストを X-User-Token から特定する。
//
// 対象をボディで受け取らないのは、他人のゲストアカウントを奪えるようにしないため。
// 引き継ぎを希望しているのにトークンが無効なら 0 ではなくエラーにする。
// ここで黙って新規登録にすると、学生は診断をやり直すことになるのに
// 画面上は成功に見える（#1374）。
func (c *AuthController) guestUserIDForPromotion(ctx echo.Context, req auth.RegisterRequest) (uint, error) {
	if !req.PromoteGuest {
		return 0, nil
	}
	if c.userSecret == "" {
		return 0, httpapi.NewAPIError(http.StatusServiceUnavailable, httpapi.ErrCodeValidationError, "guest promotion is not configured")
	}
	token := ctx.Request().Header.Get("X-User-Token")
	if token == "" {
		return 0, httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeValidationError, "ゲストの情報を引き継ぐにはログイン状態が必要です。ページを再読み込みしてからお試しください。")
	}
	userID, _, err := middleware.ParseJWT(token, c.userSecret)
	if err != nil || userID == 0 {
		return 0, httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeValidationError, "ゲストの有効期限が切れています。ページを再読み込みしてからお試しください。")
	}
	return userID, nil
}

// Register 新規ユーザー登録
func (c *AuthController) Register(ctx echo.Context) error {
	var req auth.RegisterRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}

	promoteGuestUserID, err := c.guestUserIDForPromotion(ctx, req)
	if err != nil {
		return err
	}

	resp, err := c.authService.Register(req, tenantOrgID(ctx), promoteGuestUserID)
	if err != nil {
		if errors.Is(err, auth.ErrGuestNotPromotable) {
			return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeValidationError, "このアカウントはゲストではないため引き継げません。ログインしてご利用ください。")
		}
		if err.Error() == "email already exists" {
			if isProduction() {
				log.Printf("[Register] email already exists: %s", req.Email)
				return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeDuplicateEmail, "Registration failed")
			}
			return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeDuplicateEmail, err.Error())
		}
		if isProduction() {
			log.Printf("[Register] error: %v", err)
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Registration failed")
		}
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusCreated, resp)
}

// Login ログイン
func (c *AuthController) Login(ctx echo.Context) error {
	var req auth.LoginRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}

	resp, err := c.authService.Login(req, tenantOrgID(ctx))
	if err != nil {
		msg := err.Error()
		if msg == "invalid email or password" || msg == "guest users cannot login" {
			return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, msg)
		}
		if msg == "email_not_verified" || msg == "re_verification_required" {
			return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, msg)
		}
		if msg == "tenant mismatch" {
			return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, msg)
		}
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// CreateGuest ゲストユーザー作成
func (c *AuthController) CreateGuest(ctx echo.Context) error {
	resp, err := c.authService.CreateGuestUser(tenantOrgID(ctx))
	if err != nil {
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusCreated, resp)
}

// GetUser ユーザー情報取得
func (c *AuthController) GetUser(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}

	resp, err := c.authService.GetUser(userID)
	if err != nil {
		if err.Error() == "user not found" {
			return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, err.Error())
		}
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// RequestRegistration メールアドレスに確認リンクを送信
func (c *AuthController) RequestRegistration(ctx echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}

	if err := c.authService.RequestRegistration(body.Email); err != nil {
		if isProduction() {
			log.Printf("[RequestRegistration] error for %s: %v", body.Email, err)
			return ctx.JSON(http.StatusOK, map[string]string{"message": "confirmation email sent"})
		}
		if err.Error() == "email already exists" {
			return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeDuplicateEmail, err.Error())
		}
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "confirmation email sent"})
}

// VerifyRegistration 仮登録トークンを検証してメールアドレスを返す。
// トークンは query または JSON body `{ "token": "..." }` を受け付ける（#1079）。
func (c *AuthController) VerifyRegistration(ctx echo.Context) error {
	token := registrationTokenFromRequest(ctx)
	if token == "" {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "token is required")
	}

	email, err := c.authService.ValidateRegistrationToken(token)
	if err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"email": email, "token": token})
}

func registrationTokenFromRequest(ctx echo.Context) string {
	if t := strings.TrimSpace(ctx.QueryParam("token")); t != "" {
		return t
	}
	var body struct {
		Token string `json:"token"`
	}
	_ = ctx.Bind(&body)
	return strings.TrimSpace(body.Token)
}

// UpdateProfile ユーザープロフィール更新
func (c *AuthController) UpdateProfile(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}

	var req auth.UpdateProfileRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	req.UserID = userID

	resp, err := c.authService.UpdateProfile(req)
	if err != nil {
		if err.Error() == "user not found" {
			return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, err.Error())
		}
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, resp)
}

// RequestPasswordReset POST /api/auth/forgot-password
func (c *AuthController) RequestPasswordReset(ctx echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}

	// エラーがあっても常に200を返す（情報漏洩防止）
	c.authService.RequestPasswordReset(body.Email)

	return ctx.JSON(http.StatusOK, map[string]string{"message": "パスワードリセットメールを送信しました"})
}

// ResetPassword POST /api/auth/reset-password
func (c *AuthController) ResetPassword(ctx echo.Context) error {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}

	if err := c.authService.ResetPassword(body.Token, body.Password); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "パスワードをリセットしました"})
}

// VerifyEmail メール認証トークンを検証してアカウントを有効化する
func (c *AuthController) VerifyEmail(ctx echo.Context) error {
	token := ctx.QueryParam("token")
	if token == "" {
		var req struct {
			Token string `json:"token"`
		}
		// クエリにトークンが無いときだけボディを読む。
		// その状況で壊れたボディが来たのなら、黙って空トークン扱いにせず不正リクエストとして返す。
		if err := ctx.Bind(&req); err != nil {
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
		}
		token = req.Token
	}
	if err := c.authService.VerifyEmail(token); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, err.Error())
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "メールアドレスを確認しました。ログインしてください。"})
}

// DeleteAccount アカウントを退会処理する（論理削除。保持期間後に物理削除）
// DELETE /api/auth/account
func (c *AuthController) DeleteAccount(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}

	if err := c.authService.DeleteAccount(userID); err != nil {
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "アカウントを退会処理しました"})
}

// refreshRequest はリフレッシュ/ログアウトのリクエストボディ (#616)
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh リフレッシュトークンをローテーションして新しいトークンペアを返す
// POST /api/auth/refresh
func (c *AuthController) Refresh(ctx echo.Context) error {
	var req refreshRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	if req.RefreshToken == "" {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "refresh_token is required")
	}

	resp, err := c.authService.RefreshSession(req.RefreshToken)
	if err != nil {
		if errors.Is(err, refreshtoken.ErrInvalidRefreshToken) {
			return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "invalid refresh token")
		}
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// Logout リフレッシュトークンを失効させる
// POST /api/auth/logout
func (c *AuthController) Logout(ctx echo.Context) error {
	var req refreshRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}

	if err := c.authService.LogoutSession(req.RefreshToken); err != nil {
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "ログアウトしました"})
}
