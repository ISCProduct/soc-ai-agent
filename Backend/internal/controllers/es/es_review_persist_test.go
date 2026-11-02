package es

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/esreview"

	"github.com/labstack/echo/v4"
)

// esRepoSpy は保存されたものを記録する。
type esRepoSpy struct {
	created  []*models.ESReview
	previous *models.ESReview
	nextID   uint
}

func (s *esRepoSpy) Create(r *models.ESReview) error {
	s.nextID++
	r.ID = s.nextID
	s.created = append(s.created, r)
	return nil
}

func (s *esRepoSpy) FindPreviousByQuestion(userID uint, questionKey string, excludeID uint) (*models.ESReview, error) {
	return s.previous, nil
}

type scoreSpy struct{ calls int }

func (s *scoreSpy) UpdateScoresFromESReview(uint, *models.ESReview) error { s.calls++; return nil }

func newReviewCtxWithUser(body string, userID uint) (echo.Context, *httptest.ResponseRecorder) {
	ctx, rec := newCtx(http.MethodPost, "/api/es/review", body)
	if userID > 0 {
		r := ctx.Request()
		ctx.SetRequest(r.WithContext(context.WithValue(r.Context(), middleware.UserIDContextKey, userID)))
	}
	return ctx, rec
}

// ES添削は未ログインでも使える経路なので、実行者が取れないときは保存しない（#1531）。
// ここが崩れると、ゲストの添削が user_id=0 で溜まるか、保存時に落ちて添削自体が壊れる。
func TestReview_DoesNotPersistWithoutUser(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	repo := &esRepoSpy{}
	scores := &scoreSpy{}
	c := NewESReviewController(nil)
	c.SetReviewService(esreview.NewService(repo, scores))

	ctx, rec := newReviewCtxWithUser(`{"es_text":"文章","question":"設問","question_type":"ガクチカ"}`, 0)
	if err := c.Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("添削そのものは成功するべき: %d", rec.Code)
	}
	if len(repo.created) != 0 {
		t.Fatalf("未ログインなのに保存している: %d件", len(repo.created))
	}
	if scores.calls != 0 {
		t.Fatalf("未ログインなのにスコアを反映している: %d回", scores.calls)
	}
}

// ログイン中は保存し、RAG の評価をそのまま持つ。
func TestReview_PersistsWithUser(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	repo := &esRepoSpy{}
	scores := &scoreSpy{}
	c := NewESReviewController(nil)
	c.SetReviewService(esreview.NewService(repo, scores))

	ctx, _ := newReviewCtxWithUser(
		`{"es_text":"文章","question":"学生時代に 力を入れたことは？","question_type":"ガクチカ","company_name":"テスト社"}`, 42)
	if err := c.Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("保存されていない: %d件", len(repo.created))
	}
	got := repo.created[0]
	if got.UserID != 42 {
		t.Errorf("user_id = %d, want 42", got.UserID)
	}
	// ragOKBody の値がそのまま入ること。取り違えるとスコア連携まで狂う。
	if got.SpecificityScore != 7 || got.StarScore != 6 || got.LengthBalanceScore != 5 {
		t.Errorf("評価の取り込みが違う: %+v", got)
	}
	if got.CompanyFitScore != nil {
		t.Errorf("企業名ありでも RAG が null を返せば未評価のまま: %v", *got.CompanyFitScore)
	}
	if got.QuestionKey != models.NormalizeESQuestion("学生時代に 力を入れたことは？") {
		t.Errorf("設問キーが正規化されていない: %q", got.QuestionKey)
	}
	if scores.calls != 1 {
		t.Errorf("スコア反映が呼ばれていない: %d回", scores.calls)
	}
}

// 同一設問の前回結果を previous として本文へ足す。
func TestReview_AttachesPreviousResult(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	repo := &esRepoSpy{previous: &models.ESReview{ID: 9, SpecificityScore: 3, StarScore: 3, LengthBalanceScore: 3}}
	c := NewESReviewController(nil)
	c.SetReviewService(esreview.NewService(repo, &scoreSpy{}))

	ctx, rec := newReviewCtxWithUser(`{"es_text":"文章","question":"設問","question_type":"ガクチカ"}`, 42)
	if err := c.Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("応答が JSON ではない: %v", err)
	}
	prev, ok := body["previous"].(map[string]any)
	if !ok {
		t.Fatalf("previous が無い: %v", body)
	}
	if prev["specificity_score"] != float64(3) {
		t.Errorf("previous の値が違う: %v", prev)
	}
	// 既存の項目を壊していないこと（previous を足すために本文を組み直している）。
	if body["improved_text"] != "改善後の文章" {
		t.Errorf("本文の既存項目が壊れている: %v", body["improved_text"])
	}
}
