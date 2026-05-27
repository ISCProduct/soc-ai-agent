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

)

// ---- ESReviewController ----

func TestESReviewController_Review_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewESReviewController().Review, newCtx(req, rec), http.StatusBadRequest)
}

func TestESReviewController_Review_MissingRAGURL(t *testing.T) {
	t.Setenv("RAG_REVIEW_URL", "")
	body, _ := json.Marshal(map[string]string{
		"es_text":       "志望動機テスト文章",
		"question_type": "志望動機",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/es/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewESReviewController().Review, newCtx(req, rec), http.StatusServiceUnavailable)
}

// ---- ESRewriteController ----

func TestESRewriteController_Rewrite_MissingOriginalText(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"question_type": "志望動機"})
	req := httptest.NewRequest(http.MethodPost, "/api/es/rewrite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewESRewriteController(nil).Rewrite, newCtx(req, rec), http.StatusBadRequest)
}

// ---- CompanyRelationController ----

func TestCompanyRelationController_GetCompanyRelations_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc/relations", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewCompanyRelationController(nil, nil).GetCompanyRelations, ctx, http.StatusBadRequest)
}

func TestCompanyRelationController_GetCompanyMarketInfo_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/companies/abc/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewCompanyRelationController(nil, nil).GetCompanyMarketInfo, ctx, http.StatusBadRequest)
}

