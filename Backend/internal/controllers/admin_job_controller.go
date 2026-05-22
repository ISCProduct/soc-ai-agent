package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type AdminJobController struct {
	companyRepo  repository.CompanyRepository
	jobCategory  repository.JobCategoryRepository
	graduateRepo repository.GraduateEmploymentRepository
	audit        *services.AuditLogService
}

func NewAdminJobController(companyRepo repository.CompanyRepository, jobCategory repository.JobCategoryRepository, graduateRepo repository.GraduateEmploymentRepository, audit *services.AuditLogService) *AdminJobController {
	return &AdminJobController{
		companyRepo:  companyRepo,
		jobCategory:  jobCategory,
		graduateRepo: graduateRepo,
		audit:        audit,
	}
}

// JobCategories GET /api/admin/job-categories
func (c *AdminJobController) JobCategories(ctx echo.Context) error {
	categories, err := c.jobCategory.FindAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch job categories")
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"job_categories": categories,
	})
}

// JobPositions GET /api/admin/job-positions
func (c *AdminJobController) JobPositions(ctx echo.Context) error {
	var companyID *uint
	if idStr := strings.TrimSpace(ctx.QueryParam("company_id")); idStr != "" {
		if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
			value := uint(id)
			companyID = &value
		}
	}
	limit := 50
	if limitStr := strings.TrimSpace(ctx.QueryParam("limit")); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}
	positions, err := c.companyRepo.ListJobPositions(companyID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch job positions")
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"positions": positions,
	})
}

// CreateJobPosition POST /api/admin/job-positions
func (c *AdminJobController) CreateJobPosition(ctx echo.Context) error {
	var payload models.CompanyJobPosition
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if payload.CompanyID == 0 || strings.TrimSpace(payload.Title) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "company_id and title are required")
	}
	if payload.JobCategoryID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "job_category_id is required")
	}
	payload.IsActive = true
	if err := c.companyRepo.CreateJobPosition(&payload); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create job position")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "job_position.create", "company_job_position", payload.ID, map[string]interface{}{
		"company_id": payload.CompanyID,
		"title":      payload.Title,
	})
	return ctx.JSON(http.StatusOK, payload)
}

// JobPositionAction PATCH /api/admin/job-positions/:id/:action
func (c *AdminJobController) JobPositionAction(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	action := ctx.Param("action")

	position, err := c.companyRepo.FindJobPositionByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job position not found")
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	switch action {
	case "publish":
		position.DataStatus = "published"
		position.IsActive = true
	case "reject":
		position.DataStatus = "rejected"
		position.IsActive = false
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unknown action")
	}
	if err := c.companyRepo.UpdateJobPosition(position); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update")
	}
	c.audit.Record(actor, "job_position."+action, "company_job_position", position.ID, map[string]interface{}{
		"data_status": position.DataStatus,
	})
	return ctx.JSON(http.StatusOK, position)
}

// GraduateEmployments GET /api/admin/graduate-employments
func (c *AdminJobController) GraduateEmployments(ctx echo.Context) error {
	var companyID *uint
	if idStr := strings.TrimSpace(ctx.QueryParam("company_id")); idStr != "" {
		if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
			value := uint(id)
			companyID = &value
		}
	}
	limit := 50
	if limitStr := strings.TrimSpace(ctx.QueryParam("limit")); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}
	entries, err := c.graduateRepo.List(companyID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch graduate employments")
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// CreateGraduateEmployment POST /api/admin/graduate-employments
func (c *AdminJobController) CreateGraduateEmployment(ctx echo.Context) error {
	type payloadRequest struct {
		CompanyID      uint   `json:"company_id"`
		JobPositionID  *uint  `json:"job_position_id"`
		GraduateName   string `json:"graduate_name"`
		GraduationYear int    `json:"graduation_year"`
		SchoolName     string `json:"school_name"`
		Department     string `json:"department"`
		HiredAt        string `json:"hired_at"`
		Note           string `json:"note"`
	}
	var payload payloadRequest
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if payload.CompanyID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "company_id is required")
	}
	var hiredAt *time.Time
	if strings.TrimSpace(payload.HiredAt) != "" {
		if parsed, err := time.Parse("2006-01-02", payload.HiredAt); err == nil {
			hiredAt = &parsed
		}
	}
	entry := &models.GraduateEmployment{
		CompanyID:      payload.CompanyID,
		JobPositionID:  payload.JobPositionID,
		GraduateName:   strings.TrimSpace(payload.GraduateName),
		GraduationYear: payload.GraduationYear,
		SchoolName:     strings.TrimSpace(payload.SchoolName),
		Department:     strings.TrimSpace(payload.Department),
		HiredAt:        hiredAt,
		Note:           strings.TrimSpace(payload.Note),
	}
	if err := c.graduateRepo.Create(entry); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create graduate employment")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "graduate_employment.create", "graduate_employment", entry.ID, map[string]interface{}{
		"company_id": entry.CompanyID,
	})
	return ctx.JSON(http.StatusOK, entry)
}

// GetGraduateEmployment GET /api/admin/graduate-employments/:id
func (c *AdminJobController) GetGraduateEmployment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	entry, err := c.graduateRepo.FindByID(uint(id))
	if err != nil || entry == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return ctx.JSON(http.StatusOK, entry)
}

// UpdateGraduateEmployment PUT /api/admin/graduate-employments/:id
func (c *AdminJobController) UpdateGraduateEmployment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	entry, err := c.graduateRepo.FindByID(uint(id))
	if err != nil || entry == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	type updateRequest struct {
		CompanyID      uint   `json:"company_id"`
		JobPositionID  *uint  `json:"job_position_id"`
		GraduateName   string `json:"graduate_name"`
		GraduationYear int    `json:"graduation_year"`
		SchoolName     string `json:"school_name"`
		Department     string `json:"department"`
		HiredAt        string `json:"hired_at"`
		Note           string `json:"note"`
	}
	var payload updateRequest
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if payload.CompanyID != 0 {
		entry.CompanyID = payload.CompanyID
	}
	entry.JobPositionID = payload.JobPositionID
	entry.GraduateName = strings.TrimSpace(payload.GraduateName)
	entry.GraduationYear = payload.GraduationYear
	entry.SchoolName = strings.TrimSpace(payload.SchoolName)
	entry.Department = strings.TrimSpace(payload.Department)
	entry.Note = strings.TrimSpace(payload.Note)
	if strings.TrimSpace(payload.HiredAt) != "" {
		if parsed, err := time.Parse("2006-01-02", payload.HiredAt); err == nil {
			entry.HiredAt = &parsed
		}
	} else {
		entry.HiredAt = nil
	}
	if err := c.graduateRepo.Update(entry); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "graduate_employment.update", "graduate_employment", entry.ID, map[string]interface{}{
		"company_id": entry.CompanyID,
	})
	return ctx.JSON(http.StatusOK, entry)
}
