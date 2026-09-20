package controllers

// 企業ポータルのダッシュボードと応募者管理のテスト（#1320）。
// 実行: cd Backend && go test ./internal/controllers/ -run CompanyPortalApplication -v
//
// 見たいのは「他社のデータに触れないこと」と「破壊的操作が owner に限られること」。
// company_id はJWT由来で、クエリやボディからの企業指定を受け付けない。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

// withValue は context への値詰めを短く書くためのもの。
// middleware のキー型は非公開なので、テストからは context.WithValue を直接使う。
func withValue(ctx context.Context, key, val any) context.Context {
	return context.WithValue(ctx, key, val)
}

// assertAPIStatus は newAPIError が返す echo.HTTPError の status を検証する。
func assertAPIStatus(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("エラーが返っていない（status %d を期待）", want)
	}
	var he *echo.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("echo.HTTPError ではない: %T %v", err, err)
	}
	if he.Code != want {
		t.Errorf("status = %d, want %d", he.Code, want)
	}
}

type fakePortalApps struct {
	listedCompanyID uint
	listedStatus    string
	listedLimit     int
	listedOffset    int
	apps            []*entity.UserApplicationStatus
	total           int64

	pending int64

	updatedID        uint
	updatedCompanyID uint
	updateErr        error
}

func (f *fakePortalApps) ListForCompanyPortal(companyID uint, status string, limit, offset int) ([]*entity.UserApplicationStatus, int64, error) {
	f.listedCompanyID, f.listedStatus, f.listedLimit, f.listedOffset = companyID, status, limit, offset
	return f.apps, f.total, nil
}

func (f *fakePortalApps) CountPendingForCompanyPortal(companyID uint) (int64, error) {
	f.listedCompanyID = companyID
	return f.pending, nil
}

func (f *fakePortalApps) UpdateStatusForCompanyPortal(applicationID, companyID uint, status string, notes *string) (*entity.UserApplicationStatus, error) {
	f.updatedID, f.updatedCompanyID = applicationID, companyID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &entity.UserApplicationStatus{ID: applicationID, CompanyID: companyID, Status: status}, nil
}

type fakePortalJobs struct{ n int64 }

func (f *fakePortalJobs) CountPublishedJobPositions(uint) (int64, error) { return f.n, nil }

type fakePortalStudents struct {
	n     int64
	since time.Time
	// names は同意済み学生の氏名。ここに無いユーザーは同意していない扱い。
	names        map[uint]string
	askedUserIDs []uint
}

func (f *fakePortalStudents) CountNewStudents(_ uint, since time.Time) (int64, error) {
	f.since = since
	return f.n, nil
}

func (f *fakePortalStudents) VisibleStudentNames(_ uint, userIDs []uint) (map[uint]string, error) {
	f.askedUserIDs = userIDs
	if f.names == nil {
		return map[uint]string{}, nil
	}
	return f.names, nil
}

// 企業ユーザーとして認証済みのリクエストを作る。
func portalRequest(t *testing.T, method, path string, body string, companyID uint, role string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	ctx := r.Context()
	ctx = withValue(ctx, middleware.CompanyIDContextKey, companyID)
	ctx = withValue(ctx, middleware.CompanyUserRoleContextKey, role)
	// 企業ユーザーIDは自己無効化の判定に使う。既定を 1 にしておき、
	// 別のIDが必要なテストは portalRequestAs を使う。
	ctx = withValue(ctx, middleware.CompanyUserIDContextKey, uint(1))
	r = r.WithContext(ctx)

	rec := httptest.NewRecorder()
	return e.NewContext(r, rec), rec
}

func TestCompanyPortalApplication_Dashboard_0件でもエラーにならない(t *testing.T) {
	// 応募0件・求人0件が通常の初期状態。ここで落ちると導線を出せない。
	apps := &fakePortalApps{pending: 0}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{n: 0}, &fakePortalStudents{n: 0})

	c, rec := portalRequest(t, http.MethodGet, "/dashboard", "", 5, models.CompanyUserRoleOwner)
	if err := ctrl.Dashboard(c); err != nil {
		t.Fatalf("エラーが返った: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var got DashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSONを読めない: %v", err)
	}
	if got.PendingApplications != 0 || got.PublishedJobs != 0 || got.NewCandidates != 0 {
		t.Errorf("0件を返すべき: %+v", got)
	}
	// UI が「過去7日」と出せるよう、集計期間も返す。
	if got.NewCandidateWindowDays != newCandidateWindowDays {
		t.Errorf("集計期間が返っていない: %d", got.NewCandidateWindowDays)
	}
}

