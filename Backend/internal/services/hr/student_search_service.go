package hr

import (
	"context"
	"errors"
	"strings"

	"Backend/internal/models"
	"Backend/internal/repositories"
)

// ErrInvalidTagName はタグ名が空、または長すぎるとき（400で返す）。
var ErrInvalidTagName = errors.New("invalid tag name")

// ErrEmptyQuery はセマンティック検索のクエリが空のとき（400で返す）。
var ErrEmptyQuery = errors.New("empty query")

// semanticCandidateLimit はRAGから受け取る候補件数の上限。
// 既存フィルタとAND結合した後に絞るため、DBフィルタで落ちる分を見込んで多めに取る。
const semanticCandidateLimit = 200

// StudentTagView は学生に付与された自社タグ。
type StudentTagView struct {
	ID      uint   `json:"id"`
	TagName string `json:"tag_name"`
}

// StudentListItem は学生一覧の1件（自社タグ付き）。
type StudentListItem struct {
	repositories.StudentSearchRow
	Tags []StudentTagView `json:"tags"`
}

// StudentSearchResult は一覧APIのレスポンス。
type StudentSearchResult struct {
	Items []StudentListItem `json:"items"`
	Total int64             `json:"total"`
}

type semanticSearcher interface {
	Query(ctx context.Context, query string, topK int) ([]uint, error)
}

// studentSearcher / studentTagStore は repositories の実装を差し替え可能にする
// （student_analysis_service.go と同じ、テスト用の狭いインターフェース）。
type studentSearcher interface {
	Search(companyID uint, f repositories.StudentSearchFilters) ([]repositories.StudentSearchRow, int64, error)
	IsVisible(userID uint) (bool, error)
}

type studentTagStore interface {
	Add(tag *models.CompanyStudentTag) error
	Delete(companyID, tagID uint) error
	ListByUser(companyID, userID uint) ([]models.CompanyStudentTag, error)
	ListByUsers(companyID uint, userIDs []uint) (map[uint][]models.CompanyStudentTag, error)
	ListTagNames(companyID uint) ([]string, error)
}

// StudentSearchService は企業向けの学生検索・タグ管理（#1094）。
// 公開範囲の判定は StudentSearchRepository に集約され、
// タグは company_id で完全に分離される。
type StudentSearchService struct {
	students studentSearcher
	tags     studentTagStore
	semantic semanticSearcher
}

func NewStudentSearchService(
	students studentSearcher,
	tags studentTagStore,
	semantic semanticSearcher,
) *StudentSearchService {
	return &StudentSearchService{students: students, tags: tags, semantic: semantic}
}

// Search はフィルタ条件で学生を絞り込み、自社タグを付けて返す。
func (s *StudentSearchService) Search(companyID uint, f repositories.StudentSearchFilters) (*StudentSearchResult, error) {
	rows, total, err := s.students.Search(companyID, f)
	if err != nil {
		return nil, err
	}
	items, err := s.attachTags(companyID, rows)
	if err != nil {
		return nil, err
	}
	return &StudentSearchResult{Items: items, Total: total}, nil
}

// SemanticSearch は自然文クエリで候補を取り、既存フィルタとAND結合して関連度順で返す。
func (s *StudentSearchService) SemanticSearch(
	ctx context.Context, companyID uint, query string, f repositories.StudentSearchFilters,
) (*StudentSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	if s.semantic == nil {
		return nil, ErrSemanticSearchUnavailable
	}
	rankedIDs, err := s.semantic.Query(ctx, query, semanticCandidateLimit)
	if err != nil {
		return nil, err
	}
	if rankedIDs == nil {
		// 空スライスで「候補0件」を表す。nil は「セマンティック検索なし」の意味になるため区別する。
		rankedIDs = []uint{}
	}

	// 公開同意・既存フィルタは Search が一括で適用する（ID指定はAND結合）。
	f.UserIDs = rankedIDs
	f.Limit = maxStudentSemanticResults
	f.Offset = 0
	rows, _, err := s.students.Search(companyID, f)
	if err != nil {
		return nil, err
	}

	// RAGが返した関連度順へ並べ替える（Search は id DESC で返すため）。
	byID := make(map[uint]repositories.StudentSearchRow, len(rows))
	for _, r := range rows {
		byID[r.UserID] = r
	}
	ordered := make([]repositories.StudentSearchRow, 0, len(rows))
	for _, id := range rankedIDs {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
			delete(byID, id)
		}
	}

	items, err := s.attachTags(companyID, ordered)
	if err != nil {
		return nil, err
	}
	return &StudentSearchResult{Items: items, Total: int64(len(items))}, nil
}

const maxStudentSemanticResults = 100

// IsVisible は詳細取得前の公開同意チェック。
func (s *StudentSearchService) IsVisible(userID uint) (bool, error) {
	return s.students.IsVisible(userID)
}

// AddTag は学生に自社タグを付与する。
func (s *StudentSearchService) AddTag(companyID, companyUserID, userID uint, tagName string) error {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" || len([]rune(tagName)) > models.MaxTagNameLength {
		return ErrInvalidTagName
	}
	visible, err := s.students.IsVisible(userID)
	if err != nil {
		return err
	}
	if !visible {
		return ErrStudentNotVisible
	}
	return s.tags.Add(&models.CompanyStudentTag{
		CompanyID: companyID,
		UserID:    userID,
		TagName:   tagName,
		CreatedBy: companyUserID,
	})
}

// RemoveTag は自社タグを削除する。
func (s *StudentSearchService) RemoveTag(companyID, tagID uint) error {
	return s.tags.Delete(companyID, tagID)
}

// ListTagNames は自社で使用中のタグ名一覧を返す。
func (s *StudentSearchService) ListTagNames(companyID uint) ([]string, error) {
	return s.tags.ListTagNames(companyID)
}

// ListTagsForUser は学生詳細に表示する自社タグを返す。
func (s *StudentSearchService) ListTagsForUser(companyID, userID uint) ([]StudentTagView, error) {
	tags, err := s.tags.ListByUser(companyID, userID)
	if err != nil {
		return nil, err
	}
	return toTagViews(tags), nil
}

func (s *StudentSearchService) attachTags(companyID uint, rows []repositories.StudentSearchRow) ([]StudentListItem, error) {
	items := make([]StudentListItem, 0, len(rows))
	if len(rows) == 0 {
		return items, nil
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserID)
	}
	tagsByUser, err := s.tags.ListByUsers(companyID, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		items = append(items, StudentListItem{
			StudentSearchRow: r,
			Tags:             toTagViews(tagsByUser[r.UserID]),
		})
	}
	return items, nil
}

func toTagViews(tags []models.CompanyStudentTag) []StudentTagView {
	out := make([]StudentTagView, 0, len(tags))
	for _, t := range tags {
		out = append(out, StudentTagView{ID: t.ID, TagName: t.TagName})
	}
	return out
}
