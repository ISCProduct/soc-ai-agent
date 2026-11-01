package assessment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/repositories"
	assessmentsvc "Backend/internal/services/assessment"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// fakeSchoolRepo は学校 10 だけが存在し、テンプレート 1 が学校 10 のもの。
type fakeSchoolRepo struct {
	assessmentsvc.SchoolRepository
	created *models.AssessmentTemplate
}

func (f *fakeSchoolRepo) FindSchoolOrganizationID(id uint) (uint, error) {
	if id != 10 {
		return 0, gorm.ErrRecordNotFound
	}
	return 3, nil
}
func (f *fakeSchoolRepo) CreateSchoolTemplate(t *models.AssessmentTemplate, _ []models.AssessmentQuestion) error {
	t.ID = 5
	f.created = t
	return nil
}
func (f *fakeSchoolRepo) ListSchoolTemplates(uint) ([]models.AssessmentTemplate, error) {
	return []models.AssessmentTemplate{}, nil
}
func (f *fakeSchoolRepo) FindSchoolTemplate(id, schoolID uint) (*models.AssessmentTemplate, error) {
	if id != 1 || schoolID != 10 {
		return nil, gorm.ErrRecordNotFound
	}
	return &models.AssessmentTemplate{ID: 1, Title: "A校", Version: 1}, nil
}
func (f *fakeSchoolRepo) ListQuestions(uint, int) ([]models.AssessmentQuestion, error) {
	return nil, nil
}
func (f *fakeSchoolRepo) ListSchoolStudents(uint, uint) ([]repositories.SchoolStudent, error) {
	return nil, nil
}
func (f *fakeSchoolRepo) ListSessionsByTemplate(uint, uint, uint) ([]models.AssessmentSession, error) {
	return nil, nil
}
func (f *fakeSchoolRepo) CountAnswersBySessions([]uint) (map[uint]int, error) { return nil, nil }
func (f *fakeSchoolRepo) ListResultsBySessions([]uint) (map[uint]map[string]float64, error) {
	return nil, nil
}

// newSchoolServer は EchoAdminSchoolScope が解決した結果（絞り込み先）を直接差し込む。
// filter が nil なら「絞り込みなし」（システム管理者で school_id 未指定）、noScope なら未適用ルートの再現。
func newSchoolServer(filter *uint, noScope bool) (*echo.Echo, *fakeSchoolRepo) {
	repo := &fakeSchoolRepo{}
	c := NewSchoolController(assessmentsvc.NewSchoolService(repo))
	e := echo.New()
	g := e.Group("/admin/assessment-templates", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			r := ctx.Request()
			rc := context.WithValue(r.Context(), middleware.AdminUserIDContextKey, uint(99))
			if !noScope {
				rc = context.WithValue(rc, middleware.AdminSchoolFilterContextKey, filter)
			}
			ctx.SetRequest(r.WithContext(rc))
			return next(ctx)
		}
	})
	g.GET("", c.List)
	g.POST("", c.Create)
	g.GET("/:id/students", c.Students)
	return e, repo
}

func call(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

const validBody = `{"title":"A校","questions":[{"body":"好きだ","weight_category":"技術志向"}]}`

func TestSchoolController_Statuses(t *testing.T) {
	school10, school11 := uint(10), uint(11)
	tests := []struct {
		name               string
		filter             *uint
		noScope            bool
		method, path, body string
		want               int
	}{
		{"一覧: 担当校", &school10, false, "GET", "/admin/assessment-templates", "", 200},
		{"一覧: 学校未指定(システム管理者)は400", nil, false, "GET", "/admin/assessment-templates", "", 400},
		{"一覧: スコープ未適用は fail-close", nil, true, "GET", "/admin/assessment-templates", "", 403},
		{"作成: 正常は201", &school10, false, "POST", "/admin/assessment-templates", validBody, 201},
		{"作成: 学校未指定は400", nil, false, "POST", "/admin/assessment-templates", validBody, 400},
		{"作成: スコープ未適用は fail-close", nil, true, "POST", "/admin/assessment-templates", validBody, 403},
		{"作成: 壊れたJSONは400", &school10, false, "POST", "/admin/assessment-templates", `{`, 400},
		{"作成: 正典外カテゴリは400", &school10, false, "POST", "/admin/assessment-templates", `{"title":"x","questions":[{"body":"q","weight_category":"自由"}]}`, 400},
		{"作成: 存在しない学校は404", &school11, false, "POST", "/admin/assessment-templates", validBody, 404},
		{"受検状況: 自校のテンプレート", &school10, false, "GET", "/admin/assessment-templates/1/students", "", 200},
		{"受検状況: 他校のテンプレートは404", &school11, false, "GET", "/admin/assessment-templates/1/students", "", 404},
		{"受検状況: IDが不正は400", &school10, false, "GET", "/admin/assessment-templates/abc/students", "", 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newSchoolServer(tt.filter, tt.noScope)
			assert.Equal(t, tt.want, call(e, tt.method, tt.path, tt.body).Code)
		})
	}
}

// 作成時は担当校・作成者を認証済みの値から決める。リクエスト本文の値は使わない。
func TestSchoolController_CreateIgnoresBodyOwnership(t *testing.T) {
	school10 := uint(10)
	e, repo := newSchoolServer(&school10, false)
	body := `{"title":"A校","school_id":999,"is_standard":true,"created_by":1,"questions":[{"body":"好きだ","weight_category":"技術志向"}]}`
	rec := call(e, "POST", "/admin/assessment-templates", body)
	assert.Equal(t, http.StatusCreated, rec.Code)
	if assert.NotNil(t, repo.created) {
		assert.Equal(t, uint(10), *repo.created.SchoolID)
		assert.False(t, repo.created.IsStandard)
		assert.Equal(t, uint(99), *repo.created.CreatedBy)
	}
}
