package controllers_test

// #1027 教員向け生徒傾向分析の認可テスト。
//
// このエンドポイントは生徒の氏名・メール・分析結果を返すため、
// 担当校の絞り込みが外れると「教員が全校の生徒を閲覧できる」重大インシデントになる。
// 絞り込みがサービス層まで正しく届くことをここで固定する。
//
// 実行: cd Backend && go test ./test/controllers/... -run TeacherStudentInsight -v

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/teacher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 担当校がサービスへ渡ったかを記録するスタブ。
type schoolScopeSpy struct {
	gotSchoolID *uint
	called      bool
}

func (s *schoolScopeSpy) ListStudentsPaged(limit, offset int, query string, schoolID *uint) ([]entity.User, int64, error) {
	s.called = true
	s.gotSchoolID = schoolID
	return []entity.User{{ID: 1, Name: "生徒A", Email: "a@example.com"}}, 1, nil
}

type noopScores struct{}

func (noopScores) FindLatestScoresByUsers(_ []uint) (map[uint]map[string]float64, error) {
	return map[uint]map[string]float64{}, nil
}

type noopIndustries struct{}

func (noopIndustries) ListActive() ([]repositories.IndustryOption, error) { return nil, nil }

type noopProfiles struct{}

func (noopProfiles) ListAll() ([]models.IndustryWeightProfile, error) { return nil, nil }

func newInsightController(lister teacher.StudentLister) *controllers.TeacherStudentInsightController {
	return controllers.NewTeacherStudentInsightController(
		teacher.NewStudentInsightService(lister, noopScores{}, noopIndustries{}, noopProfiles{}),
	)
}

// withSchoolFilter は EchoAdminSchoolScope が入れる値を再現する。
func withSchoolFilter(r *http.Request, schoolID *uint) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.AdminSchoolFilterContextKey, schoolID))
}

// TestTeacherStudentInsight_PassesSchoolScopeToService は、
// 担当校の絞り込みがサービス層まで届くことを検証する。
//
// ここが nil になると全校の生徒が返る。コントローラが
// AdminSchoolFilterFromContext の戻り値を無視していれば必ず落ちる。
func TestTeacherStudentInsight_PassesSchoolScopeToService(t *testing.T) {
	schoolID := uint(7)
	spy := &schoolScopeSpy{}

	req := withSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/teacher/students/tendency-analysis", nil), &schoolID)
	rec := httptest.NewRecorder()
	assertStatus(t, newInsightController(spy).TendencyAnalysis, newCtx(req, rec), http.StatusOK)

	require.True(t, spy.called, "サービスが呼ばれていない")
	require.NotNil(t, spy.gotSchoolID, "担当校が渡っていない。全校の生徒が返る状態")
	assert.Equal(t, uint(7), *spy.gotSchoolID)
}

// 担当校を持たないシステム管理者は nil（絞り込みなし）で渡る。
func TestTeacherStudentInsight_UnrestrictedAdminGetsNilScope(t *testing.T) {
	spy := &schoolScopeSpy{}

	req := withSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/teacher/students/tendency-analysis", nil), nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newInsightController(spy).TendencyAnalysis, newCtx(req, rec), http.StatusOK)

	require.True(t, spy.called)
	assert.Nil(t, spy.gotSchoolID)
}

// TestTeacherStudentInsight_FailsClosedWithoutScope は、
// ミドルウェアを通っていない場合に拒否することを検証する（fail-close）。
//
// ルート定義から schoolScope が外れると、コンテキストに値が入らない。
// そのまま nil を渡すと全校開放になるため、403 で止める。
func TestTeacherStudentInsight_FailsClosedWithoutScope(t *testing.T) {
	spy := &schoolScopeSpy{}

	// AdminSchoolFilterContextKey を入れずに呼ぶ = ミドルウェア未経由。
	req := httptest.NewRequest(http.MethodGet, "/api/admin/teacher/students/tendency-analysis", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newInsightController(spy).TendencyAnalysis, newCtx(req, rec), http.StatusForbidden)

	assert.False(t, spy.called, "スコープ未解決なのに生徒一覧を引いている")
}

// クエリのページング上限が効くこと。
func TestTeacherStudentInsight_ClampsPaging(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		wantLimit, wantOff int
	}{
		{name: "既定", url: "/x", wantLimit: 25, wantOff: 0},
		{name: "limit上限を超えたら既定", url: "/x?limit=500", wantLimit: 25, wantOff: 0},
		{name: "offset上限で丸める", url: "/x?offset=999999", wantLimit: 25, wantOff: 10000},
		{name: "正常値", url: "/x?limit=50&offset=100", wantLimit: 50, wantOff: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotLimit, gotOffset int
			lister := &pagingSpy{onCall: func(l, o int) { gotLimit, gotOffset = l, o }}
			req := withSchoolFilter(httptest.NewRequest(http.MethodGet, tt.url, nil), nil)
			rec := httptest.NewRecorder()
			assertStatus(t, newInsightController(lister).TendencyAnalysis, newCtx(req, rec), http.StatusOK)
			assert.Equal(t, tt.wantLimit, gotLimit)
			assert.Equal(t, tt.wantOff, gotOffset)
		})
	}
}

type pagingSpy struct{ onCall func(limit, offset int) }

func (s *pagingSpy) ListStudentsPaged(limit, offset int, _ string, _ *uint) ([]entity.User, int64, error) {
	s.onCall(limit, offset)
	return nil, 0, nil
}

// レスポンス形が壊れていないこと（フロントの型と一致する）。
func TestTeacherStudentInsight_ResponseShape(t *testing.T) {
	spy := &schoolScopeSpy{}
	req := withSchoolFilter(httptest.NewRequest(http.MethodGet, "/x", nil), nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newInsightController(spy).TendencyAnalysis, newCtx(req, rec), http.StatusOK)

	var got teacher.TendencyResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Students, 1)
	assert.Equal(t, uint(1), got.Students[0].UserID)
	// スコアが無いのでデータ不足として返る。
	assert.False(t, got.Students[0].DataAvailable)
	assert.Equal(t, "分析データ不足", got.Students[0].TypeLabel)
}
