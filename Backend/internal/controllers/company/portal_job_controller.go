package company

// 企業ポータルの求人管理（#1321）。
//
// company_id は JWT 由来。ボディに company_id があっても無視する。
// 他社の求人IDを指定された場合は 404 ではなく 403 を返す（#1156）。
// 作成・更新・公開は破壊的操作なので owner のみ（#1319 の共通方針）。

import (
	"errors"
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/companyportal"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

type portalJobService interface {
	List(companyID uint) ([]models.CompanyJobPosition, error)
	Create(companyID uint, in companyportal.JobInput) (*models.CompanyJobPosition, error)
	Update(jobID, companyID uint, in companyportal.JobInput) (*models.CompanyJobPosition, error)
	SetPublished(jobID, companyID uint, published bool) (*companyportal.JobVisibility, error)
	CompanyPublished(companyID uint) bool
}

type CompanyPortalJobController struct {
	jobs portalJobService
}

func NewCompanyPortalJobController(jobs portalJobService) *CompanyPortalJobController {
	return &CompanyPortalJobController{jobs: jobs}
}

// jobResponse は求人1件分。
//
// IsPublished は「学生に見えているか」を表す。求人が published でも
// 企業本体が未公開なら見えないため、UI がその差を出せるようにする。
type jobResponse struct {
	ID              uint   `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	JobURL          string `json:"job_url"`
	JobCategoryID   uint   `json:"job_category_id"`
	MinSalary       int    `json:"min_salary"`
	MaxSalary       int    `json:"max_salary"`
	EmploymentType  string `json:"employment_type"`
	WorkLocation    string `json:"work_location"`
	RemoteOption    bool   `json:"remote_option"`
	RequiredSkills  string `json:"required_skills"`
	PreferredSkills string `json:"preferred_skills"`
	DataStatus      string `json:"data_status"`
	IsActive        bool   `json:"is_active"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func toJobResponse(j *models.CompanyJobPosition) jobResponse {
	return jobResponse{
		ID:              j.ID,
		Title:           j.Title,
		Description:     j.Description,
		JobURL:          j.JobURL,
		JobCategoryID:   j.JobCategoryID,
		MinSalary:       j.MinSalary,
		MaxSalary:       j.MaxSalary,
		EmploymentType:  j.EmploymentType,
		WorkLocation:    j.WorkLocation,
		RemoteOption:    j.RemoteOption,
		RequiredSkills:  j.RequiredSkills,
		PreferredSkills: j.PreferredSkills,
		DataStatus:      j.DataStatus,
		IsActive:        j.IsActive,
		CreatedAt:       j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type jobBody struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	JobURL          string `json:"job_url"`
	JobCategoryID   uint   `json:"job_category_id"`
	MinSalary       int    `json:"min_salary"`
	MaxSalary       int    `json:"max_salary"`
	EmploymentType  string `json:"employment_type"`
	WorkLocation    string `json:"work_location"`
	RemoteOption    bool   `json:"remote_option"`
	RequiredSkills  string `json:"required_skills"`
	PreferredSkills string `json:"preferred_skills"`
}

func (b jobBody) toInput() companyportal.JobInput {
	return companyportal.JobInput{
		Title:           b.Title,
		Description:     b.Description,
		JobURL:          b.JobURL,
		JobCategoryID:   b.JobCategoryID,
		MinSalary:       b.MinSalary,
		MaxSalary:       b.MaxSalary,
		EmploymentType:  b.EmploymentType,
		WorkLocation:    b.WorkLocation,
		RemoteOption:    b.RemoteOption,
		RequiredSkills:  b.RequiredSkills,
		PreferredSkills: b.PreferredSkills,
	}
}

// List GET /api/company-portal/jobs
// 下書きも含めて返す。下書きが見えないと編集できない。
func (c *CompanyPortalJobController) List(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	jobs, err := c.jobs.List(companyID)
	if err != nil {
		return mapPortalJobError(err)
	}
	items := make([]jobResponse, len(jobs))
	for i := range jobs {
		items[i] = toJobResponse(&jobs[i])
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"jobs":              items,
		"company_published": c.jobs.CompanyPublished(companyID),
	})
}

// Create POST /api/company-portal/jobs （owner のみ）
func (c *CompanyPortalJobController) Create(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	var body jobBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	job, err := c.jobs.Create(companyID, body.toInput())
	if err != nil {
		return mapPortalJobError(err)
	}
	return ctx.JSON(http.StatusCreated, toJobResponse(job))
}

// Update PATCH /api/company-portal/jobs/:id （owner のみ）
func (c *CompanyPortalJobController) Update(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	var body jobBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	job, err := c.jobs.Update(id, companyID, body.toInput())
	if err != nil {
		return mapPortalJobError(err)
	}
	return ctx.JSON(http.StatusOK, toJobResponse(job))
}

type publishBody struct {
	// Published を省略した場合は公開扱いにする。
	// 既定を非公開にすると、公開ボタンが無反応に見える。
	Published *bool `json:"published"`
}

// Publish POST /api/company-portal/jobs/:id/publish （owner のみ）
//
// 削除は提供しない。応募が紐づくため、非公開化で対応する。
func (c *CompanyPortalJobController) Publish(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	var body publishBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	published := body.Published == nil || *body.Published

	res, err := c.jobs.SetPublished(id, companyID, published)
	if err != nil {
		return mapPortalJobError(err)
	}
	// 企業本体が未公開なら、求人を published にしても学生に出ないことがある。
	// UI が黙らずに警告を出せるよう、企業側の状態も返す(#1321)。
	return ctx.JSON(http.StatusOK, map[string]any{
		"job":               toJobResponse(res.Job),
		"company_published": res.CompanyPublished,
	})
}

// requireOwner は owner 権限を確認し、JWT 由来の company_id を返す。
func (c *CompanyPortalJobController) requireOwner(ctx echo.Context) (uint, error) {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return 0, httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	if !middleware.CompanyUserIsOwner(ctx.Request().Context()) {
		return 0, httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "求人の管理は管理者のみ行えます")
	}
	return companyID, nil
}

func mapPortalJobError(err error) error {
	var ve *shared.ValidationError
	switch {
	case errors.Is(err, shared.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "この求人を操作する権限がありません")
	case errors.Is(err, companyportal.ErrJobLimitReached):
		return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeInvalidStatus, err.Error())
	case errors.As(err, &ve):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, httpapi.ErrCodeValidationError, ve.Message)
	}
	return httpapi.InternalError(err)
}
