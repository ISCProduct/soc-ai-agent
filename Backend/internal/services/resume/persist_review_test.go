package resume

// 履歴書レビューの保存のテスト（#1332）。
// 実行: cd Backend && go test ./internal/services/resume/ -run PersistReview -v
//
// ストリーム経路(フロントが使っている側)がレビューを保存しておらず、
// resume_documents に status='reviewed' の行はあるのに resume_reviews は
// 0件という状態になっていた。学生は画面を閉じるとレビューを二度と見られず、
// スコアが無いので EvaluateResumeStatus は永久に「要対応」と判定し、
// 履歴書スコアが user_weight_scores に載らずフライホイールも回らない。

import (
	"errors"
	"os"
	"strings"
	"testing"

	"Backend/internal/models"
)

type fakeResumeRepo struct {
	createdReview  *models.ResumeReview
	replacedItems  []models.ResumeReviewItem
	replacedFor    uint
	updatedDoc     *models.ResumeDocument
	createErr      error
	replaceErr     error
	updateDocCalls int
}

func (r *fakeResumeRepo) CreateReview(review *models.ResumeReview) error {
	if r.createErr != nil {
		return r.createErr
	}
	review.ID = 42 // DBが採番した体にする
	r.createdReview = review
	return nil
}

func (r *fakeResumeRepo) ReplaceReviewItems(reviewID uint, items []models.ResumeReviewItem) error {
	if r.replaceErr != nil {
		return r.replaceErr
	}
	r.replacedFor = reviewID
	r.replacedItems = items
	return nil
}

func (r *fakeResumeRepo) UpdateDocument(doc *models.ResumeDocument) error {
	r.updateDocCalls++
	r.updatedDoc = doc
	return nil
}

// 以下は persistReview から呼ばれない。
func (r *fakeResumeRepo) CreateDocument(*models.ResumeDocument) error { return nil }
func (r *fakeResumeRepo) FindDocumentByID(uint) (*models.ResumeDocument, error) {
	return nil, nil
}
func (r *fakeResumeRepo) FindDocumentByIDForUser(uint, uint) (*models.ResumeDocument, error) {
	return nil, nil
}
func (r *fakeResumeRepo) ReplaceTextBlocks(uint, []models.ResumeTextBlock) error { return nil }
func (r *fakeResumeRepo) FindTextBlocks(uint) ([]models.ResumeTextBlock, error) {
	return nil, nil
}
func (r *fakeResumeRepo) FindReviewItems(uint) ([]models.ResumeReviewItem, error) {
	return nil, nil
}
func (r *fakeResumeRepo) FindLatestDocumentWithReview(uint) (*models.ResumeDocument, *models.ResumeReview, error) {
	return nil, nil, nil
}

func TestPersistReview_レビューと指摘項目を保存する(t *testing.T) {
	repo := &fakeResumeRepo{}
	s := &ResumeService{repo: repo} // crossFeature は nil（連携は別途 nil ガード済み）

	doc := &models.ResumeDocument{ID: 7, UserID: 8, SessionID: "sess-1"}
	review := &models.ResumeReview{Score: 80, Summary: "よくまとまっています"}
	items := []models.ResumeReviewItem{{Message: "志望動機が薄い"}, {Message: "数値が無い"}}

	if err := s.persistReview(doc, review, items); err != nil {
		t.Fatalf("保存に失敗: %v", err)
	}

	if repo.createdReview == nil {
		t.Fatal("CreateReview が呼ばれていない。これが欠けると resume_reviews が0件のままになる")
	}
	// ドキュメントに紐づかないレビューは後から引けない。
	if repo.createdReview.DocumentID != doc.ID {
		t.Errorf("DocumentID が紐づいていない: %d", repo.createdReview.DocumentID)
	}
	if repo.replacedFor != review.ID {
		t.Errorf("指摘項目が別のレビューに紐づいている: %d (review=%d)", repo.replacedFor, review.ID)
	}
	if len(repo.replacedItems) != len(items) {
		t.Fatalf("指摘項目の件数が違う: %d", len(repo.replacedItems))
	}
	for i, it := range repo.replacedItems {
		if it.ReviewID != review.ID {
			t.Errorf("items[%d].ReviewID が設定されていない: %d", i, it.ReviewID)
		}
	}
}

func TestPersistReview_保存に失敗したらエラーを返す(t *testing.T) {
	wantErr := errors.New("db down")

	tests := []struct {
		name string
		repo *fakeResumeRepo
	}{
		// 握り潰すと「レビューはあるはずなのに0件」に逆戻りする。
		{"CreateReview が失敗", &fakeResumeRepo{createErr: wantErr}},
		{"ReplaceReviewItems が失敗", &fakeResumeRepo{replaceErr: wantErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ResumeService{repo: tt.repo}
			doc := &models.ResumeDocument{ID: 7, UserID: 8}
			err := s.persistReview(doc, &models.ResumeReview{}, []models.ResumeReviewItem{{}})

			if !errors.Is(err, wantErr) {
				t.Errorf("エラーが返っていない: %v", err)
			}
			// 保存に失敗した以上 status を進めてはいけない。
			if tt.repo.updateDocCalls != 0 {
				t.Errorf("保存失敗なのに UpdateDocument が呼ばれた: %d回", tt.repo.updateDocCalls)
			}
		})
	}
}

func TestPersistReview_crossFeatureがnilでも落ちない(t *testing.T) {
	// 連携未設定の環境(テスト・一部の起動構成)でレビュー保存まで巻き添えにしない。
	repo := &fakeResumeRepo{}
	s := &ResumeService{repo: repo}

	err := s.persistReview(&models.ResumeDocument{ID: 1}, &models.ResumeReview{}, nil)
	if err != nil {
		t.Errorf("crossFeature が nil でも保存は成功すべき: %v", err)
	}
	if repo.createdReview == nil {
		t.Error("レビューが保存されていない")
	}
}

// レビューを生成する経路が、必ず保存を通ることを固定する。
//
// 元の不具合は「通常経路には保存があり、ストリーム経路には無い」という
// 片側だけの実装だった。関数が増えたときに同じ形で漏れるため、
// 生成呼び出しと保存呼び出しの数が釣り合っていることを検査する。
func Testレビュー生成経路は必ず保存を通る(t *testing.T) {
	src, err := os.ReadFile("resume_review.go")
	if err != nil {
		t.Fatalf("読めない: %v", err)
	}
	text := string(src)

	// レビューを組み立てる呼び出し（定義行・コメント行は除く）
	generators := 0
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "func ") {
			continue
		}
		// buildResumeReviewWithAI は内部で buildReviewScoreItems を呼ぶ委譲なので
		// 二重に数えない。呼び出し元(= 保存が必要な地点)だけを数える。
		if strings.Contains(t, "s.buildResumeReviewWithAI(") || strings.Contains(t, "s.buildReviewScoreItems(") {
			if strings.Contains(t, "return s.buildReviewScoreItems(") {
				continue // 委譲
			}
			generators++
		}
	}

	persists := strings.Count(text, "s.persistReview(")

	if generators == 0 {
		t.Fatal("レビュー生成の呼び出しが見つからない。テストの前提が古い")
	}
	if persists < generators {
		t.Errorf("レビューを生成して保存していない経路がある: 生成 %d 箇所 / 保存 %d 箇所", generators, persists)
	}
}
