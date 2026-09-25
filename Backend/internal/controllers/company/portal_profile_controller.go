package company

// 企業ポータルの自社プロフィール編集と担当者管理（#1322）。
//
// company_id は JWT 由来。URL やボディから企業を指定させない。
// 管理者向け(/api/admin/companies/:id/company-users)は :id で企業を
// 指定するが、企業ポータルでは自社しか触れないためパラメータを持たない。
//
// 新しいサービスは書かず、CompanyUserService の Invite / ListByCompany /
// SetDisabled をそのまま使う。

import (
	"errors"
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	companyauth "Backend/internal/services/companyauth"
	"Backend/internal/services/companyportal"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

type portalProfileService interface {
	Get(companyID uint) (*models.Company, error)
	Update(companyID uint, in companyportal.ProfileInput) (*models.Company, error)
}

type portalMemberService interface {
	Invite(companyID uint, req companyauth.InviteRequest) (*models.CompanyUser, error)
	ListByCompany(companyID uint) ([]models.CompanyUser, error)
	SetDisabled(companyID, companyUserID uint, disabled bool) (*models.CompanyUser, error)
}

type CompanyPortalProfileController struct {
	profiles portalProfileService
	members  portalMemberService
}

func NewCompanyPortalProfileController(
	profiles portalProfileService,
	members portalMemberService,
) *CompanyPortalProfileController {
	return &CompanyPortalProfileController{profiles: profiles, members: members}
}

type companyProfileResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Industry       string `json:"industry"`
	Location       string `json:"location"`
	WebsiteURL     string `json:"website_url"`
	LogoURL        string `json:"logo_url"`
	FoundedYear    int    `json:"founded_year"`
	EmployeeCount  int    `json:"employee_count"`
	Culture        string `json:"culture"`
	WorkStyle      string `json:"work_style"`
	WelfareDetails string `json:"welfare_details"`
	MainBusiness   string `json:"main_business"`
	// 以下は企業からは変更できない。状態を知るためだけに返す。
	DataStatus      string `json:"data_status"`
	IsActive        bool   `json:"is_active"`
	CorporateNumber string `json:"corporate_number"`
}

func toCompanyProfileResponse(c *models.Company) companyProfileResponse {
	return companyProfileResponse{
		ID:              c.ID,
		Name:            c.Name,
		Description:     c.Description,
		Industry:        c.Industry,
		Location:        c.Location,
		WebsiteURL:      c.WebsiteURL,
		LogoURL:         c.LogoURL,
		FoundedYear:     c.FoundedYear,
		EmployeeCount:   c.EmployeeCount,
		Culture:         c.Culture,
		WorkStyle:       c.WorkStyle,
		WelfareDetails:  c.WelfareDetails,
		MainBusiness:    c.MainBusiness,
		DataStatus:      c.DataStatus,
		IsActive:        c.IsActive,
		CorporateNumber: c.CorporateNumber,
	}
}

// profileBody は更新で受け取る値。
//
// すべてポインタにして「指定されなかった項目は変更しない」を表す。
// 未指定とゼロ値を区別しないと、一部だけ更新したつもりで他が消える。
type profileBody struct {
	Description    *string `json:"description"`
	Industry       *string `json:"industry"`
	Location       *string `json:"location"`
	WebsiteURL     *string `json:"website_url"`
	LogoURL        *string `json:"logo_url"`
	FoundedYear    *int    `json:"founded_year"`
	EmployeeCount  *int    `json:"employee_count"`
	Culture        *string `json:"culture"`
	WorkStyle      *string `json:"work_style"`
	WelfareDetails *string `json:"welfare_details"`
	MainBusiness   *string `json:"main_business"`
}

// GetCompany GET /api/company-portal/company
func (c *CompanyPortalProfileController) GetCompany(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	company, err := c.profiles.Get(companyID)
	if err != nil {
		return mapPortalProfileError(err)
	}
	return ctx.JSON(http.StatusOK, toCompanyProfileResponse(company))
}

// UpdateCompany PATCH /api/company-portal/company （owner のみ）
func (c *CompanyPortalProfileController) UpdateCompany(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	var body profileBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	company, err := c.profiles.Update(companyID, companyportal.ProfileInput{
		Description:    body.Description,
		Industry:       body.Industry,
		Location:       body.Location,
		WebsiteURL:     body.WebsiteURL,
		LogoURL:        body.LogoURL,
		FoundedYear:    body.FoundedYear,
		EmployeeCount:  body.EmployeeCount,
		Culture:        body.Culture,
		WorkStyle:      body.WorkStyle,
		WelfareDetails: body.WelfareDetails,
		MainBusiness:   body.MainBusiness,
	})
	if err != nil {
		return mapPortalProfileError(err)
	}
	return ctx.JSON(http.StatusOK, toCompanyProfileResponse(company))
}

// ListMembers GET /api/company-portal/members
func (c *CompanyPortalProfileController) ListMembers(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	users, err := c.members.ListByCompany(companyID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		items = append(items, map[string]any{
			"id":             u.ID,
			"email":          u.Email,
			"name":           u.Name,
			"role":           u.Role,
			"invite_pending": !u.PasswordSet(),
			"disabled":       u.Disabled(),
			"disabled_at":    u.DisabledAt,
		})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": items})
}

// InviteMember POST /api/company-portal/members （owner のみ）
func (c *CompanyPortalProfileController) InviteMember(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	var req companyauth.InviteRequest
	if err := ctx.Bind(&req); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	user, err := c.members.Invite(companyID, req)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
	})
}

type setMemberDisabledBody struct {
	Disabled bool `json:"disabled"`
}

// SetMemberDisabled PATCH /api/company-portal/members/:userID （owner のみ）
//
// 物理削除は提供しない。company_student_tags.created_by が参照しており、
// 行を消すとその担当者が付けた自社タグまで失われる（#1196）。
func (c *CompanyPortalProfileController) SetMemberDisabled(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	targetID, apiErr := httpapi.UintParam(ctx, "userID")
	if apiErr != nil {
		return apiErr
	}

	// 自分自身を無効化すると、その企業に owner がいなくなりうる。
	// 復旧が運営対応になるので防ぐ。
	if actorID, ok := middleware.CompanyUserIDFromContext(ctx.Request().Context()); ok && actorID == targetID {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "自分自身を無効化することはできません")
	}

	var body setMemberDisabledBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	user, err := c.members.SetDisabled(companyID, targetID, body.Disabled)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"id":          user.ID,
		"email":       user.Email,
		"disabled":    user.Disabled(),
		"disabled_at": user.DisabledAt,
	})
}

func (c *CompanyPortalProfileController) requireOwner(ctx echo.Context) (uint, error) {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return 0, httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	if !middleware.CompanyUserIsOwner(ctx.Request().Context()) {
		return 0, httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "この操作は管理者のみ行えます")
	}
	return companyID, nil
}

func mapPortalProfileError(err error) error {
	var ve *shared.ValidationError
	switch {
	case errors.Is(err, shared.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "この企業を操作する権限がありません")
	case errors.As(err, &ve):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, httpapi.ErrCodeValidationError, ve.Message)
	}
	return httpapi.InternalError(err)
}
