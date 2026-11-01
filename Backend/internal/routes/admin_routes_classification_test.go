package routes_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"runtime/debug"
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

// 担当校スコープ(EchoAdminSchoolScope)がミドルウェアで掛かっているルート。
// school_id クエリの検証(担当校外は403・担当校ありで未指定は400)は、ここに挙げた
// ルートだけがミドルウェアで行う。ここに無い指導系ルートは「スコープなし」で、
// 単一リソースの所属校チェックはコントローラ側(EnsureAdminSchoolAccess 等)の責務。
// コントローラ内の検査は各コントローラのテストで見ること(ここでは見ない)。
// TestAdminRoutes_SchoolScopeMiddleware が、この集合と実際の挙動のずれを検出する。
var schoolScopedRoutes = map[string]bool{
	"GET /users": true,
	"GET /teacher/students/tendency-analysis": true,
	"GET /interviews":                         true,
	"GET /dashboard/users":                    true,
	"GET /dashboard/export/csv":               true,
	"GET /applications":                       true,
	"GET /graduate-employments":               true,
}

// 主体ごとの固定ID。fakeSchoolRepo の担当校は userID で引かれる。
const (
	staffWithSchoolID uint = iota + 1
	staffNoSchoolID
	adminNoSchoolID
	adminWithSchoolID
)

type principal struct {
	name    string // サブテスト名に使うASCII名
	id      uint
	email   string
	isAdmin bool
	role    string
	anon    bool // 認証ヘッダを付けない
}

// 順序固定(サブテスト名の安定のため)。
var (
	pStaffSchool   = principal{name: "staff_with_school", id: staffWithSchoolID, email: "staff1@example.com", role: models.UserRoleStaff}
	pStaffNoSchool = principal{name: "staff_no_school", id: staffNoSchoolID, email: "staff2@example.com", role: models.UserRoleStaff}
	pAdminNoSchool = principal{name: "admin_no_school", id: adminNoSchoolID, email: "admin1@example.com", isAdmin: true}
	pAdminSchool   = principal{name: "admin_with_school", id: adminWithSchoolID, email: "admin2@example.com", isAdmin: true}
	pAnonymous     = principal{name: "anonymous", anon: true}
	allPrincipals  = []principal{pStaffSchool, pStaffNoSchool, pAdminNoSchool, pAdminSchool, pAnonymous}
)

// 認可で拒否されたときの本文。echo_adapter.go のメッセージと対応する。
// GetUserByEmail 失敗などの AdminAuth 由来の403("Forbidden")と取り違えないために本文で区別する。
const (
	msgUnauthorized = "Unauthorized"
	msgPlatformOnly = "platform admin only"
	msgAdminOnly    = "この操作は管理者のみ行えます"
	msgSchoolDenied = "school access denied"
	msgSchoolIDReq  = "school_id is required"
)

// newClassificationEcho は実際の SetupAdminRoutes(認証込み)を、sqlmock の UserRepository と
// 担当校を返す fake で組み立てる。コントローラーは nil のまま。
// 共有の mock を使うため、このファイルのテストで t.Parallel() を使ってはならない。
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
	}}, nil)

	e := echo.New()
	registerAdminRoutesWithNilControllers(e.Group("/api"), repositories.NewUserRepository(db), schools, classificationAdminSecret)
	return e, mock
}

// registeredAdminRoutes は /api/admin 配下の登録ルートを "METHOD /path" で返す。
// echo が自動登録する 404 用の包括ルート(RouteNotFound)は対象外。
// /api/admin 配下以外のルートが登録されていたら、登録関数の想定外の変更として失敗させる。
func registeredAdminRoutes(t *testing.T, e *echo.Echo) []string {
	t.Helper()
	var got []string
	for _, r := range e.Routes() {
		if r.Method == echo.RouteNotFound {
			continue
		}
		rest, ok := strings.CutPrefix(r.Path, "/api/admin")
		if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
			t.Fatalf("/api/admin 配下ではないルートが登録されている: %s %s", r.Method, r.Path)
		}
		got = append(got, r.Method+" "+rest)
	}
	slices.Sort(got)
	return got
}

