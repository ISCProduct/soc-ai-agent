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

// #2028: 先生のスレッドは、学生の学校が担当校でなければ 404（存在しない ID と同じ）。学生は本人のスレッドだけ。

type store struct {
	written   int
	asked     int
	seenUpTo  uint
	gotBefore uint
}

func (s *store) FindGuidance(id uint) (*models.TeacherStudentGuidance, error) {
	if id != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &models.TeacherStudentGuidance{ID: 1, StudentUserID: 10, Kind: models.GuidanceKindQuestion, Message: "質問"}, nil
}
func (s *store) CreateQuestion(studentUserID uint, body string) (*models.TeacherStudentGuidance, error) {
	s.asked++
	return &models.TeacherStudentGuidance{ID: 2, StudentUserID: studentUserID, Kind: models.GuidanceKindQuestion, Message: body}, nil
}
func (s *store) CreateMessage(*models.GuidanceMessage) error { s.written++; return nil }
func (s *store) CreateMessageAndMarkRead(_ *models.GuidanceMessage, seenUpToID uint, _ time.Time) error {
	s.written++
	s.seenUpTo = seenUpToID
	return nil
}
func (s *store) ListMessages(uint) ([]models.GuidanceMessage, error) { return nil, nil }
func (s *store) MarkReadBy(uint, string, uint, time.Time) error      { return nil }
func (s *store) UnreadForStudent(uint) (map[uint]int, error)         { return map[uint]int{}, nil }
func (s *store) CountOpenQuestions(uint) (int64, error)              { return 0, nil }
func (s *store) ListStudentThreads(uint, *repositories.StudentThreadCursor, int) ([]repositories.StudentThreadRow, error) {
	return nil, nil
}
func (s *store) TeacherInbox(_ *uint, beforeID uint, _ int) ([]repositories.ConsultationInboxRow, error) {
	s.gotBefore = beforeID
	return nil, nil
}

// resolver は学生の組織と学校を返す。既定は組織 1・学校 5。
type resolver struct {
	school *uint
	err    error
}

func (r resolver) ResolveStudentSchool(uint) (uint, *uint, error) {
	if r.err != nil {
		return 0, nil, r.err
	}
	if r.school != nil {
		return 1, r.school, nil
	}
	school := uint(5)
	return 1, &school, nil
}

func ptr(v uint) *uint { return &v }

func teacherCtx(method, target string, adminID uint, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), middleware.AdminUserIDContextKey, adminID))
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	return ctx, rec
}

func studentCtx(method, target string, userID uint, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, userID))
	rec := httptest.NewRecorder()
	return testsupport.NewCtx(req, rec), rec
}

func code(err error) int {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he.Code
	}
	return -1
}

// 学校 5 は組織 1、学校 6 は組織 2 に属する
func schools() *school.SchoolService {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(20)).Return([]models.School{{ID: 5, OrganizationID: 1}}, nil)
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)
	repo.On("FindByID", uint(6)).Return(&models.School{ID: 6, OrganizationID: 2}, nil)
	return school.NewSchoolService(repo, nil)
}

func TestTeacherReply_担当校以外は404(t *testing.T) {
	st := &store{}
	// 先生 20 の担当校は 3。学生の学校は 5
	c := NewController(teacher.NewConsultationService(st), resolver{}, testsupport.NewRestrictedSchoolService(20, 3))
	ctx, _ := teacherCtx(http.MethodPost, "/api/admin/teacher/consultations/1/messages", 20, `{"body":"返信です"}`)
	if err := c.TeacherReply(ctx); code(err) != http.StatusNotFound {
		t.Fatalf("err = %v, want 404（存在しない ID と同じ応答）", err)
	}
	if st.written != 0 {
		t.Fatal("担当校以外のスレッドに書き込んだ")
	}
	ctx, _ = teacherCtx(http.MethodGet, "/api/admin/teacher/consultations/1", 20, "")
	if err := c.TeacherThread(ctx); code(err) != http.StatusNotFound {
		t.Fatalf("thread: err = %v, want 404", err)
	}
}

func TestTeacherReply_担当校なら返信でき_見ていた発言までを既読にする(t *testing.T) {
	st := &store{}
	c := NewController(teacher.NewConsultationService(st), resolver{}, schools())
	ctx, rec := teacherCtx(http.MethodPost, "/api/admin/teacher/consultations/1/messages", 20, `{"body":"返信です","seen_message_id":7}`)
	if err := c.TeacherReply(ctx); err != nil || rec.Code != http.StatusCreated || st.written != 1 || st.seenUpTo != 7 {
		t.Fatalf("err = %v code = %d written = %d seen = %d", err, rec.Code, st.written, st.seenUpTo)
	}
}

