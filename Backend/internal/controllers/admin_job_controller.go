package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/internal/services/interfaces"
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
	audit        interfaces.AuditLogService
	schools      *services.SchoolService
}

// SetSchoolAccess は担当校スコープ検証に使うサービスを注入する(#1157)。
// 未設定のまま単体取得/更新を呼ぶと fail-closed で拒否する。
func (c *AdminJobController) SetSchoolAccess(schools *services.SchoolService) {
	c.schools = schools
}

func NewAdminJobController(companyRepo repository.CompanyRepository, jobCategory repository.JobCategoryRepository, graduateRepo repository.GraduateEmploymentRepository, audit interfaces.AuditLogService) *AdminJobController {
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
	return ctx.JSON(http.StatusOK, map[string]any{
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
	// 求人カタログは共有のため school_id は「承認済み企業の求人だけ見る」任意の絞り込み(閲覧制限ではない)。
	var jobSchoolID *uint
	if raw := strings.TrimSpace(ctx.QueryParam("school_id")); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			v := uint(id)
			jobSchoolID = &v
		}
	}
	positions, err := c.companyRepo.ListJobPositions(companyID, jobSchoolID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch job positions")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
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
	// 求人の公開状態は所属企業に合わせる。
	// DBデフォルトの draft のままだと、公開済み企業に管理者が求人を足しても
	// 学生側のクエリ(data_status='published')に乗らず、別途 publish 操作が要る（#1074）。
	//
	// エラーを握り潰すと継承がスキップされて draft で作られ、
	// 「公開済み企業に足したのに学生に出ない」が無言で再発する。
	company, err := c.companyRepo.FindByID(payload.CompanyID)
	if err != nil || company == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "company not found")
	}
	payload.DataStatus = company.DataStatus
	if err := c.companyRepo.CreateJobPosition(&payload); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create job position")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "job_position.create", "company_job_position", payload.ID, map[string]any{
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
		// 企業が未公開のまま求人を publish しても学生側クエリで弾かれるが、
		// ここでは止めない。63031830 で「FE が警告したうえで通す」という
		// 製品判断が既に入っており（job-positions/page-content.tsx の確認ダイアログ）、
		// サーバ側だけ 409 にすると「続行しますか？」に OK しても必ず失敗する
		// 死んだ UI になるため。企業を公開すれば draft の求人はまとめて
		// published になるので、実害も無い。
		position.DataStatus = "published"
		position.IsActive = true
	case "reject":
		position.DataStatus = "rejected"
		position.IsActive = false
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unknown action")
	}
	if err := c.companyRepo.UpdateJobPosition(position); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update job position")
	}
	c.audit.Record(actor, "job_position."+action, "company_job_position", position.ID, map[string]any{
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
	schoolID, err := echoAdminSchoolFilter(ctx)
	if err != nil {
		return err
	}
	entries, err := c.graduateRepo.List(companyID, schoolID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch graduate employments")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"entries": entries,
	})
}

// resolveGraduateSchoolID は作成する卒業生就職情報に紐づける学校IDを決める(#1157)。
//
//   - 担当校を持つ管理者(先生): 指定が無ければ担当校が1校ならそれを使い、複数なら school_id を要求する。
//     指定があれば担当校に含まれることを検証する。
//   - 無制限管理者(担当校0件): 指定をそのまま使う。未指定(nil)も許容する(学校に紐づかない全体データ)。
func (c *AdminJobController) resolveGraduateSchoolID(ctx echo.Context, requested *uint) (*uint, error) {
	if c.schools == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "school access check is not configured")
	}
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	restricted, allowed, err := c.schools.ResolveAdminAccess(adminUserID)
	if err != nil {
		return nil, echoInternalError(err)
	}
	if !restricted {
		return requested, nil
	}
	if requested == nil {
		if len(allowed) == 1 {
			return &allowed[0], nil
		}
		return nil, echo.NewHTTPError(http.StatusBadRequest, "school_id is required")
	}
	if err := ensureAdminSchoolAccess(ctx, c.schools, requested); err != nil {
		return nil, err
	}
	return requested, nil
}

// CreateGraduateEmployment POST /api/admin/graduate-employments
func (c *AdminJobController) CreateGraduateEmployment(ctx echo.Context) error {
	type payloadRequest struct {
		CompanyID      uint   `json:"company_id"`
		JobPositionID  *uint  `json:"job_position_id"`
		GraduateName   string `json:"graduate_name"`
		GraduationYear int    `json:"graduation_year"`
		SchoolName     string `json:"school_name"`
		SchoolID       *uint  `json:"school_id"`
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
	// school_id を入れないと、一覧(WHERE school_id = ?)も単体取得(ensureAdminSchoolAccess)も
	// 担当校を持つ管理者から見えなくなる。作成者自身が見られない行が増えるのを防ぐ(#1157)
	schoolID, err := c.resolveGraduateSchoolID(ctx, payload.SchoolID)
	if err != nil {
		return err
	}

	var hiredAt *time.Time
	if strings.TrimSpace(payload.HiredAt) != "" {
		if parsed, err := time.Parse("2006-01-02", payload.HiredAt); err == nil {
			hiredAt = &parsed
		}
	}
	entry := &models.GraduateEmployment{
		CompanyID:      payload.CompanyID,
		SchoolID:       schoolID,
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
	c.audit.Record(actor, "graduate_employment.create", "graduate_employment", entry.ID, map[string]any{
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
	// 一覧(GraduateEmployments)は担当校で絞り込まれるが、単体取得は素通りだった(#1157)
	if err := ensureAdminSchoolAccess(ctx, c.schools, entry.SchoolID); err != nil {
		return err
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
	// 他校の卒業生就職情報を書き換えられないようにする(#1157)
	if err := ensureAdminSchoolAccess(ctx, c.schools, entry.SchoolID); err != nil {
		return err
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
	c.audit.Record(actor, "graduate_employment.update", "graduate_employment", entry.ID, map[string]any{
		"company_id": entry.CompanyID,
	})
	return ctx.JSON(http.StatusOK, entry)
}
