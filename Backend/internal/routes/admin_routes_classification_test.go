package routes_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// routeClass は /admin ルートの認可分類。
type routeClass string

const (
	// 指導系: 担当校スコープ内で職員(staff)にも開放する。
	classStaff routeClass = "staff"
	// 運用系: システム管理者(is_admin かつ担当校0件)専用。staff・担当校付き is_admin は403。
	classPlatform routeClass = "platform"
	// 権限管理: is_admin 必須。staff は403、担当校付き is_admin は通る。
	classRequireAdmin routeClass = "requireAdmin"
	// 意図的に staff へ開放しているが、締めるかは要判断のルート(#1398)。
	// 挙動は classStaff と同じ。分類だけを分けて「未判断」を表に残す。
	classStaffPendingReview routeClass = "staffPendingReview"
)

// expectedAdminRouteClasses は SetupAdminRoutes が登録する全ルートの期待分類。
// 新しい /admin ルートを足したら、ここへ分類を足すこと。足さないと
// TestAdminRoutes_ClassificationIsExhaustive が失敗する(fail-close)。
var expectedAdminRouteClasses = mergeRouteClasses(
	map[string]routeClass{
		// ── 指導系(staff可) ──
		"GET /me/school-access":                                 classStaff,
		"GET /users":                                            classStaff,
		"GET /teacher/students/tendency-analysis":               classStaff,
		"POST /teacher/students/:id/guidances":                  classStaff,
		"GET /interviews":                                       classStaff,
		"GET /interviews/:id/videos":                            classStaff,
		"GET /interviews/videos/:video_id/url":                  classStaff,
		"GET /dashboard/users":                                  classStaff,
		"GET /dashboard/users/:id":                              classStaff,
		"GET /dashboard/export/csv":                             classStaff,
		"GET /applications":                                     classStaff,
		"PATCH /applications/:id/status":                        classStaff,
		"GET /graduate-employments":                             classStaff,
		"POST /graduate-employments":                            classStaff,
		"GET /graduate-employments/:id":                         classStaff,
		"PUT /graduate-employments/:id":                         classStaff,
		"GET /schools/:id":                                      classStaff,
		"GET /schools/:id/company-approvals":                    classStaff,
		"POST /schools/:id/company-approvals":                   classStaff,
		"DELETE /schools/:id/company-approvals/:company_id":     classStaff,
		"GET /schools/:id/company-applications":                 classStaff,
		"POST /schools/:id/company-applications/:appId/approve": classStaff,
		"POST /schools/:id/company-applications/:appId/reject":  classStaff,
		"GET /schools/:id/job-suppressions":                     classStaff,
		"POST /schools/:id/job-suppressions":                    classStaff,
		"DELETE /schools/:id/job-suppressions/:jobId":           classStaff,
		"GET /companies":                                        classStaff,
		"GET /companies/industries":                             classStaff,
		"GET /companies/names":                                  classStaff,
		"GET /companies/:id":                                    classStaff,
		"GET /companies/:id/interview-questions":                classStaff,
		"GET /job-categories":                                   classStaff,
		"GET /job-positions":                                    classStaff,
		"GET /company-graph/target-year":                        classStaff,
	},
	map[string]routeClass{
		// ── 権限管理(requireAdmin) ──
		"PUT /users/:id":                       classRequireAdmin,
		"DELETE /users/:id":                    classRequireAdmin,
		"POST /schools/:id/members":            classRequireAdmin,
		"DELETE /schools/:id/members/:user_id": classRequireAdmin,
	},
	// ── 意図的に staff へ開放(要判断) ──
	// 掲載申請の審査(#1507)で担当校の職員も企業情報・面接質問・求人を触る可能性があり、
	// 現状は platform を掛けていない。ただし企業(PUT)・求人(POST)は全テナント共通データを
	// 書き換え、generate は OpenAI を呼ぶ(AIコスト発生)。
	// 締める場合は admin_routes.go の該当ルートへ platform を付け、ここの分類を
	// classPlatform へ移すこと(テストが両者のずれを検出する)。
	map[string]routeClass{
		"PUT /companies/:id":                               classStaffPendingReview,
		"GET /companies/:id/relation-graph":                classStaffPendingReview,
		"POST /companies/:id/interview-questions":          classStaffPendingReview,
		"POST /companies/:id/interview-questions/generate": classStaffPendingReview,
		"PUT /companies/:id/interview-questions/:qid":      classStaffPendingReview,
		"DELETE /companies/:id/interview-questions/:qid":   classStaffPendingReview,
		"POST /job-positions":                              classStaffPendingReview,
	},
	map[string]routeClass{
		// ── 運用系(platform専用) ──
		"PATCH /companies/:id/publish":                classPlatform,
		"PATCH /companies/:id/verify":                 classPlatform,
		"PATCH /companies/:id/reject":                 classPlatform,
		"PATCH /job-positions/:id/:action":            classPlatform,
		"POST /users/purge-expired":                   classPlatform,
		"GET /organizations":                          classPlatform,
		"POST /organizations":                         classPlatform,
		"GET /organizations/:id":                      classPlatform,
		"PUT /organizations/:id":                      classPlatform,
		"GET /organizations/:id/members":              classPlatform,
		"POST /organizations/:id/members":             classPlatform,
		"PUT /organizations/:id/members/:user_id":     classPlatform,
		"DELETE /organizations/:id/members/:user_id":  classPlatform,
		"GET /schools":                                classPlatform,
		"POST /schools":                               classPlatform,
		"GET /audit-logs":                             classPlatform,
		"GET /costs/summary":                          classPlatform,
		"GET /costs/breakdown":                        classPlatform,
		"GET /costs/daily":                            classPlatform,
		"GET /costs/monthly":                          classPlatform,
		"POST /profile-recalculation":                 classPlatform,
		"POST /profile-recalculation/:id":             classPlatform,
		"POST /profile-recalculation/:id/rollback":    classPlatform,
		"GET /profile-recalculation/history":          classPlatform,
		"GET /score-validation/correlation":           classPlatform,
		"GET /score-validation/phase-metrics":         classPlatform,
		"GET /score-validation/calibration":           classPlatform,
		"POST /score-validation/calibration/run":      classPlatform,
		"GET /score-validation/calibration/history":   classPlatform,
		"GET /score-validation/variants":              classPlatform,
		"POST /score-validation/variants":             classPlatform,
		"GET /score-validation/variants/results":      classPlatform,
		"GET /diagnosis-quality":                      classPlatform,
		"POST /collective-insights/rebuild-summaries": classPlatform,
		"GET /scraper-sessions":                       classPlatform,
		"POST /scraper-sessions":                      classPlatform,
		"DELETE /scraper-sessions/:site_key":          classPlatform,
		"GET /vector/status":                          classPlatform,
		"POST /vector/reembed":                        classPlatform,
		"GET /vector/stats":                           classPlatform,
		"GET /vector/collections":                     classPlatform,
		"GET /crawl-sources":                          classPlatform,
		"POST /crawl-sources":                         classPlatform,
		"PUT /crawl-sources/:id":                      classPlatform,
		"POST /crawl-sources/:id/run":                 classPlatform,
		"GET /crawl-runs":                             classPlatform,
		"POST /company-graph/crawl":                   classPlatform,
		"POST /company-graph/enrich-relations":        classPlatform,
		"POST /companies":                             classPlatform,
		"GET /companies/l1-coverage":                  classPlatform,
		"POST /companies/warm-l1":                     classPlatform,
		"POST /companies/fetch-missing-batch":         classPlatform,
		"POST /companies/seed-l1":                     classPlatform,
		"POST /companies/web-search":                  classPlatform,
		"POST /companies/web-search-relations":        classPlatform,
		"GET /companies/:id/gbiz-search":              classPlatform,
		"POST /companies/:id/gbiz-sync":               classPlatform,
		"POST /companies/:id/fetch-tech-stack":        classPlatform,
		"POST /companies/:id/fetch-info":              classPlatform,
		"POST /companies/:id/confirm-info":            classPlatform,
		"POST /companies/:id/fetch-relations":         classPlatform,
		"POST /companies/:id/confirm-relations":       classPlatform,
		"POST /companies/:id/fetch-jobs":              classPlatform,
		"POST /companies/:id/fetch-persona":           classPlatform,
		"POST /companies/:id/fetch-all":               classPlatform,
		"POST /companies/:id/fetch-primary":           classPlatform,
	},
)

