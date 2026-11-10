package insight

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"

	"github.com/labstack/echo/v4"
)

type esReaderSpy struct {
	schoolCalls []struct{ student, school uint }
	userCalls   []uint
	result      []models.ESReview
}

func (s *esReaderSpy) ListForSchoolStudent(student, school uint, _ int) ([]models.ESReview, error) {
	s.schoolCalls = append(s.schoolCalls, struct{ student, school uint }{student, school})
	return s.result, nil
}

func (s *esReaderSpy) ListByUser(user uint, _ int) ([]models.ESReview, error) {
	s.userCalls = append(s.userCalls, user)
	return s.result, nil
}

// 教員向けの履歴は、ルートに schoolScope が掛かっていないときに全校が見えてはいけない（fail-closed）。
// 担当校あり→学校で絞る、担当校なしの管理者→絞らない、スコープ未解決→403。
func TestTeacherESReviewController_List(t *testing.T) {
	school := uint(101)
	tests := []struct {
		name        string
		id          string
		scope       any // nil: ミドルウェア未経由 / (*uint)(nil): 無制限管理者 / &school: 担当校
		withScope   bool
		wantStatus  int
		wantSchool  int
		wantByUser  int
		wantSchoolI uint
	}{
		{name: "学生IDが数値でない", id: "abc", withScope: true, scope: &school, wantStatus: http.StatusBadRequest},
		{name: "学生IDが0", id: "0", withScope: true, scope: &school, wantStatus: http.StatusBadRequest},
		{name: "スコープ未解決は403（全校を返さない）", id: "9001", withScope: false, wantStatus: http.StatusForbidden},
		{name: "担当校ありは学校で絞る", id: "9001", withScope: true, scope: &school, wantStatus: http.StatusOK, wantSchool: 1, wantSchoolI: 101},
		{name: "担当校なしの管理者は絞らない", id: "9001", withScope: true, scope: (*uint)(nil), wantStatus: http.StatusOK, wantByUser: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &esReaderSpy{}
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.withScope {
				req = req.WithContext(context.WithValue(req.Context(), middleware.AdminSchoolFilterContextKey, tt.scope))
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues(tt.id)

			err := NewTeacherESReviewController(spy).List(c)
			status := rec.Code
			if he, ok := err.(*echo.HTTPError); ok {
				status = he.Code
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status != tt.wantStatus {
				t.Fatalf("status=%d, want %d", status, tt.wantStatus)
			}
			if len(spy.schoolCalls) != tt.wantSchool || len(spy.userCalls) != tt.wantByUser {
				t.Fatalf("school=%d byUser=%d, want school=%d byUser=%d",
					len(spy.schoolCalls), len(spy.userCalls), tt.wantSchool, tt.wantByUser)
			}
			if tt.wantSchool == 1 && (spy.schoolCalls[0].school != tt.wantSchoolI || spy.schoolCalls[0].student != 9001) {
				t.Fatalf("学校・学生の引数が違う: %+v", spy.schoolCalls[0])
			}
		})
	}
}

// 担当校外・他学園の学生は空で返す。存在を明かさないため 404 にしない。
func TestTeacherESReviewController_List_EmptyIsEmptyArray(t *testing.T) {
	school := uint(101)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.AdminSchoolFilterContextKey, &school))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("9001")

	if err := NewTeacherESReviewController(&esReaderSpy{}).List(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); body != "{\"es_reviews\":[],\"total\":0}\n" {
		t.Fatalf("body=%q", body)
	}
}
