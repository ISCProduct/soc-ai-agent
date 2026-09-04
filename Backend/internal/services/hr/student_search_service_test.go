package hr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubStudentSearcher struct {
	rows        []repositories.StudentSearchRow
	total       int64
	err         error
	visible     bool
	visibleErr  error
	lastFilters repositories.StudentSearchFilters
	lastCompany uint
}

func (s *stubStudentSearcher) Search(companyID uint, f repositories.StudentSearchFilters) ([]repositories.StudentSearchRow, int64, error) {
	s.lastCompany = companyID
	s.lastFilters = f
	if s.err != nil {
		return nil, 0, s.err
	}
	// UserIDs が指定されていれば、その集合に含まれる行だけを返す（DBのIN条件を模す）。
	if f.UserIDs != nil {
		allowed := map[uint]bool{}
		for _, id := range f.UserIDs {
			allowed[id] = true
		}
		out := []repositories.StudentSearchRow{}
		for _, r := range s.rows {
			if allowed[r.UserID] {
				out = append(out, r)
			}
		}
		return out, int64(len(out)), nil
	}
	return s.rows, s.total, nil
}

func (s *stubStudentSearcher) IsVisible(uint) (bool, error) { return s.visible, s.visibleErr }

type stubTagStore struct {
	added    []models.CompanyStudentTag
	byUser   map[uint][]models.CompanyStudentTag
	names    []string
	deleted  [][2]uint
	addErr   error
	listErr  error
	nameErr  error
	usersErr error
}

func (s *stubTagStore) Add(tag *models.CompanyStudentTag) error {
	if s.addErr != nil {
		return s.addErr
	}
	s.added = append(s.added, *tag)
	return nil
}

func (s *stubTagStore) Delete(companyID, tagID uint) error {
	s.deleted = append(s.deleted, [2]uint{companyID, tagID})
	return nil
}

func (s *stubTagStore) ListByUser(_, userID uint) ([]models.CompanyStudentTag, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.byUser[userID], nil
}

func (s *stubTagStore) ListByUsers(_ uint, userIDs []uint) (map[uint][]models.CompanyStudentTag, error) {
	if s.usersErr != nil {
		return nil, s.usersErr
	}
	out := map[uint][]models.CompanyStudentTag{}
	for _, id := range userIDs {
		if tags, ok := s.byUser[id]; ok {
			out[id] = tags
		}
	}
	return out, nil
}

func (s *stubTagStore) ListTagNames(uint) ([]string, error) {
	if s.nameErr != nil {
		return nil, s.nameErr
	}
	return s.names, nil
}

type stubSemantic struct {
	ids      []uint
	err      error
	lastTopK int
	called   bool
}

func (s *stubSemantic) Query(_ context.Context, _ string, topK int) ([]uint, error) {
	s.called = true
	s.lastTopK = topK
	return s.ids, s.err
}

func rows(ids ...uint) []repositories.StudentSearchRow {
	out := make([]repositories.StudentSearchRow, 0, len(ids))
	for _, id := range ids {
		out = append(out, repositories.StudentSearchRow{UserID: id})
	}
	return out
}

func TestStudentSearchService_Search_AttachesOwnCompanyTags(t *testing.T) {
	students := &stubStudentSearcher{rows: rows(1, 2), total: 2}
	tags := &stubTagStore{byUser: map[uint][]models.CompanyStudentTag{
		1: {{ID: 10, TagName: "即戦力"}},
	}}
	svc := NewStudentSearchService(students, tags, &stubSemantic{})

	got, err := svc.Search(7, repositories.StudentSearchFilters{Limit: 30})
	require.NoError(t, err)

	assert.Equal(t, uint(7), students.lastCompany, "自社IDでスコープされる")
	require.Len(t, got.Items, 2)
	assert.Equal(t, int64(2), got.Total)
	require.Len(t, got.Items[0].Tags, 1)
	assert.Equal(t, "即戦力", got.Items[0].Tags[0].TagName)
	assert.Empty(t, got.Items[1].Tags, "タグ未付与の学生は空配列")
}

