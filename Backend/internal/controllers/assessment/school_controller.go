package assessment

import (
	"errors"
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	assessmentsvc "Backend/internal/services/assessment"

	"github.com/labstack/echo/v4"
)

// maxTemplateBodyBytes は作成リクエストの上限。60問 × 200文字でも十分に収まる。
const maxTemplateBodyBytes = 256 << 10

// SchoolController は学校独自テンプレートの作成と、先生向けの受検状況 API（#1675）。
// 認可は EchoAdminAuth + EchoAdminSchoolScope に委ねる。担当校外の school_id は 403、
// 職員（staff）は担当校内でのみ使える。学校独自の結果はマッチングに使われない。
type SchoolController struct {
	svc *assessmentsvc.SchoolService
}

func NewSchoolController(svc *assessmentsvc.SchoolService) *SchoolController {
	return &SchoolController{svc: svc}
}

// scopedSchoolID は絞り込み済みの学校ID。システム管理者でも school_id を必須にする
// （学校をまたいだ作成・一覧は、どの学校のものか曖昧になるため行わない）。
func scopedSchoolID(ctx echo.Context) (uint, error) {
	filter, err := httpapi.AdminSchoolFilter(ctx)
	if err != nil {
		return 0, err
	}
	if filter == nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "school_id is required")
	}
	return *filter, nil
}

// List GET /api/admin/assessment-templates?school_id=
func (c *SchoolController) List(ctx echo.Context) error {
	schoolID, err := scopedSchoolID(ctx)
	if err != nil {
		return err
	}
	rows, err := c.svc.ListTemplates(schoolID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"templates": rows, "categories": assessmentsvc.Categories()})
}

// Create POST /api/admin/assessment-templates?school_id=
func (c *SchoolController) Create(ctx echo.Context) error {
	schoolID, err := scopedSchoolID(ctx)
	if err != nil {
		return err
	}
	adminID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	ctx.Request().Body = http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxTemplateBodyBytes)
	var in assessmentsvc.CreateTemplateInput
	if err := ctx.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "検査の形式が正しくありません")
	}
	got, err := c.svc.CreateTemplate(schoolID, adminID, in)
	if err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusCreated, got)
}

// Students GET /api/admin/assessment-templates/:id/students?school_id=
func (c *SchoolController) Students(ctx echo.Context) error {
	schoolID, err := scopedSchoolID(ctx)
	if err != nil {
		return err
	}
	templateID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return err
	}
	got, err := c.svc.StudentStatuses(schoolID, templateID)
	if err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusOK, got)
}

func mapSchoolError(err error) error {
	var invalid assessmentsvc.ErrInvalidTemplate
	switch {
	case errors.As(err, &invalid):
		return echo.NewHTTPError(http.StatusBadRequest, invalid.Reason)
	case errors.Is(err, assessmentsvc.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "検査が見つかりません")
	}
	return httpapi.InternalError(err)
}