func TestStudentThread_他人のスレッドは404(t *testing.T) {
	c := NewController(teacher.NewConsultationService(&store{}), resolver{}, nil)
	ctx, _ := studentCtx(http.MethodGet, "/api/user/consultations/1", 99, "")
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	if err := c.StudentThread(ctx); code(err) != http.StatusNotFound {
		t.Fatalf("err = %v, want 404", err)
	}
}

// 質問は学生の学校の先生に届く。届け先が無い質問は受け付けない。
func TestAsk(t *testing.T) {
	for _, tt := range []struct {
		name     string
		resolver resolver
		body     string
		want     int
	}{
		{"学校の学生", resolver{}, `{"body":"質問です"}`, http.StatusCreated},
		{"学校の学園が本人と違う", resolver{school: ptr(6)}, `{"body":"質問です"}`, http.StatusBadRequest},
		{"学生でない（ゲスト・管理者・職員）", resolver{err: teacher.ErrGuidanceStudentNotFound}, `{"body":"質問です"}`, http.StatusForbidden},
		{"空", resolver{}, `{"body":"  "}`, http.StatusBadRequest},
		{"1001文字", resolver{}, `{"body":"` + strings.Repeat("あ", 1001) + `"}`, http.StatusBadRequest},
	} {
		st := &store{}
		c := NewController(teacher.NewConsultationService(st), tt.resolver, schools())
		ctx, rec := studentCtx(http.MethodPost, "/api/user/consultations", 10, tt.body)
		err := c.Ask(ctx)
		got := rec.Code
		if err != nil {
			got = code(err)
		}
		if got != tt.want {
			t.Errorf("%s: %d, want %d (err=%v)", tt.name, got, tt.want, err)
		}
		if tt.want != http.StatusCreated && st.asked != 0 {
			t.Errorf("%s: 受け付けない質問を保存した", tt.name)
		}
	}
}

type noSchoolResolver struct{}

func (noSchoolResolver) ResolveStudentSchool(uint) (uint, *uint, error) { return 1, nil, nil }

func TestAsk_学校が未登録なら受け付けない(t *testing.T) {
	st := &store{}
	c := NewController(teacher.NewConsultationService(st), noSchoolResolver{}, schools())
	ctx, _ := studentCtx(http.MethodPost, "/api/user/consultations", 10, `{"body":"質問です"}`)
	if err := c.Ask(ctx); code(err) != http.StatusBadRequest || st.asked != 0 {
		t.Fatalf("err = %v asked = %d", err, st.asked)
	}
}

func TestInbox_before_id(t *testing.T) {
	st := &store{}
	c := NewController(teacher.NewConsultationService(st), resolver{}, nil)
	for _, tt := range []struct {
		q    string
		want int
	}{{"before_id=abc", 400}, {"before_id=0", 400}, {"before_id=42&limit=500", 200}} {
		ctx, rec := teacherCtx(http.MethodGet, "/api/admin/teacher/consultations?"+tt.q, 20, "")
		// 担当校スコープ（EchoAdminSchoolScope）が学校 5 に決めた状態
		req := ctx.Request()
		ctx.SetRequest(req.WithContext(context.WithValue(req.Context(), middleware.AdminSchoolFilterContextKey, ptr(5))))
		err := c.Inbox(ctx)
		got := rec.Code
		if err != nil {
			got = code(err)
		}
		if got != tt.want {
			t.Errorf("%s: %d, want %d (err=%v)", tt.q, got, tt.want, err)
		}
	}
	if st.gotBefore != 42 {
		t.Fatalf("before_id が渡っていない: %d", st.gotBefore)
	}
}

func TestCursor(t *testing.T) {
	c := &repositories.StudentThreadCursor{UpdatedAt: time.Unix(0, 1763000000123456789), ID: 9}
	got, err := parseCursor(formatCursor(c))
	if err != nil || !got.UpdatedAt.Equal(c.UpdatedAt) || got.ID != 9 {
		t.Fatalf("got = %+v err = %v", got, err)
	}
	for _, bad := range []string{"x", "1_", "_1", "1_0", "a_1"} {
		if _, err := parseCursor(bad); err == nil {
			t.Errorf("%q を受け付けた", bad)
		}
	}
	if got, err := parseCursor(""); got != nil || err != nil {
		t.Fatalf("空: %v %v", got, err)
	}
}