func TestStudentSearchService_SemanticSearch(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		semanticIDs []uint
		semanticErr error
		dbRows      []repositories.StudentSearchRow
		wantIDs     []uint
		wantErr     error
	}{
		{
			name:        "RAGの関連度順を維持する",
			query:       "リーダーシップ経験があってReactができる学生",
			semanticIDs: []uint{5, 2, 9},
			dbRows:      rows(2, 5, 9),
			wantIDs:     []uint{5, 2, 9},
		},
		{
			name:        "フィルタで落ちた学生は結果から除外される",
			query:       "React",
			semanticIDs: []uint{5, 2, 9},
			dbRows:      rows(2, 9), // 5 はDBフィルタ(公開同意/業界等)に一致しない
			wantIDs:     []uint{2, 9},
		},
		{
			name:        "候補0件なら空の結果を返す",
			query:       "該当なし",
			semanticIDs: []uint{},
			dbRows:      rows(1, 2),
			wantIDs:     []uint{},
		},
		{
			name:    "空クエリは400相当のエラー",
			query:   "   ",
			wantErr: ErrEmptyQuery,
		},
		{
			name:        "RAG不通はそのまま伝播する",
			query:       "React",
			semanticErr: ErrSemanticSearchUnavailable,
			wantErr:     ErrSemanticSearchUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			students := &stubStudentSearcher{rows: tt.dbRows}
			semantic := &stubSemantic{ids: tt.semanticIDs, err: tt.semanticErr}
			svc := NewStudentSearchService(students, &stubTagStore{}, semantic)

			got, err := svc.SemanticSearch(context.Background(), 7, tt.query, repositories.StudentSearchFilters{})

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			gotIDs := []uint{}
			for _, item := range got.Items {
				gotIDs = append(gotIDs, item.UserID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
			assert.Equal(t, int64(len(tt.wantIDs)), got.Total)
		})
	}
}

func TestStudentSearchService_SemanticSearch_KeepsFiltersForAndCondition(t *testing.T) {
	students := &stubStudentSearcher{rows: rows(3)}
	semantic := &stubSemantic{ids: []uint{3}}
	svc := NewStudentSearchService(students, &stubTagStore{}, semantic)

	_, err := svc.SemanticSearch(context.Background(), 7, "React", repositories.StudentSearchFilters{
		IndustryID: 4, Location: "東京", Skill: "基本情報", Tag: "候補A",
	})
	require.NoError(t, err)

	// セマンティック検索の結果と既存フィルタはAND結合される（受け入れ条件）。
	assert.Equal(t, uint(4), students.lastFilters.IndustryID)
	assert.Equal(t, "東京", students.lastFilters.Location)
	assert.Equal(t, "基本情報", students.lastFilters.Skill)
	assert.Equal(t, "候補A", students.lastFilters.Tag)
	assert.Equal(t, []uint{3}, students.lastFilters.UserIDs)
}

func TestStudentSearchService_SemanticSearch_UnconfiguredClient(t *testing.T) {
	svc := NewStudentSearchService(&stubStudentSearcher{}, &stubTagStore{}, nil)
	_, err := svc.SemanticSearch(context.Background(), 7, "React", repositories.StudentSearchFilters{})
	assert.ErrorIs(t, err, ErrSemanticSearchUnavailable)
}

func TestStudentSearchService_AddTag(t *testing.T) {
	tests := []struct {
		name    string
		tagName string
		visible bool
		wantErr error
	}{
		{name: "通常のタグ", tagName: "即戦力", visible: true},
		{name: "前後の空白は除去", tagName: "  候補A  ", visible: true},
		{name: "空文字は拒否", tagName: "   ", visible: true, wantErr: ErrInvalidTagName},
		{name: "上限超過は拒否", tagName: strings.Repeat("あ", models.MaxTagNameLength+1), visible: true, wantErr: ErrInvalidTagName},
		{name: "非公開の学生には付与できない", tagName: "即戦力", visible: false, wantErr: ErrStudentNotVisible},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			students := &stubStudentSearcher{visible: tt.visible}
			tags := &stubTagStore{}
			svc := NewStudentSearchService(students, tags, &stubSemantic{})

			err := svc.AddTag(7, 42, 100, tt.tagName)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, tags.added)
				return
			}
			require.NoError(t, err)
			require.Len(t, tags.added, 1)
			assert.Equal(t, strings.TrimSpace(tt.tagName), tags.added[0].TagName)
			assert.Equal(t, uint(7), tags.added[0].CompanyID, "タグは自社IDに紐づく")
			assert.Equal(t, uint(42), tags.added[0].CreatedBy)
			assert.Equal(t, uint(100), tags.added[0].UserID)
		})
	}
}

func TestStudentSearchService_RemoveTag_ScopedToOwnCompany(t *testing.T) {
	tags := &stubTagStore{}
	svc := NewStudentSearchService(&stubStudentSearcher{}, tags, &stubSemantic{})

	require.NoError(t, svc.RemoveTag(7, 55))
	// 削除は必ず company_id 付きで行われ、他社のタグIDを渡しても消えない。
	assert.Equal(t, [][2]uint{{7, 55}}, tags.deleted)
}

func TestStudentSearchService_AddTag_PropagatesStoreError(t *testing.T) {
	students := &stubStudentSearcher{visible: true}
	tags := &stubTagStore{addErr: errors.New("db down")}
	svc := NewStudentSearchService(students, tags, &stubSemantic{})

	err := svc.AddTag(7, 42, 100, "即戦力")
	assert.Error(t, err)
}