func mergeRouteClasses(ms ...map[string]routeClass) map[string]routeClass {
	out := map[string]routeClass{}
	for _, m := range ms {
		for k, v := range m {
			if _, dup := out[k]; dup {
				panic("分類表でルートが重複している: " + k)
			}
			out[k] = v
		}
	}
	return out
}

const classificationAdminSecret = "test-admin-secret"

// 主体ごとの固定ID。fakeSchoolRepo の担当校は userID で引かれる。
const (
	staffWithSchoolID uint = iota + 1
	staffNoSchoolID
	adminNoSchoolID
	adminWithSchoolID
)

type principal struct {
	id      uint
	email   string
	isAdmin bool
	role    string
}

var principals = map[string]principal{
	"staff(担当校あり)":    {staffWithSchoolID, "staff1@example.com", false, models.UserRoleStaff},
	"staff(担当校0件)":    {staffNoSchoolID, "staff2@example.com", false, models.UserRoleStaff},
	"is_admin(担当校0件)": {adminNoSchoolID, "admin1@example.com", true, ""},
	"is_admin(担当校あり)": {adminWithSchoolID, "admin2@example.com", true, ""},
}

// newClassificationEcho は実際の SetupAdminRoutes(認証込み)を、sqlmock の UserRepository と
// 担当校を返す fake で組み立てる。コントローラーは nil のまま。
func newClassificationEcho(t *testing.T) (*echo.Echo, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
		staffWithSchoolID: {{ID: 5}},
		adminWithSchoolID: {{ID: 5}},
	}})

	e := echo.New()
	routes.SetupAdminRoutes(
		e.Group("/api"),
		// コントローラー22個(userRepo・schoolService の前まで)
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
		repositories.NewUserRepository(db), schools,
		classificationAdminSecret,
	)
	return e, mock
}

