package controllers

import (
	"Backend/internal/services"
	"net/http"

	"github.com/labstack/echo/v4"
)

type AuthController struct {
	authService *services.AuthService
}

func NewAuthController(authService *services.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// Register 新規ユーザー登録
func (c *AuthController) Register(ctx echo.Context) error {
	var req services.RegisterRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	resp, err := c.authService.Register(req)
	if err != nil {
		if err.Error() == "email already exists" {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusCreated, resp)
}

// Login ログイン
func (c *AuthController) Login(ctx echo.Context) error {
	var req services.LoginRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	resp, err := c.authService.Login(req)
	if err != nil {
		msg := err.Error()
		if msg == "invalid email or password" || msg == "guest users cannot login" {
			return echo.NewHTTPError(http.StatusUnauthorized, msg)
		}
		if msg == "email_not_verified" || msg == "re_verification_required" {
			return ctx.JSON(http.StatusForbidden, map[string]string{"error": msg})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, msg)
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
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	resp, err := c.authService.GetUser(userID)
	if err != nil {
		if err.Error() == "user not found" {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
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
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if err := c.authService.RequestRegistration(body.Email); err != nil {
		if err.Error() == "email already exists" {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "confirmation email sent"})
}

// VerifyRegistration 仮登録トークンを検証してメールアドレスを返す
func (c *AuthController) VerifyRegistration(ctx echo.Context) error {
	token := ctx.QueryParam("token")
	if token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token is required")
	}

	email, err := c.authService.ValidateRegistrationToken(token)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{"email": email, "token": token})
}

// UpdateProfile ユーザープロフィール更新
func (c *AuthController) UpdateProfile(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req services.UpdateProfileRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	// リクエストボディの user_id を無視し、JWTから取得した値で上書き
	req.UserID = userID

	resp, err := c.authService.UpdateProfile(req)
	if err != nil {
		if err.Error() == "user not found" {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, resp)
}

// RequestPasswordReset POST /api/auth/forgot-password
func (c *AuthController) RequestPasswordReset(ctx echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := ctx.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
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
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if err := c.authService.ResetPassword(body.Token, body.Password); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
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
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "メールアドレスを確認しました。ログインしてください。"})
}

// DeleteAccount アカウントと全データを削除（個人情報保護法第28条対応）
// DELETE /api/auth/account
func (c *AuthController) DeleteAccount(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	if err := c.authService.DeleteAccount(userID); err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{"message": "アカウントを削除しました"})
}
