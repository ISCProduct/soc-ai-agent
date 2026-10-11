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

// Register POST /api/company-auth/register
// 招待なしで企業＋担当者（owner）を作成し、そのままログイン状態を返す。
func (c *CompanyAuthController) Register(ctx echo.Context) error {
	var req companyauth.RegisterRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "リクエストの形式が正しくありません")
	}
	resp, err := c.svc.Register(req)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusCreated, resp)
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
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "メールアドレスまたはパスワードが正しくありません")
	case errors.Is(err, companyauth.ErrInviteNotFound):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "招待リンクが無効です")
	case errors.Is(err, companyauth.ErrInviteExpired):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "招待リンクの有効期限が切れています")
	case errors.Is(err, companyauth.ErrEmailExists):
		return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeConflict, "このメールアドレスは既に登録されています")
	case errors.Is(err, companyauth.ErrCompanyNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "企業が見つかりません")
	case errors.Is(err, companyauth.ErrCompanyNotVerified):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "この企業はまだ認証されていないため招待できません")
	case errors.Is(err, companyauth.ErrAccountDisabled):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "このアカウントは無効化されています")
	case errors.Is(err, companyauth.ErrResetTokenInvalid):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "パスワード再設定リンクが無効です")
	case errors.Is(err, companyauth.ErrResetTokenExpired):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "パスワード再設定リンクの有効期限が切れています")
	case errors.Is(err, companyauth.ErrUserNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "企業ユーザーが見つかりません")
	default:
		msg := err.Error()
		if msg == "forbidden" {
			return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "権限がありません")
		}
		if msg == "COMPANY_USER_SECRET is not configured" {
			return httpapi.NewAPIError(
				http.StatusServiceUnavailable,
				httpapi.ErrCodeServiceUnavail,
				"企業ポータルの認証設定（COMPANY_USER_SECRET）が未設定です。管理者に連絡してください",
			)
		}
		switch msg {
		case "email and password are required":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "メールアドレスとパスワードを入力してください")
		case "token and password are required":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "トークンとパスワードを入力してください")
		case "password must be at least 8 characters":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "パスワードは8文字以上で入力してください")
		case "email and name are required":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "メールアドレスと担当者名を入力してください")
		case "email is invalid":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "メールアドレスの形式を確認してください")
		case "invalid role":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "権限の指定が不正です")
		case "invite already accepted":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "この招待はすでに受諾済みです")
		case "company_name, name, email and password are required":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "企業名・担当者名・メールアドレス・パスワードを入力してください")
		case "company_name is too long":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "企業名が長すぎます")
		case "name is too long":
			return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "担当者名が長すぎます")
		}
		return httpapi.InternalError(err)
	}
}