func registeredAdminRoutes(e *echo.Echo) []string {
	var got []string
	for _, r := range e.Routes() {
		// echo がグループ作成時に自動登録する 404 用の包括ルートは対象外。
		if r.Method == echo.RouteNotFound {
			continue
		}
		if p, ok := strings.CutPrefix(r.Path, "/api/admin"); ok {
			got = append(got, r.Method+" "+p)
		}
	}
	slices.Sort(got)
	return got
}

// 全 /admin ルートが分類表に載っていること、表に余計なルートが無いことを固定する。
func TestAdminRoutes_ClassificationIsExhaustive(t *testing.T) {
	e, _ := newClassificationEcho(t)
	registered := registeredAdminRoutes(e)
	if len(registered) == 0 {
		t.Fatal("/api/admin 配下のルートが 1 本も登録されていない。登録関数のシグネチャ変更を疑うこと")
	}

	seen := map[string]bool{}
	for _, key := range registered {
		seen[key] = true
		if _, ok := expectedAdminRouteClasses[key]; !ok {
			t.Errorf("分類されていないルート: %s（expectedAdminRouteClasses へ指導系/運用系/権限管理のどれかで追加すること）", key)
		}
	}
	for key := range expectedAdminRouteClasses {
		if !seen[key] {
			t.Errorf("分類表にあるが登録されていないルート: %s（ルートを消したなら表からも消すこと）", key)
		}
	}
}

// doAs は p として method path を叩く。ハンドラへ到達した場合(nil コントローラーで panic)は
// reached=true を返す。認可で止まった場合は reached=false と応答コード。
func doAs(e *echo.Echo, mock sqlmock.Sqlmock, p principal, method, path string) (code int, reached bool) {
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\?").
		WithArgs(p.email, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "is_admin", "is_guest", "role"}).
			AddRow(p.id, p.email, "テスト", p.isAdmin, false, p.role))

	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Admin-Email", p.email)
	req.Header.Set("X-Admin-Token", middleware.GenerateAdminToken(p.id, p.email, classificationAdminSecret))
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if recover() != nil {
				reached = true
			}
		}()
		e.ServeHTTP(rec, req)
	}()
	return rec.Code, reached
}

var pathParam = regexp.MustCompile(`:[A-Za-z_]+`)

// 分類表どおりに実際の認可が効いていることを、全ルート×4主体で確認する。
// 運用系(/audit-logs, /costs, /scraper-sessions, /companies/:id/fetch-*, 学校メンバー操作など)の
// staff 403 もここで網羅される。
func TestAdminRoutes_ClassificationMatchesAuthorization(t *testing.T) {
	e, mock := newClassificationEcho(t)

	// 各主体が「ハンドラ(または担当校スコープ)まで通るか」の期待。
	// 通る = 403/401 ではない。
	type want = map[string]bool
	passByClass := map[routeClass]want{
		classStaff:              {"staff(担当校あり)": true, "staff(担当校0件)": true, "is_admin(担当校0件)": true, "is_admin(担当校あり)": true},
		classStaffPendingReview: {"staff(担当校あり)": true, "staff(担当校0件)": true, "is_admin(担当校0件)": true, "is_admin(担当校あり)": true},
		classRequireAdmin:       {"staff(担当校あり)": false, "staff(担当校0件)": false, "is_admin(担当校0件)": true, "is_admin(担当校あり)": true},
		classPlatform:           {"staff(担当校あり)": false, "staff(担当校0件)": false, "is_admin(担当校0件)": true, "is_admin(担当校あり)": false},
	}

	for _, key := range registeredAdminRoutes(e) {
		class, ok := expectedAdminRouteClasses[key]
		if !ok {
			continue // 未分類は網羅性テストが報告する
		}
		method, tmpl, _ := strings.Cut(key, " ")
		path := "/api/admin" + pathParam.ReplaceAllString(tmpl, "1")
		for name, p := range principals {
			t.Run(fmt.Sprintf("%s/%s/%s", class, key, name), func(t *testing.T) {
				code, reached := doAs(e, mock, p, method, path)
				passed := reached || (code != http.StatusForbidden && code != http.StatusUnauthorized)
				if want := passByClass[class][name]; passed != want {
					t.Errorf("%s が %s で通過=%v(status=%d), want 通過=%v", name, key, passed, code, want)
				}
				if !passed && code != http.StatusForbidden {
					t.Errorf("拒否の応答 = %d, want 403", code)
				}
			})
		}
	}
}
