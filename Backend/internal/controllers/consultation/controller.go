// Package consultation は教員と学生の相談（案内への返信・学生からの質問、#2028）の API。
//
// 学生側は EchoUserAuth で本人を決め、本人の案内だけを扱う（他人の案内は 404）。
// 教員側は EchoAdminAuth の下に置き、スレッドの学生の学校が担当校でなければ 404 にする
// （他校の相談の ID が存在するかを探られないよう、存在しない ID と同じ応答にそろえる）。
package consultation

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"Backend/internal/services/school"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
)

// StudentSchoolResolver は学生の組織と学校を返す（teacher.GuidanceService が満たす）。
// 学生ロールでない・管理者・ゲスト・退会済みなら teacher.ErrGuidanceStudentNotFound。
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

type teacherReplyRequest struct {
	Body string `json:"body"`
	// SeenMessageID は、返信を書いた画面に出ていた最後の発言の ID。学生の発言はそこまでを既読にする。
	SeenMessageID uint `json:"seen_message_id"`
}

const defaultListLimit = 30

func parseID(ctx echo.Context) (uint, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "id が不正です")
	}
	return uint(id), nil
}

func notFound() error {
	return echo.NewHTTPError(http.StatusNotFound, "相談が見つかりません")
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, teacher.ErrConsultationNotFound):
		return notFound()
	case errors.Is(err, teacher.ErrConsultationEmpty), errors.Is(err, teacher.ErrConsultationTooLong):
		return echo.NewHTTPError(http.StatusBadRequest, teacher.FormatConsultationError(err))
	case errors.Is(err, teacher.ErrConsultationTooManyOpen):
		return echo.NewHTTPError(http.StatusTooManyRequests, teacher.FormatConsultationError(err))
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
//
// 質問は学生の学校の先生の受信一覧に届く。学生でない（ゲスト・管理者・職員）なら 403、
// 学校が未登録・学園と一致しないなら 400 にする（受け付けても、どの先生にも届かないため）。
func (c *Controller) Ask(ctx echo.Context) error {
	userID, err := student(ctx)
	if err != nil {
		return err
	}
	var req bodyRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	orgID, schoolID, err := c.students.ResolveStudentSchool(userID)
	if errors.Is(err, teacher.ErrGuidanceStudentNotFound) {
		return echo.NewHTTPError(http.StatusForbidden, "学生のアカウントだけが先生に質問できます")
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
	if c.schools != nil {
		if schoolID, err = c.schools.EffectiveSchoolID(orgID, schoolID); err != nil {
			return httpapi.InternalError(err)
		}
	}
	if schoolID == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "学校が登録されていないため、先生に質問を送れません。プロフィールで学校を設定してください")
	}
	view, err := c.svc.Ask(userID, req.Body)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusCreated, view)
}

// formatCursor / parseCursor は学生の相談一覧の続きの位置を、"<更新日時のUnixナノ秒>_<案内ID>" で表す。
func formatCursor(c *repositories.StudentThreadCursor) string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%d_%d", c.UpdatedAt.UnixNano(), c.ID)
}

func parseCursor(raw string) (*repositories.StudentThreadCursor, error) {
	if raw == "" {
		return nil, nil
	}
	ts, id, ok := strings.Cut(raw, "_")
	if !ok {
		return nil, errors.New("invalid cursor")
	}
	nanos, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil || gid == 0 {
		return nil, errors.New("invalid cursor")
	}
	return &repositories.StudentThreadCursor{UpdatedAt: time.Unix(0, nanos), ID: uint(gid)}, nil
}

// StudentList は GET /api/user/consultations?limit=&cursor=（本人の案内と質問。閉じた案内も含む）。
func (c *Controller) StudentList(ctx echo.Context) error {
	userID, err := student(ctx)
	if err != nil {
		return err
	}
	cursor, err := parseCursor(ctx.QueryParam("cursor"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cursor が不正です")
	}
	page, err := c.svc.StudentThreads(userID, cursor, httpapi.LimitQuery(ctx, "limit", defaultListLimit))
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": page.Items, "next_cursor": formatCursor(page.Next)})
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

// StudentThread は GET /api/user/consultations/:id（表示した先生の発言を既読にする）。
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

// Inbox は GET /api/admin/teacher/consultations?school_id=&limit=&before_id=（担当校スコープの下）。
// 続きは、前のページの next_before_id を before_id に渡して取る。
func (c *Controller) Inbox(ctx echo.Context) error {
	schoolID, err := httpapi.AdminSchoolFilter(ctx)
	if err != nil {
		return err
	}
	var beforeID uint
	if raw := ctx.QueryParam("before_id"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || v == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "before_id が不正です")
		}
		beforeID = uint(v)
	}
	page, err := c.svc.Inbox(schoolID, beforeID, httpapi.LimitQuery(ctx, "limit", defaultListLimit))
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, page)
}

// teacherGuidance はスレッドを引き、学生の学校が担当校かを確かめる。
// 担当校でなければ、存在しない ID と同じく 404 にする（他校の相談の有無を探らせない）。
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
			return nil, notFound()
		}
		return nil, httpapi.InternalError(err)
	}
	if err := httpapi.EnsureAdminSchoolAccessForUser(ctx, c.schools, orgID, schoolID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return nil, notFound()
		}
		return nil, err
	}
	return &teacherGuidanceRef{guidance: g}, nil
}

// TeacherThread は GET /api/admin/teacher/consultations/:id（表示した学生の発言を既読にする）。
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
// body の seen_message_id までの学生の発言を、返信と同じトランザクションで既読にする。
func (c *Controller) TeacherReply(ctx echo.Context) error {
	teacherID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	ref, err := c.teacherGuidance(ctx)
	if err != nil {
		return err
	}
	var req teacherReplyRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	view, err := c.svc.TeacherReply(teacherID, ref.guidance, req.Body, req.SeenMessageID)
	if err != nil {
		return toHTTPError(err)
	}
	return ctx.JSON(http.StatusCreated, view)
}
