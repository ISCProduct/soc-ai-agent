package controllers_test

// バリデーションパスのテスト (Issue #422)
// ESReview, ESRewrite, GitHub, CompanyRelation コントローラーの入力検証テスト
//
// 実行: cd Backend && go test ./test/controllers/... -run "ESReview|ESRewrite|GitHub|CompanyRelation" -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"

	"github.com/stretchr/testify/assert"
)

// ---- ESReviewController ----

func TestESReviewController_Review_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/es/review", nil)
			w := httptest.NewRecorder()
			controllers.NewESReviewController().Review(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestESReviewController_Review_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewESReviewController().Review(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestESReviewController_Review_MissingESText(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"question_type": "志望動機"})
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewESReviewController().Review(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestESReviewController_Review_MissingRAGURL(t *testing.T) {
	t.Setenv("RAG_REVIEW_URL", "")
	body, _ := json.Marshal(map[string]string{
		"es_text":       "志望動機テスト文章",
		"question_type": "志望動機",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewESReviewController().Review(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ---- ESRewriteController ----

func TestESRewriteController_Rewrite_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/es/rewrite", nil)
			w := httptest.NewRecorder()
			controllers.NewESRewriteController(nil).Rewrite(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestESRewriteController_Rewrite_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/es/rewrite", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewESRewriteController(nil).Rewrite(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestESRewriteController_Rewrite_MissingOriginalText(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"question_type": "志望動機"})
	req := httptest.NewRequest(http.MethodPost, "/api/es/rewrite", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewESRewriteController(nil).Rewrite(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- GitHubController (バリデーション・認証パス) ----

func TestGitHubController_GetProfile_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/profile", nil)
	w := httptest.NewRecorder()
	controllers.NewGitHubController(nil, nil).GetProfile(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGitHubController_GetProfile_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/profile", nil)
	w := httptest.NewRecorder()
	controllers.NewGitHubController(nil, nil).GetProfile(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGitHubController_Sync_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/sync", nil)
	w := httptest.NewRecorder()
	controllers.NewGitHubController(nil, nil).Sync(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGitHubController_Sync_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync", nil)
	w := httptest.NewRecorder()
	controllers.NewGitHubController(nil, nil).Sync(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGitHubController_GetSkills_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/skills", nil)
	w := httptest.NewRecorder()
	controllers.NewGitHubController(nil, nil).GetSkills(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGitHubController_GetSkills_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/skills", nil)
	w := httptest.NewRecorder()
	controllers.NewGitHubController(nil, nil).GetSkills(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---- CompanyRelationController ----

func TestCompanyRelationController_GetCompanyRelations_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc/relations", nil)
	req.URL.Path = "/api/companies/abc/relations"
	w := httptest.NewRecorder()
	controllers.NewCompanyRelationController(nil, nil).GetCompanyRelations(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompanyRelationController_GetCompanyMarketInfo_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc/market-info", nil)
	req.URL.Path = "/api/companies/abc/market-info"
	w := httptest.NewRecorder()
	controllers.NewCompanyRelationController(nil, nil).GetCompanyMarketInfo(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
