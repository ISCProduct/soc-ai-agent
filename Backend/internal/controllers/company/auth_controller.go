package company

import (
	"Backend/internal/controllers/httpapi"
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
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	resp, err := c.svc.Login(req)
	if err != nil {
		if errors.Is(err, companyauth.ErrInvalidCredentials) {
			return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "invalid email or password")
		}
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

func (c *CompanyAuthController) AcceptInvite(ctx echo.Context) error {
	var req companyauth.AcceptInviteRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
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
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	if err := c.svc.RequestPasswordReset(req); err != nil {
		// DB障害などの内部エラーもここで飲み込むと調査できないのでログには残す。
		// ただしレスポンスは成功と区別できない形にする。
		httpapi.LogError(err)
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
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
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
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	if req.RefreshToken == "" {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "refresh_token is required")
	}

	resp, err := c.svc.RefreshSession(req.RefreshToken)
	if err != nil {
		if errors.Is(err, companyauth.ErrInvalidRefreshToken) {
			return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "invalid refresh token")
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Logout リフレッシュトークンを失効させる
// POST /api/company-auth/logout
func (c *CompanyAuthController) Logout(ctx echo.Context) error {
	var req companyRefreshRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	if err := c.svc.LogoutSession(req.RefreshToken); err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "ログアウトしました"})
}

func (c *CompanyAuthController) Me(ctx echo.Context) error {
	companyUserID, ok := httpapi.CompanyUserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
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
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "invalid email or password")
	case errors.Is(err, companyauth.ErrInviteNotFound):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "invalid invite token")
	case errors.Is(err, companyauth.ErrInviteExpired):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "invite token expired")
	case errors.Is(err, companyauth.ErrEmailExists):
		return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeConflict, "email already exists")
	case errors.Is(err, companyauth.ErrCompanyNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "company not found")
	case errors.Is(err, companyauth.ErrCompanyNotVerified):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "company is not verified")
	case errors.Is(err, companyauth.ErrAccountDisabled):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "このアカウントは無効化されています")
	case errors.Is(err, companyauth.ErrResetTokenInvalid):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "invalid password reset token")
	case errors.Is(err, companyauth.ErrResetTokenExpired):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "password reset token expired")
	case errors.Is(err, companyauth.ErrUserNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "company user not found")
	default:
		msg := err.Error()
		if msg == "forbidden" {
			return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "Forbidden")
		}
		if msg == "email and password are required" || msg == "token and password are required" ||
			msg == "password must be at least 8 characters" || msg == "email and name are required" ||
			msg == "email is invalid" || msg == "invalid role" || msg == "invite already accepted" {
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, msg)
		}
		return httpapi.InternalError(err)
	}
}