func TestCompanyPortalApplication_Dashboard_件数を返す(t *testing.T) {
	apps := &fakePortalApps{pending: 3}
	students := &fakePortalStudents{n: 7}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{n: 2}, students)

	c, rec := portalRequest(t, http.MethodGet, "/dashboard", "", 5, "member")
	if err := ctrl.Dashboard(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}

	var got DashboardResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.PendingApplications != 3 || got.PublishedJobs != 2 || got.NewCandidates != 7 {
		t.Errorf("件数が違う: %+v", got)
	}
	// JWT由来の company_id で集計していること
	if apps.listedCompanyID != 5 {
		t.Errorf("company_id が JWT 由来でない: %d", apps.listedCompanyID)
	}
	// 新着は直近7日
	if d := time.Since(students.since); d < 6*24*time.Hour || d > 8*24*time.Hour {
		t.Errorf("集計の起点がおかしい: %v前", d)
	}
}

func TestCompanyPortalApplication_Dashboard_未認証は401(t *testing.T) {
	ctrl := NewCompanyPortalApplicationController(&fakePortalApps{}, &fakePortalJobs{}, &fakePortalStudents{})
	c, _ := portalRequest(t, http.MethodGet, "/dashboard", "", 0, "")

	err := ctrl.Dashboard(c)
	assertAPIStatus(t, err, http.StatusUnauthorized)
}

func TestCompanyPortalApplication_List_JWT由来の企業で絞る(t *testing.T) {
	apps := &fakePortalApps{total: 1}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{}, &fakePortalStudents{})

	// クエリに company_id を入れても無視されること
	c, rec := portalRequest(t, http.MethodGet, "/applications?status=applied&limit=10&offset=5&company_id=999", "", 5, "member")
	if err := ctrl.List(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if apps.listedCompanyID != 5 {
		t.Errorf("他社の company_id が使われた: %d", apps.listedCompanyID)
	}
	if apps.listedStatus != "applied" || apps.listedLimit != 10 || apps.listedOffset != 5 {
		t.Errorf("絞り込みが渡っていない: status=%q limit=%d offset=%d",
			apps.listedStatus, apps.listedLimit, apps.listedOffset)
	}
}

func TestCompanyPortalApplication_List_limitの範囲を丸める(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"上限超えは既定に戻す", "?limit=1000", 30},
		{"0以下は既定に戻す", "?limit=0", 30},
		{"負値は既定に戻す", "?limit=-5", 30},
		{"範囲内はそのまま", "?limit=50", 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apps := &fakePortalApps{}
			ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{}, &fakePortalStudents{})
			c, _ := portalRequest(t, http.MethodGet, "/applications"+tt.query, "", 5, "member")
			if err := ctrl.List(c); err != nil {
				t.Fatalf("エラー: %v", err)
			}
			if apps.listedLimit != tt.want {
				t.Errorf("limit = %d, want %d", apps.listedLimit, tt.want)
			}
		})
	}
}

func TestCompanyPortalApplication_UpdateStatus_memberは403(t *testing.T) {
	// 破壊的操作は owner のみ（#1319 の共通方針）。
	apps := &fakePortalApps{}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{}, &fakePortalStudents{})

	c, _ := portalRequest(t, http.MethodPatch, "/applications/1/status", `{"status":"document_screening"}`, 5, "member")
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := ctrl.UpdateStatus(c)
	assertAPIStatus(t, err, http.StatusForbidden)

	// 権限が無いなら、サービスまで到達してはいけない。
	if apps.updatedID != 0 {
		t.Error("member なのに更新処理が呼ばれた")
	}
}

