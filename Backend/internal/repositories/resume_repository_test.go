package repositories

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newResumeMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	return db, mock
}

// TestFindLatestDocumentWithReview_OrdersByNewest は「最新」の定義（#1030 境界値）を検証する。
// 最新ドキュメントを作成日時降順で1件選び、そのドキュメントに紐づくレビューをさらに降順で1件選ぶ。
func TestFindLatestDocumentWithReview_OrdersByNewest(t *testing.T) {
	db, mock := newResumeMockDB(t)
	repo := NewResumeRepository(db)

	// 1本目: user_id で絞り、created_at DESC, id DESC の順に1件。
	mock.ExpectQuery("SELECT.*resume_documents.*user_id.*ORDER BY created_at DESC, id DESC.*LIMIT").
		WithArgs(uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(20, 7))

	// 2本目: 直前で選んだ document_id に紐づくレビューを同じ順序で1件。
	mock.ExpectQuery("SELECT.*resume_reviews.*document_id.*ORDER BY created_at DESC, id DESC.*LIMIT").
		WithArgs(uint(20), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "document_id", "score"}).AddRow(99, 20, 45))

	doc, review, err := repo.FindLatestDocumentWithReview(7)
	require.NoError(t, err)
	require.NotNil(t, doc)
	require.NotNil(t, review)
	require.Equal(t, uint(20), doc.ID, "最新のドキュメントが選ばれること")
	require.Equal(t, uint(99), review.ID, "そのドキュメントの最新レビューが選ばれること")
	require.Equal(t, 45, review.Score)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestFindLatestDocumentWithReview_NoDocument は未提出のユーザーで (nil, nil, nil) が返ることを検証する。
// エラーにしないのは、未提出が正常な状態だから。
func TestFindLatestDocumentWithReview_NoDocument(t *testing.T) {
	db, mock := newResumeMockDB(t)
	repo := NewResumeRepository(db)

	mock.ExpectQuery("SELECT.*resume_documents").
		WillReturnError(gorm.ErrRecordNotFound)

	doc, review, err := repo.FindLatestDocumentWithReview(7)
	require.NoError(t, err)
	require.Nil(t, doc)
	require.Nil(t, review)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestFindLatestDocumentWithReview_DocumentWithoutReview はレビュー未生成（処理中）の状態を検証する。
// 未提出と区別する必要があるため、ドキュメントだけ返る。
func TestFindLatestDocumentWithReview_DocumentWithoutReview(t *testing.T) {
	db, mock := newResumeMockDB(t)
	repo := NewResumeRepository(db)

	mock.ExpectQuery("SELECT.*resume_documents").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(20, 7))
	mock.ExpectQuery("SELECT.*resume_reviews").
		WillReturnError(gorm.ErrRecordNotFound)

	doc, review, err := repo.FindLatestDocumentWithReview(7)
	require.NoError(t, err)
	require.NotNil(t, doc)
	require.Nil(t, review, "レビュー未生成なら nil を返し、未提出と区別できること")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestFindLatestDocumentWithReview_PropagatesError はDBエラーを握り潰さないことを検証する。
func TestFindLatestDocumentWithReview_PropagatesError(t *testing.T) {
	db, mock := newResumeMockDB(t)
	repo := NewResumeRepository(db)

	wantErr := errors.New("connection refused")
	mock.ExpectQuery("SELECT.*resume_documents").WillReturnError(wantErr)

	_, _, err := repo.FindLatestDocumentWithReview(7)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
