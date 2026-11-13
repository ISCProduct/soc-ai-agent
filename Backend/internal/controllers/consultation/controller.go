// Package consultation は教員と学生の相談（案内への返信・学生からの質問、#2028）の API。
//
// 学生側は EchoUserAuth で本人を決め、本人の案内だけを扱う（他人の案内は 404）。
// 教員側は EchoAdminAuth の下に置き、スレッドの学生の学校が担当校でなければ 403 にする。
package consultation

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/services/school"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
)

// StudentSchoolResolver は学生の組織と学校を返す（teacher.GuidanceService が満たす）。
type StudentSchoolResolver interface {
	ResolveStudentSchool(studentUserID uint) (organizationID uint, schoolID *uint, err error)
}

type Controller struct {
	svc      *teacher.ConsultationService
	students StudentSchoolResolver
	schools  *school.SchoolService
}

func NewController(svc *teacher.ConsultationService, students StudentSchoolResolver, schools *school.SchoolService) *Controller {
	return &Controller{svc: svc, students: students, schools: schools}
}

type bodyRequest struct {
	Body string `json:"body"`
}

func parseID(ctx echo.Context) (uint, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "id が不正です")
	}
	return uint(id), nil
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, teacher.ErrConsultationNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "相談が見つかりません")
	case errors.Is(err, teacher.ErrConsultationEmpty), errors.Is(err, teacher.ErrConsultationTooLong):
		return echo.NewHTTPError(http.StatusBadRequest, teacher.FormatConsultationError(err))
	}
	return httpapi.InternalError(err)
}

func student(ctx echo.Context) (uint, error) {
	id, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || id == 0 {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return id, nil
}

// ---- 学生 ----

// Ask は POST /api/user/consultations（学生から先生への質問）。
func (c *Controller) Ask(ctx echo.Context) error {
	userID, err := student(ctx)
	if err != nil {
		return err
	}
	var req bodyRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	view, err := c.svc.Ask(userID, req.Body)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusCreated, view)
}

// StudentUnread は GET /api/user/consultations/unread（スレッドごとの、先生の未読の発言の数）。
func (c *Controller) StudentUnread(ctx echo.Context) error {
	userID, err := student(ctx)
	if err != nil {
		return err
	}
	counts, err := c.svc.UnreadForStudent(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	out := map[string]int{}
	total := 0
	for id, n := range counts {
		out[strconv.FormatUint(uint64(id), 10)] = n
		total += n
	}
	return ctx.JSON(http.StatusOK, map[string]any{"by_guidance": out, "total": total})
}

// StudentThread は GET /api/user/consultations/:id（先生の発言を既読にする）。
func (c *Controller) StudentThread(ctx echo.Context) error {
	userID, err := student(ctx)
	if err != nil {
		return err
	}
	id, err := parseID(ctx)
	if err != nil {
		return err
	}
	view, err := c.svc.StudentThread(userID, id)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusOK, view)
}

// StudentReply は POST /api/user/consultations/:id/messages。
func (c *Controller) StudentReply(ctx echo.Context) error {
	userID, err := student(ctx)
	if err != nil {
		return err
	}
	id, err := parseID(ctx)
	if err != nil {
		return err
	}
	var req bodyRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	view, err := c.svc.StudentReply(userID, id, req.Body)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusCreated, view)
}

// ---- 教員 ----

// Inbox は GET /api/admin/teacher/consultations?school_id=（担当校スコープの下）。
func (c *Controller) Inbox(ctx echo.Context) error {
	schoolID, err := httpapi.AdminSchoolFilter(ctx)
	if err != nil {
		return err
	}
	items, err := c.svc.Inbox(schoolID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": items})
}

// teacherGuidance はスレッドを引き、学生の学校が担当校かを確かめる（越境は 403）。
func (c *Controller) teacherGuidance(ctx echo.Context) (*teacherGuidanceRef, error) {
	id, err := parseID(ctx)
	if err != nil {
		return nil, err
	}
	g, err := c.svc.FindGuidance(id)
	if err != nil {
		return nil, toHTTPError(err)
	}
	orgID, schoolID, err := c.students.ResolveStudentSchool(g.StudentUserID)
	if err != nil {
		if errors.Is(err, teacher.ErrGuidanceStudentNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "相談が見つかりません")
		}
		return nil, httpapi.InternalError(err)
	}
	if err := httpapi.EnsureAdminSchoolAccessForUser(ctx, c.schools, orgID, schoolID); err != nil {
		return nil, err
	}
	return &teacherGuidanceRef{guidance: g}, nil
}

// TeacherThread は GET /api/admin/teacher/consultations/:id（学生の発言を既読にする）。
func (c *Controller) TeacherThread(ctx echo.Context) error {
	ref, err := c.teacherGuidance(ctx)
	if err != nil {
		return err
	}
	view, err := c.svc.TeacherThread(ref.guidance)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusOK, view)
}

// TeacherReply は POST /api/admin/teacher/consultations/:id/messages。
func (c *Controller) TeacherReply(ctx echo.Context) error {
	teacherID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	ref, err := c.teacherGuidance(ctx)
	if err != nil {
		return err
	}
	var req bodyRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	view, err := c.svc.TeacherReply(teacherID, ref.guidance, req.Body)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusCreated, view)
}