func TestCompanyPortalApplication_UpdateStatus_他社の応募は403(t *testing.T) {
	// サービス層が企業スコープで弾く。404 にすると存在が漏れる。
	apps := &fakePortalApps{updateErr: shared.ErrForbidden}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{}, &fakePortalStudents{})

	c, _ := portalRequest(t, http.MethodPatch, "/applications/42/status", `{"status":"rejected"}`, 5, models.CompanyUserRoleOwner)
	c.SetParamNames("id")
	c.SetParamValues("42")

	err := ctrl.UpdateStatus(c)
	assertAPIStatus(t, err, http.StatusForbidden)

	// JWT由来の company_id で検証していること
	if apps.updatedCompanyID != 5 {
		t.Errorf("company_id が JWT 由来でない: %d", apps.updatedCompanyID)
	}
}

func TestCompanyPortalApplication_UpdateStatus_ownerは更新できる(t *testing.T) {
	apps := &fakePortalApps{}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{}, &fakePortalStudents{})

	c, rec := portalRequest(t, http.MethodPatch, "/applications/7/status", `{"status":"document_screening"}`, 5, models.CompanyUserRoleOwner)
	c.SetParamNames("id")
	c.SetParamValues("7")

	if err := ctrl.UpdateStatus(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if apps.updatedID != 7 || apps.updatedCompanyID != 5 {
		t.Errorf("渡された値が違う: id=%d company=%d", apps.updatedID, apps.updatedCompanyID)
	}
}

func TestCompanyPortalApplication_UpdateStatus_statusが空なら400(t *testing.T) {
	ctrl := NewCompanyPortalApplicationController(&fakePortalApps{}, &fakePortalJobs{}, &fakePortalStudents{})

	c, _ := portalRequest(t, http.MethodPatch, "/applications/1/status", `{"status":"  "}`, 5, models.CompanyUserRoleOwner)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := ctrl.UpdateStatus(c)
	assertAPIStatus(t, err, http.StatusBadRequest)
}

// 応募者一覧の氏名は、スカウト公開に同意した学生のみ出す。
//
// 「自社に応募した学生を同意の有無に関わらず見せるか」は #1319 で未決定。
// 決まるまでは既存の同意ルールを適用する。ここを緩めると、
// 同意していない学生の氏名が企業に渡る。
func TestCompanyPortalApplication_List_同意した学生の氏名だけ出す(t *testing.T) {
	apps := &fakePortalApps{
		apps: []*entity.UserApplicationStatus{
			{ID: 1, UserID: 10, CompanyID: 5, Status: "applied"},
			{ID: 2, UserID: 11, CompanyID: 5, Status: "applied"},
		},
		total: 2,
	}
	// 10 は同意済み、11 は未同意（マップに無い）
	students := &fakePortalStudents{names: map[uint]string{10: "山田 太郎"}}
	ctrl := NewCompanyPortalApplicationController(apps, &fakePortalJobs{}, students)

	c, rec := portalRequest(t, http.MethodGet, "/applications", "", 5, "member")
	if err := ctrl.List(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}

	var got struct {
		Applications []struct {
			UserID      uint   `json:"user_id"`
			StudentName string `json:"student_name"`
		} `json:"applications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSONを読めない: %v (%s)", err, rec.Body.String())
	}
	if len(got.Applications) != 2 {
		t.Fatalf("件数が違う: %d", len(got.Applications))
	}

	byUser := map[uint]string{}
	for _, a := range got.Applications {
		byUser[a.UserID] = a.StudentName
	}
	if byUser[10] != "山田 太郎" {
		t.Errorf("同意済みの氏名が出ていない: %q", byUser[10])
	}
	if byUser[11] != "" {
		t.Errorf("未同意の学生の氏名が漏れている: %q", byUser[11])
	}

	// 氏名の解決は visibleStudents 経由であること（users を直接引いていない）
	if len(students.askedUserIDs) != 2 {
		t.Errorf("氏名の問い合わせが行われていない: %v", students.askedUserIDs)
	}
}

// portalRequestAs は企業ユーザーIDを指定できる版。
// 自分自身かどうかで挙動が変わる操作のテストに使う。
func portalRequestAs(t *testing.T, method, path, body string, companyID, companyUserID uint, role string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := portalRequest(t, method, path, body, companyID, role)
	ctx := withValue(c.Request().Context(), middleware.CompanyUserIDContextKey, companyUserID)
	c.SetRequest(c.Request().WithContext(ctx))
	return c, rec
}
