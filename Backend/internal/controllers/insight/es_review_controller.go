package insight

import (
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/models"

	"github.com/labstack/echo/v4"
)

// ESReviewReader は教員向けの ES添削履歴の読み出し面（#1531）。
type ESReviewReader interface {
	// ListForSchoolStudent は担当校の在籍学生1名の履歴。学園一致はクエリ側で担保する。
	ListForSchoolStudent(studentUserID, schoolID uint, limit int) ([]models.ESReview, error)
	// ListByUser は学校で絞らない読み出し。無制限管理者だけが通る経路で使う。
	ListByUser(userID uint, limit int) ([]models.ESReview, error)
}

// TeacherESReviewController は教員が担当校の学生の ES添削履歴を見るための経路（#1531）。
type TeacherESReviewController struct {
	reviews ESReviewReader
}

func NewTeacherESReviewController(reviews ESReviewReader) *TeacherESReviewController {
	return &TeacherESReviewController{reviews: reviews}
}

const esReviewHistoryLimit = 50

// List GET /api/admin/teacher/students/:id/es-reviews
//
// 担当校ありの管理者・職員は、担当校に在籍する学生の履歴だけを見られる。
// 学校の絞り込みはクエリ側で学園一致つきに行うので、学校名を他学園の校名に
// 書き替えた学生の履歴が混ざることはない（#1675 と同じ方針）。
func (c *TeacherESReviewController) List(ctx echo.Context) error {
	studentID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || studentID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid student id")
	}
	schoolID, err := httpapi.AdminSchoolFilter(ctx)
	if err != nil {
		return err
	}

	var reviews []models.ESReview
	if schoolID == nil {
		// 担当校を持たない無制限管理者。学校で絞らない。
		reviews, err = c.reviews.ListByUser(uint(studentID), esReviewHistoryLimit)
	} else {
		reviews, err = c.reviews.ListForSchoolStudent(uint(studentID), *schoolID, esReviewHistoryLimit)
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
	// 担当校外・他学園の学生は 0 件になる。存在を明かさないため 404 ではなく空で返す。
	if reviews == nil {
		reviews = []models.ESReview{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"es_reviews": reviews, "total": len(reviews)})
}
