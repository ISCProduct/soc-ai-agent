package teacher

import (
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/models"
)

type stubGuidanceStore struct {
	created []*models.TeacherStudentGuidance
	active  []models.TeacherStudentGuidance
	err     error
}

func (s *stubGuidanceStore) Create(g *models.TeacherStudentGuidance) error {
	if s.err != nil {
		return s.err
	}
	g.ID = uint(len(s.created) + 1)
	cp := *g
	s.created = append(s.created, &cp)
	return nil
}

func (s *stubGuidanceStore) ListActiveByStudent(uint, int) ([]models.TeacherStudentGuidance, error) {
	return s.active, s.err
}

func (s *stubGuidanceStore) FindByIDForStudent(id, studentUserID uint) (*models.TeacherStudentGuidance, error) {
	for i := range s.active {
		if s.active[i].ID == id && s.active[i].StudentUserID == studentUserID {
			return &s.active[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (s *stubGuidanceStore) Dismiss(id, studentUserID uint) error {
	_, err := s.FindByIDForStudent(id, studentUserID)
	return err
}

type stubStudentByID struct {
	user *entity.User
	err  error
}

func (s *stubStudentByID) GetUserByID(uint) (*entity.User, error) {
	return s.user, s.err
}

func TestGuidanceService_CreateLowMatchDefaultMessage(t *testing.T) {
	store := &stubGuidanceStore{}
	svc := NewGuidanceService(store, &stubStudentByID{})
	view, err := svc.Create(CreateGuidanceInput{
		TeacherUserID:       9,
		StudentUserID:       3,
		Kind:                models.GuidanceKindLowMatch,
		SuggestedIndustries: []string{"情報通信業", "ソフトウェア開発"},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if view.ID == 0 || view.Kind != models.GuidanceKindLowMatch {
		t.Fatalf("view = %+v", view)
	}
	if !strings.Contains(view.Message, "マッチ度") || !strings.Contains(view.Message, "情報通信業") {
		t.Errorf("default message missing pieces: %q", view.Message)
	}
	if len(store.created) != 1 || store.created[0].TeacherUserID != 9 {
		t.Errorf("store = %+v", store.created)
	}
}

func TestGuidanceService_RejectsBadKind(t *testing.T) {
	svc := NewGuidanceService(&stubGuidanceStore{}, &stubStudentByID{})
	_, err := svc.Create(CreateGuidanceInput{Kind: "nope", Message: "x", StudentUserID: 1, TeacherUserID: 1})
	if !errors.Is(err, ErrGuidanceInvalidKind) {
		t.Fatalf("err = %v", err)
	}
}

func TestGuidanceService_ResolveStudentSchool(t *testing.T) {
	schoolID := uint(7)
	svc := NewGuidanceService(&stubGuidanceStore{}, &stubStudentByID{
		user: &entity.User{ID: 3, Role: "student", SchoolID: &schoolID},
	})
	got, err := svc.ResolveStudentSchool(3)
	if err != nil || got == nil || *got != 7 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestGuidanceService_ResolveStudentSchool_RejectsNonStudents(t *testing.T) {
	schoolID := uint(7)
	now := time.Now()
	tests := []struct {
		name string
		user entity.User
	}{
		{"教員", entity.User{Role: "teacher", SchoolID: &schoolID}},
		{"管理者", entity.User{Role: "student", IsAdmin: true, SchoolID: &schoolID}},
		{"ゲスト", entity.User{Role: "student", IsGuest: true, SchoolID: &schoolID}},
		{"退会済み", entity.User{Role: "student", WithdrawnAt: &now, SchoolID: &schoolID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewGuidanceService(&stubGuidanceStore{}, &stubStudentByID{user: &tt.user})
			if _, err := svc.ResolveStudentSchool(3); !errors.Is(err, ErrGuidanceStudentNotFound) {
				t.Fatalf("err = %v, want ErrGuidanceStudentNotFound", err)
			}
		})
	}
}

func TestGuidanceService_CreateWithoutIndustriesStoresNull(t *testing.T) {
	store := &stubGuidanceStore{}
	svc := NewGuidanceService(store, &stubStudentByID{})
	if _, err := svc.Create(CreateGuidanceInput{TeacherUserID: 9, StudentUserID: 3, Kind: models.GuidanceKindResume}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := store.created[0].SuggestedIndustries; got != nil {
		t.Fatalf("SuggestedIndustries = %q, want nil (JSON カラムに空文字は書けない)", *got)
	}
}
