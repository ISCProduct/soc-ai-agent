package controllers

import (
	companyauth "Backend/internal/services/companyauth"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

type CompanyAuthController struct {
	svc *companyauth.CompanyUserService
}

func NewCompanyAuthController(svc *companyauth.CompanyUserService) *CompanyAuthController {
	return &CompanyAuthController{svc: svc}
}

func (c *CompanyAuthController) Login(ctx echo.Context) error {
	var req companyauth.LoginRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	resp, err := c.svc.Login(req)
	if err != nil {
		if errors.Is(err, companyauth.ErrInvalidCredentials) {
			return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "invalid email or password")
		}
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

func (c *CompanyAuthController) AcceptInvite(ctx echo.Context) error {
	var req companyauth.AcceptInviteRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	resp, err := c.svc.AcceptInvite(req)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// ForgotPassword はパスワード再設定メールの送信を要求する。
// POST /api/company-auth/forgot-password
//
// アカウントの存在有無を漏らさないため、結果に関わらず常に 200 を返す。
func (c *CompanyAuthController) ForgotPassword(ctx echo.Context) error {
	var req companyauth.ForgotPasswordRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	if err := c.svc.RequestPasswordReset(req); err != nil {
		// DB障害などの内部エラーもここで飲み込むと調査できないのでログには残す。
		// ただしレスポンスは成功と区別できない形にする。
		logError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{
		"message": "パスワード再設定用のメールを送信しました",
	})
}

// ResetPassword はトークンを検証してパスワードを再設定し、そのままログインさせる。
// POST /api/company-auth/reset-password
func (c *CompanyAuthController) ResetPassword(ctx echo.Context) error {
	var req companyauth.ResetPasswordRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	resp, err := c.svc.ResetPassword(req)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

type companyRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh リフレッシュトークンをローテーションして新しいトークンペアを返す
// POST /api/company-auth/refresh
func (c *CompanyAuthController) Refresh(ctx echo.Context) error {
	var req companyRefreshRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	if req.RefreshToken == "" {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "refresh_token is required")
	}

	resp, err := c.svc.RefreshSession(req.RefreshToken)
	if err != nil {
		if errors.Is(err, companyauth.ErrInvalidRefreshToken) {
			return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "invalid refresh token")
		}
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Logout リフレッシュトークンを失効させる
// POST /api/company-auth/logout
func (c *CompanyAuthController) Logout(ctx echo.Context) error {
	var req companyRefreshRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	if err := c.svc.LogoutSession(req.RefreshToken); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "ログアウトしました"})
}

func (c *CompanyAuthController) Me(ctx echo.Context) error {
	companyUserID, ok := echoCompanyUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	resp, err := c.svc.GetMe(companyUserID)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

func mapCompanyAuthError(err error) error {
	switch {
	case errors.Is(err, companyauth.ErrInvalidCredentials):
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "invalid email or password")
	case errors.Is(err, companyauth.ErrInviteNotFound):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid invite token")
	case errors.Is(err, companyauth.ErrInviteExpired):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invite token expired")
	case errors.Is(err, companyauth.ErrEmailExists):
		return newAPIError(http.StatusConflict, ErrCodeConflict, "email already exists")
	case errors.Is(err, companyauth.ErrCompanyNotFound):
		return newAPIError(http.StatusNotFound, ErrCodeNotFound, "company not found")
	case errors.Is(err, companyauth.ErrCompanyNotVerified):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "company is not verified")
	case errors.Is(err, companyauth.ErrAccountDisabled):
		return newAPIError(http.StatusForbidden, ErrCodeForbidden, "このアカウントは無効化されています")
	case errors.Is(err, companyauth.ErrResetTokenInvalid):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid password reset token")
	case errors.Is(err, companyauth.ErrResetTokenExpired):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "password reset token expired")
	case errors.Is(err, companyauth.ErrUserNotFound):
		return newAPIError(http.StatusNotFound, ErrCodeNotFound, "company user not found")
	default:
		msg := err.Error()
		if msg == "forbidden" {
			return newAPIError(http.StatusForbidden, ErrCodeForbidden, "Forbidden")
		}
		if msg == "email and password are required" || msg == "token and password are required" ||
			msg == "password must be at least 8 characters" || msg == "email and name are required" ||
			msg == "email is invalid" || msg == "invalid role" || msg == "invite already accepted" {
			return newAPIError(http.StatusBadRequest, ErrCodeValidationError, msg)
		}
		return echoInternalError(err)
	}
}
