package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"
)

type fakeGrowthStore struct {
	userID uint
	rows   []models.UserGrowthSnapshot
}

func (f *fakeGrowthStore) ListRecent(userID uint, limit int) ([]models.UserGrowthSnapshot, error) {
	f.userID = userID
	if limit < len(f.rows) {
		return f.rows[:limit], nil
	}
	return f.rows, nil
}

type fakeUserLookup struct {
	user *models.User
}

func (f fakeUserLookup) GetUserByID(id uint) (*models.User, error) {
	if f.user == nil || f.user.ID != id {
		return nil, nil
	}
	return f.user, nil
}

func growthRows() []models.UserGrowthSnapshot {
	base := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	return []models.UserGrowthSnapshot{
		{
			ID: 2, UserID: 7, Source: "chat", SessionID: "s", CreatedAt: base.Add(time.Hour),
			ScoresJSON: `{"技術志向":70}`, PhaseProgressJSON: `[{"name":"興味分析","display_name":"興味","is_completed":true}]`,
		},
		{
			ID: 1, UserID: 7, Source: "chat", SessionID: "s", CreatedAt: base,
			ScoresJSON: `{"技術志向":50}`, PhaseProgressJSON: `[{"name":"興味分析","display_name":"興味","is_completed":false}]`,
		},
	}
}

func TestGrowthForStudent_IgnoresQueryUserID(t *testing.T) {
	store := &fakeGrowthStore{rows: growthRows()}
	controller := NewGrowthController(store, nil, nil)
	req := testsupport.WithUserID(httptest.NewRequest(http.MethodGet, "/api/user/growth-history?user_id=999", nil), 7)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, controller.ForStudent, testsupport.NewCtx(req, rec), http.StatusOK)
	if store.userID != 7 {
		t.Fatalf("listed user %d, want the token user 7", store.userID)
	}
	var body struct {
		HasPrevious bool `json:"has_previous"`
		Categories  []struct {
			Category    string `json:"category"`
			ChangeLabel string `json:"change_label"`
			Delta       *int   `json:"delta"`
		} `json:"categories"`
		Phase struct {
			ChangeLabel string `json:"change_label"`
		} `json:"phase"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.HasPrevious || len(body.Categories) != 1 || body.Categories[0].ChangeLabel != "伸びた" || body.Categories[0].Delta == nil || *body.Categories[0].Delta != 20 {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if body.Phase.ChangeLabel != "進んだ" {
		t.Fatalf("phase: %s", rec.Body.String())
	}
}

func TestGrowthForStaff_RequiresSchoolScope(t *testing.T) {
	store := &fakeGrowthStore{rows: growthRows()}
	controller := NewGrowthController(store, fakeUserLookup{}, nil)
	req := testsupport.WithStaffUserID(httptest.NewRequest(http.MethodGet, "/api/admin/students/7/growth-history", nil), 3)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("7")
	testsupport.AssertStatus(t, controller.ForStaff, ctx, http.StatusForbidden)
	if store.userID != 0 {
		t.Fatal("school scope が無いのに履歴を読んだ")
	}
}

func TestGrowthForStaff_OtherSchoolIsForbidden(t *testing.T) {
	schoolID := uint(9)
	store := &fakeGrowthStore{rows: growthRows()}
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(3)).Return([]models.School{{ID: 5, OrganizationID: 1}}, nil)
	controller := NewGrowthController(store, fakeUserLookup{user: &models.User{ID: 7, OrganizationID: 1, SchoolID: &schoolID}}, school.NewSchoolService(repo, nil))
	req := testsupport.WithSchoolFilter(
		testsupport.WithStaffUserID(httptest.NewRequest(http.MethodGet, "/api/admin/students/7/growth-history", nil), 3),
		&schoolID,
	)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("7")
	testsupport.AssertStatus(t, controller.ForStaff, ctx, http.StatusForbidden)
	if store.userID != 0 {
		t.Fatal("担当外の学生の履歴を読んだ")
	}
}

func TestGrowthForStaff_OwnSchool(t *testing.T) {
	schoolID := uint(5)
	store := &fakeGrowthStore{rows: growthRows()}
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(3)).Return([]models.School{{ID: schoolID, OrganizationID: 1, Name: "担当校"}}, nil)
	repo.On("FindByID", schoolID).Return(&models.School{ID: schoolID, OrganizationID: 1}, nil)
	controller := NewGrowthController(store, fakeUserLookup{user: &models.User{ID: 7, OrganizationID: 1, SchoolID: &schoolID}}, school.NewSchoolService(repo, nil))
	req := testsupport.WithSchoolFilter(
		testsupport.WithStaffUserID(httptest.NewRequest(http.MethodGet, "/api/admin/students/7/growth-history", nil), 3),
		&schoolID,
	)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("7")
	testsupport.AssertStatus(t, controller.ForStaff, ctx, http.StatusOK)
	if store.userID != 7 {
		t.Fatalf("listed user %d, want 7", store.userID)
	}
}
