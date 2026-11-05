package insight_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/controllers/insight"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
)

type guidanceStoreStub struct{ created int }

func (s *guidanceStoreStub) Create(*models.TeacherStudentGuidance) error { s.created++; return nil }
func (s *guidanceStoreStub) ListActiveByStudent(uint, int) ([]models.TeacherStudentGuidance, error) {
	return nil, nil
}
func (s *guidanceStoreStub) FindByIDForStudent(uint, uint) (*models.TeacherStudentGuidance, error) {
	return nil, nil
}
func (s *guidanceStoreStub) Dismiss(uint, uint) error { return nil }

type studentStub struct{ user *entity.User }

func (s studentStub) GetUserByID(uint) (*entity.User, error) { return s.user, nil }

// 面談記録の作成は書き込みなので、学園をまたいだ対象には絶対に通してはいけない（#1675）。
// 担当校(5)に在籍しているように見えても、学校の学園(1)と生徒の学園(9)が違えば拒否する。
// 403 のときに行が作られていないことまで確認する。
func TestTeacherGuidanceController_Create_RejectsCrossOrganizationStudent(t *testing.T) {
	const (
		teacherID = uint(42)
		studentID = uint(3)
		schoolID  = uint(5)
		schoolOrg = uint(1)
	)
	sid := schoolID
	student := &entity.User{
		ID: studentID, Role: entity.UserRoleStudent,
		OrganizationID: 9, // 学校の学園(1)と違う
		SchoolID:       &sid,
	}

	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", teacherID).
		Return([]models.School{{ID: schoolID, OrganizationID: schoolOrg}}, nil)
	schoolRepo.On("FindByID", schoolID).
		Return(&models.School{ID: schoolID, OrganizationID: schoolOrg}, nil)

	store := &guidanceStoreStub{}
	ctrl := insight.NewTeacherGuidanceController(
		teacher.NewGuidanceService(store, studentStub{user: student}),
		school.NewSchoolService(schoolRepo, nil))

	body, _ := json.Marshal(map[string]any{"kind": "resume", "message": "x"})
	req := testsupport.WithAdminUserID(
		httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)), teacherID)
	req.Header.Set("Content-Type", "application/json")
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())
	ctx.SetParamNames("id")
	ctx.SetParamValues("3")

	err := ctrl.Create(ctx)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusForbidden {
		t.Fatalf("学園不一致の生徒への書き込みを拒否していない: err=%v", err)
	}
	if store.created != 0 {
		t.Fatalf("403 なのに面談記録が作られている: %d件", store.created)
	}
}
