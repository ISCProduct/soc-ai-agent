package controllers

import (
	"Backend/internal/services"
	"Backend/internal/services/interfaces"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// isProduction は APP_ENV=production のときのみ true を返す。
func isProduction() bool { return os.Getenv("APP_ENV") == "production" }

type AuthController struct {
	authService interfaces.AuthService
}

func NewAuthController(authService interfaces.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// Register 新規ユーザー登録
func (c *AuthController) Register(ctx echo.Context) error {
	var req services.RegisterRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}

	resp, err := c.authService.Register(req)
	if err != nil {
		if err.Error() == "email already exists" {
			if isProduction() {
				log.Printf("[Register] email already exists: %s", req.Email)
				return newAPIError(http.StatusConflict, ErrCodeDuplicateEmail, "Registration failed")
			}
			return newAPIError(http.StatusConflict, ErrCodeDuplicateEmail, err.Error())
		}
		if isProduction() {
			log.Printf("[Register] error: %v", err)
			return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Registration failed")
		}
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusCreated, resp)
}

// Login ログイン
func (c *AuthController) Login(ctx echo.Context) error {
	var req services.LoginRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}

	resp, err := c.authService.Login(req)
	if err != nil {
		msg := err.Error()
		if msg == "invalid email or password" || msg == "guest users cannot login" {
			return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, msg)
		}
		if msg == "email_not_verified" || msg == "re_verification_required" {
			return newAPIError(http.StatusForbidden, ErrCodeForbidden, msg)
		}
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// CreateGuest ゲストユーザー作成
func (c *AuthController) CreateGuest(ctx echo.Context) error {
	resp, err := c.authService.CreateGuestUser()
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusCreated, resp)
}

// GetUser ユーザー情報取得
func (c *AuthController) GetUser(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}

	resp, err := c.authService.GetUser(userID)
	if err != nil {
		if err.Error() == "user not found" {
			return newAPIError(http.StatusNotFound, ErrCodeNotFound, err.Error())
		}
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// RequestRegistration メールアドレスに確認リンクを送信
func (c *AuthController) RequestRegistration(ctx echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := ctx.Bind(&body); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}

	if err := c.authService.RequestRegistration(body.Email); err != nil {
		if isProduction() {
			log.Printf("[RequestRegistration] error for %s: %v", body.Email, err)
			return ctx.JSON(http.StatusOK, map[string]string{"message": "confirmation email sent"})
		}
		if err.Error() == "email already exists" {
			return newAPIError(http.StatusConflict, ErrCodeDuplicateEmail, err.Error())
		}
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "confirmation email sent"})
}

// VerifyRegistration 仮登録トークンを検証してメールアドレスを返す
func (c *AuthController) VerifyRegistration(ctx echo.Context) error {
	token := ctx.QueryParam("token")
	if token == "" {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "token is required")
	}

	email, err := c.authService.ValidateRegistrationToken(token)
	if err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"email": email, "token": token})
}

// UpdateProfile ユーザープロフィール更新
func (c *AuthController) UpdateProfile(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}

	var req services.UpdateProfileRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	req.UserID = userID

	resp, err := c.authService.UpdateProfile(req)
	if err != nil {
		if err.Error() == "user not found" {
			return newAPIError(http.StatusNotFound, ErrCodeNotFound, err.Error())
		}
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, err.Error())
	}

	return ctx.JSON(http.StatusOK, resp)
}

// RequestPasswordReset POST /api/auth/forgot-password
func (c *AuthController) RequestPasswordReset(ctx echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := ctx.Bind(&body); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
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
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}

	if err := c.authService.ResetPassword(body.Token, body.Password); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, err.Error())
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
		ctx.Bind(&req)
		token = req.Token
	}
	if err := c.authService.VerifyEmail(token); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, err.Error())
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "メールアドレスを確認しました。ログインしてください。"})
}

// DeleteAccount アカウントと全データを削除（個人情報保護法第28条対応）
// DELETE /api/auth/account
func (c *AuthController) DeleteAccount(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}

	if err := c.authService.DeleteAccount(userID); err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "アカウントを削除しました"})
}
