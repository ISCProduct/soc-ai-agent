package resume

import (
	"errors"
	"testing"

	"Backend/internal/models"
)

// persistRepoStub は保存呼び出しを記録する最小スタブ。
type persistRepoStub struct {
	createdReview *models.ResumeReview
	replacedItems []models.ResumeReviewItem
	replacedForID uint
	updatedDoc    *models.ResumeDocument
	createErr     error
	replaceErr    error
	nextReviewID  uint
}

func (s *persistRepoStub) CreateDocument(*models.ResumeDocument) error { return nil }
func (s *persistRepoStub) UpdateDocument(d *models.ResumeDocument) error {
	copied := *d
	s.updatedDoc = &copied
	return nil
}
func (s *persistRepoStub) FindDocumentByID(uint) (*models.ResumeDocument, error) { return nil, nil }
func (s *persistRepoStub) FindDocumentByIDForUser(uint, uint) (*models.ResumeDocument, error) {
	return nil, nil
}
func (s *persistRepoStub) ReplaceTextBlocks(uint, []models.ResumeTextBlock) error { return nil }
func (s *persistRepoStub) FindTextBlocks(uint) ([]models.ResumeTextBlock, error)  { return nil, nil }
func (s *persistRepoStub) CreateReview(r *models.ResumeReview) error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.nextReviewID == 0 {
		s.nextReviewID = 77
	}
	r.ID = s.nextReviewID
	s.createdReview = r
	return nil
}
func (s *persistRepoStub) ReplaceReviewItems(reviewID uint, items []models.ResumeReviewItem) error {
	if s.replaceErr != nil {
		return s.replaceErr
	}
	s.replacedForID = reviewID
	s.replacedItems = items
	return nil
}
func (s *persistRepoStub) FindReviewItems(uint) ([]models.ResumeReviewItem, error) { return nil, nil }
func (s *persistRepoStub) FindLatestDocumentWithReview(uint) (*models.ResumeDocument, *models.ResumeReview, error) {
	return nil, nil, nil
}

// TestPersistReview_SavesReviewAndItems は #1332 の回帰テスト。
//
// ストリーム経路はレビューを画面へ流すだけで保存していなかった。
// 学生は二度と見られず、latest_score が nil のままなので「要対応」から
// 外れず、履歴書スコアがマッチングにも反映されなかった。
func TestPersistReview_SavesReviewAndItems(t *testing.T) {
	repo := &persistRepoStub{}
	svc := &ResumeService{repo: repo}
	doc := &models.ResumeDocument{ID: 42, UserID: 8, SessionID: "s1"}
	review := &models.ResumeReview{Score: 73, Summary: "全体として具体性が高い"}
	items := []models.ResumeReviewItem{{Message: "数値を添える"}, {Message: "主語を明確に"}}

	if err := svc.persistReview(doc, review, items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.createdReview == nil {
		t.Fatal("レビューが保存されていない")
	}
	if repo.createdReview.DocumentID != doc.ID {
		t.Errorf("DocumentID = %d, want %d", repo.createdReview.DocumentID, doc.ID)
	}
	if repo.createdReview.Score != 73 {
		t.Errorf("Score = %d, want 73", repo.createdReview.Score)
	}
	if len(repo.replacedItems) != 2 {
		t.Fatalf("指摘事項の保存件数 = %d, want 2", len(repo.replacedItems))
	}
	if repo.replacedForID != repo.createdReview.ID {
		t.Errorf("指摘事項が別のレビューに紐づいている: %d != %d", repo.replacedForID, repo.createdReview.ID)
	}
	// 採番されたレビューIDが各指摘に入らないと、後から引けない。
	for i, it := range repo.replacedItems {
		if it.ReviewID != repo.createdReview.ID {
			t.Errorf("items[%d].ReviewID = %d, want %d", i, it.ReviewID, repo.createdReview.ID)
		}
	}
}

// TestPersistReview_StatusFollowsPersistence は #1332 の核心を固定する。
//
// 「reviewed」は「レビューが引ける」ことを意味しなければならない。
// 保存せずに reviewed にすると、レビューが無いのに要対応から外れ、
// 学生にも教員にも見えない状態になる（resume_status.go の判定は
// latest_score が nil かどうかで決まる）。
func TestPersistReview_StatusFollowsPersistence(t *testing.T) {
	t.Run("保存できたら reviewed へ進む", func(t *testing.T) {
		svc := &ResumeService{repo: &persistRepoStub{}}
		doc := &models.ResumeDocument{ID: 1, Status: "normalized"}

		if err := svc.persistReview(doc, &models.ResumeReview{}, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Status != "reviewed" {
			t.Errorf("Status = %q, want reviewed", doc.Status)
		}
	})

	t.Run("保存に失敗したら reviewed にしない", func(t *testing.T) {
		svc := &ResumeService{repo: &persistRepoStub{createErr: errors.New("db down")}}
		doc := &models.ResumeDocument{ID: 1, Status: "normalized"}

		if err := svc.persistReview(doc, &models.ResumeReview{}, nil); err == nil {
			t.Fatal("error = nil, want error")
		}
		if doc.Status == "reviewed" {
			t.Error("保存できていないのに reviewed にしている（レビューが無いのに要対応から外れる）")
		}
	})

	t.Run("指摘事項の保存に失敗しても reviewed にしない", func(t *testing.T) {
		svc := &ResumeService{repo: &persistRepoStub{replaceErr: errors.New("db down")}}
		doc := &models.ResumeDocument{ID: 1, Status: "normalized"}

		if err := svc.persistReview(doc, &models.ResumeReview{}, nil); err == nil {
			t.Fatal("error = nil, want error")
		}
		if doc.Status == "reviewed" {
			t.Error("指摘事項が保存できていないのに reviewed にしている")
		}
	})
}

// レビュー本体の保存に失敗したら、指摘事項を書きに行かずエラーを返す。
// 中途半端に指摘だけ残すと、親のないレビュー項目ができる。
func TestPersistReview_CreateFailureStops(t *testing.T) {
	repo := &persistRepoStub{createErr: errors.New("db down")}
	svc := &ResumeService{repo: repo}

	err := svc.persistReview(&models.ResumeDocument{ID: 1}, &models.ResumeReview{}, []models.ResumeReviewItem{{}})
	if err == nil {
		t.Fatal("error = nil, want error")
	}
	if repo.replacedItems != nil {
		t.Error("レビュー保存に失敗したのに指摘事項を書いている")
	}
}

// 指摘事項の保存に失敗したらエラーを返す。
func TestPersistReview_ReplaceItemsFailure(t *testing.T) {
	repo := &persistRepoStub{replaceErr: errors.New("db down")}
	svc := &ResumeService{repo: repo}

	if err := svc.persistReview(&models.ResumeDocument{ID: 1}, &models.ResumeReview{}, nil); err == nil {
		t.Fatal("error = nil, want error")
	}
}

// crossFeature 未注入でも保存は成立する（オプション依存）。
func TestPersistReview_WithoutCrossFeature(t *testing.T) {
	repo := &persistRepoStub{}
	svc := &ResumeService{repo: repo}

	if err := svc.persistReview(&models.ResumeDocument{ID: 1}, &models.ResumeReview{}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createdReview == nil {
		t.Error("レビューが保存されていない")
	}
}
