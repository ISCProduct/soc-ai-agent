package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"Backend/internal/middleware"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
)

type TeacherStudentInsightController struct {
	svc *teacher.StudentInsightService
}

func NewTeacherStudentInsightController(svc *teacher.StudentInsightService) *TeacherStudentInsightController {
	return &TeacherStudentInsightController{svc: svc}
}

// TendencyAnalysis GET /api/admin/teacher/students/tendency-analysis
//
// 担当生徒の傾向タイプと向いている業界を一覧で返す（#1027）。
//
// 認可は EchoAdminAuth + EchoAdminSchoolScope に委ねる。
// 担当校を持つ管理者は school_id が必須で、範囲外は 403 で弾かれる
// （echo_adapter.go の EchoAdminSchoolScope）。担当校を持たない
// システム管理者は絞り込みなしで全生徒を見る。
//
// 「教員」を users.role で表現していないのは、role に 'teacher' を
// 書く経路がコード上に存在しないため。この製品では担当校を持つ管理者が
// 教員に相当する（models/school.go の AdminSchoolMembership のコメント）。
func (c *TeacherStudentInsightController) TendencyAnalysis(ctx echo.Context) error {
	const maxOffset = 10000 // DoS対策: 巨大オフセットのフルスキャンを防ぐ（admin_user_controller と同じ）

	limit := 25
	if l, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && o >= 0 {
		if o > maxOffset {
			o = maxOffset
		}
		offset = o
	}
	query := strings.TrimSpace(ctx.QueryParam("q"))

	// 担当校の絞り込みが解決されていなければ拒否する（fail-close）。
	// ok=false はミドルウェア(EchoAdminSchoolScope)を通っていないことを意味し、
	// そのまま nil を渡すと「絞り込みなし = 全校の生徒の氏名・メール・分析結果」を
	// 返してしまう。ルート定義から schoolScope が外れた場合に
	// 静かに全開放されるのを防ぐ。
	schoolID, ok := middleware.AdminSchoolFilterFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "school scope is not resolved")
	}

	result, err := c.svc.ListTendencies(limit, offset, query, schoolID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, result)
}