// 全 /admin ルートが分類表に載っていること、表に余計なルートが無いことを固定する。
func TestAdminRoutes_ClassificationIsExhaustive(t *testing.T) {
	e, _ := newClassificationEcho(t)
	registered := registeredAdminRoutes(t, e)
	if len(registered) == 0 {
		t.Fatal("/api/admin 配下のルートが 1 本も登録されていない。登録関数のシグネチャ変更を疑うこと")
	}

	seen := map[string]bool{}
	for _, key := range registered {
		seen[key] = true
		if _, ok := expectedAdminRouteClasses[key]; !ok {
			t.Errorf("分類されていないルート: %s（expectedAdminRouteClasses へ追加すること。迷ったら classPlatform(運用系)を既定にする）", key)
		}
	}
	for key := range expectedAdminRouteClasses {
		if !seen[key] {
			t.Errorf("分類表にあるが登録されていないルート: %s（ルートを消したなら表からも消すこと）", key)
		}
	}
}

type response struct {
	code    int
	body    string
	reached bool // 認可を通過してコントローラーまで到達した(nil コントローラーで panic した)
}

// passed は認可(とスコープ)で止まらなかったか。
func (r response) passed() bool {
	return r.reached || (r.code != http.StatusForbidden && r.code != http.StatusUnauthorized)
}

// cleared は認可・スコープのミドルウェアを通過したか。コントローラーが nil で panic した場合に加え、
// 入力検証で先に 400 を返すコントローラーもあるため、ミドルウェア由来のメッセージが無いことで判定する。
func (r response) cleared() bool {
	if r.reached {
		return true
	}
	for _, m := range []string{msgUnauthorized, msgPlatformOnly, msgAdminOnly, msgSchoolDenied, msgSchoolIDReq} {
		if strings.Contains(r.body, m) {
			return false
		}
	}
	return true
}

// doAs は p として method target を叩く。共有 mock を使うため並列実行しないこと。
// 「到達」とみなす panic は、コントローラー側で起きた runtime.Error の nil 参照だけ。
// それ以外の panic(ミドルウェア内など)はテスト失敗にする。
func doAs(t *testing.T, e *echo.Echo, mock sqlmock.Sqlmock, p principal, method, target string) response {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if !p.anon {
		mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\?").
			WithArgs(p.email, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "is_admin", "is_guest", "role"}).
				AddRow(p.id, p.email, "テスト", p.isAdmin, false, p.role))
		req.Header.Set("X-Admin-Email", p.email)
		req.Header.Set("X-Admin-Token", middleware.GenerateAdminToken(p.id, p.email, classificationAdminSecret))
	}
	rec := httptest.NewRecorder()
	reached := false
	func() {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			rerr, isRuntime := v.(runtime.Error)
			if isRuntime && strings.Contains(rerr.Error(), "nil pointer dereference") &&
				strings.Contains(string(debug.Stack()), "Backend/internal/controllers/") {
				reached = true
				return
			}
			t.Errorf("想定外の panic（nil コントローラー以外、またはミドルウェア内）: %v", v)
		}()
		e.ServeHTTP(rec, req)
	}()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock の期待が満たされていない: %v", err)
	}
	return response{code: rec.Code, body: rec.Body.String(), reached: reached}
}

var (
	pathParam          = regexp.MustCompile(`:[A-Za-z_]+`)
	subtestNameReplace = strings.NewReplacer("/", "_", " ", "_", "?", "_", "=", "_")
)

func concretePath(tmpl string) string {
	return "/api/admin" + pathParam.ReplaceAllString(tmpl, "1")
}

// expectedAuthz は class のルートを who が叩いたときの期待。
// 通過しない場合は期待する拒否コードと本文(拒否の発生元)も返す。
func expectedAuthz(class routeClass, who principal) (pass bool, code int, msg string) {
	switch {
	case who.anon:
		return false, http.StatusUnauthorized, msgUnauthorized
	case class == classPlatform && (!who.isAdmin || who.id == adminWithSchoolID):
		return false, http.StatusForbidden, msgPlatformOnly
	case class == classRequireAdmin && !who.isAdmin:
		return false, http.StatusForbidden, msgAdminOnly
	}
	return true, 0, ""
}

