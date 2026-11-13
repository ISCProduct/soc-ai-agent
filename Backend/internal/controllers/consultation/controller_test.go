package consultation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/school"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// #2028: 先生のスレッドは、学生の学校が担当校でなければ 403。学生は本人のスレッドだけ。

type store struct{ written int }

func (s *store) FindGuidance(id uint) (*models.TeacherStudentGuidance, error) {
	if id != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &models.TeacherStudentGuidance{ID: 1, StudentUserID: 10, Kind: models.GuidanceKindQuestion, Message: "質問"}, nil
}
func (s *store) CreateQuestion(uint, string) (*models.TeacherStudentGuidance, error) {
	return nil, errors.New("not used")
}
func (s *store) CreateMessage(*models.GuidanceMessage) error         { s.written++; return nil }
func (s *store) ListMessages(uint) ([]models.GuidanceMessage, error) { return nil, nil }
func (s *store) MarkReadBy(uint, string, time.Time) error            { return nil }
func (s *store) UnreadForStudent(uint) (map[uint]int, error)         { return map[uint]int{}, nil }
func (s *store) TeacherInbox(*uint, int) ([]repositories.ConsultationInboxRow, error) {
	return nil, nil
}

// 学生 10 は組織 1・学校 5 に所属する
type resolver struct{}

func (resolver) ResolveStudentSchool(uint) (uint, *uint, error) {
	school := uint(5)
	return 1, &school, nil
}

func teacherCtx(method, target string, adminID uint) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(`{"body":"返信です"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), middleware.AdminUserIDContextKey, adminID))
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	return ctx, rec
}

func TestTeacherReply_担当校以外は403(t *testing.T) {
	st := &store{}
	// 先生 20 の担当校は 3。学生の学校は 5
	c := NewController(teacher.NewConsultationService(st), resolver{}, testsupport.NewRestrictedSchoolService(20, 3))
	ctx, _ := teacherCtx(http.MethodPost, "/api/admin/teacher/consultations/1/messages", 20)
	var he *echo.HTTPError
	if err := c.TeacherReply(ctx); !errors.As(err, &he) || he.Code != http.StatusForbidden {
		t.Fatalf("err = %v, want 403", err)
	}
	if st.written != 0 {
		t.Fatal("担当校以外のスレッドに書き込んだ")
	}
}

func TestTeacherReply_担当校なら返信できる(t *testing.T) {
	st := &store{}
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(20)).Return([]models.School{{ID: 5, OrganizationID: 1}}, nil)
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)
	c := NewController(teacher.NewConsultationService(st), resolver{}, school.NewSchoolService(repo, nil))
	ctx, rec := teacherCtx(http.MethodPost, "/api/admin/teacher/consultations/1/messages", 20)
	if err := c.TeacherReply(ctx); err != nil || rec.Code != http.StatusCreated || st.written != 1 {
		t.Fatalf("err = %v code = %d written = %d", err, rec.Code, st.written)
	}
}

func TestStudentThread_他人のスレッドは404(t *testing.T) {
	c := NewController(teacher.NewConsultationService(&store{}), resolver{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/user/consultations/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, uint(99)))
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	var he *echo.HTTPError
	if err := c.StudentThread(ctx); !errors.As(err, &he) || he.Code != http.StatusNotFound {
		t.Fatalf("err = %v, want 404", err)
	}
}