// 分類表どおりに実際の認可が効いていることを、全ルート×5主体で確認する。
// 運用系(/audit-logs, /costs, /scraper-sessions, /companies/:id/fetch-*, 学校メンバー操作など)の
// staff 403 もここで網羅される。拒否は本文で発生元(platform / requireAdmin / 認証)まで区別する。
func TestAdminRoutes_ClassificationMatchesAuthorization(t *testing.T) {
	e, mock := newClassificationEcho(t)

	for _, key := range registeredAdminRoutes(t, e) {
		class, ok := expectedAdminRouteClasses[key]
		if !ok {
			continue // 未分類は網羅性テストが報告する
		}
		method, tmpl, _ := strings.Cut(key, " ")
		for _, who := range allPrincipals {
			t.Run(string(class)+"_"+subtestNameReplace.Replace(key)+"_"+who.name, func(t *testing.T) {
				res := doAs(t, e, mock, who, method, concretePath(tmpl))
				wantPass, wantCode, wantMsg := expectedAuthz(class, who)
				if res.passed() != wantPass {
					t.Fatalf("分類=%s 主体=%s: 通過=%v(status=%d), want 通過=%v(拒否なら status=%d)",
						class, who.name, res.passed(), res.code, wantPass, wantCode)
				}
				if !wantPass && (res.code != wantCode || !strings.Contains(res.body, wantMsg)) {
					t.Errorf("分類=%s 主体=%s: 拒否 status=%d body=%q, want status=%d body に %q",
						class, who.name, res.code, res.body, wantCode, wantMsg)
				}
			})
		}
	}
}

// 未知パスも認証で止まる(包括ルートの認証が外れていない)。
func TestAdminRoutes_UnknownPathRequiresAuth(t *testing.T) {
	e, mock := newClassificationEcho(t)
	res := doAs(t, e, mock, pAnonymous, http.MethodGet, "/api/admin/__none__")
	if res.code != http.StatusUnauthorized {
		t.Errorf("未認証の未知パス status=%d, want %d", res.code, http.StatusUnauthorized)
	}
}

// 担当校スコープ(EchoAdminSchoolScope)の責務はミドルウェアで、schoolScopedRoutes のルートにだけ掛かる。
// 掛かっているルートは school_id を検証し、掛かっていない指導系ルートは素通しでコントローラーへ届く。
// 単一リソースの所属校検査はコントローラ側の責務で、各コントローラのテストが担う。
func TestAdminRoutes_SchoolScopeMiddleware(t *testing.T) {
	e, mock := newClassificationEcho(t)

	cases := []struct {
		who      principal
		query    string
		wantCode int    // スコープ付きルートでの期待。0 ならコントローラーまで到達
		wantMsg  string // 拒否時の本文
	}{
		{pStaffSchool, "?school_id=6", http.StatusForbidden, msgSchoolDenied},
		{pStaffSchool, "?school_id=5", 0, ""},
		{pStaffSchool, "", http.StatusBadRequest, msgSchoolIDReq},
		{pStaffNoSchool, "?school_id=5", http.StatusForbidden, msgSchoolDenied},
		{pStaffNoSchool, "", http.StatusBadRequest, msgSchoolIDReq},
		{pAdminSchool, "?school_id=6", http.StatusForbidden, msgSchoolDenied},
		{pAdminNoSchool, "?school_id=6", 0, ""},
		{pAdminNoSchool, "", 0, ""},
	}

	for _, key := range registeredAdminRoutes(t, e) {
		if c := expectedAdminRouteClasses[key]; c != classStaff && c != classStaffPendingReview {
			continue
		}
		method, tmpl, _ := strings.Cut(key, " ")
		scoped := schoolScopedRoutes[key]
		for _, sc := range cases {
			t.Run(subtestNameReplace.Replace(key)+"_"+sc.who.name+"_"+subtestNameReplace.Replace(sc.query), func(t *testing.T) {
				res := doAs(t, e, mock, sc.who, method, concretePath(tmpl)+sc.query)
				if !scoped || sc.wantCode == 0 {
					if !res.cleared() {
						t.Fatalf("スコープ付き=%v: ミドルウェアを通過するはずが status=%d body=%q", scoped, res.code, res.body)
					}
					return
				}
				if res.code != sc.wantCode || !strings.Contains(res.body, sc.wantMsg) {
					t.Errorf("スコープ付きルート: status=%d body=%q, want status=%d body に %q", res.code, res.body, sc.wantCode, sc.wantMsg)
				}
			})
		}
	}
}
