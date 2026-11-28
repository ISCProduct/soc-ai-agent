package chat

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/models"
)

type jobCategoryRepoMock struct {
	topCategories    []models.JobCategory
	topCategoriesErr error
	byIDs            []models.JobCategory
	byIDsErr         error
	byIDsCalls       int
	byIDsArgs        []uint
}

func (m *jobCategoryRepoMock) FindAll(context.Context) ([]models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) FindByID(context.Context, uint) (*models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) FindByIDs(ctx context.Context, ids []uint) ([]models.JobCategory, error) {
	m.byIDsCalls++
	m.byIDsArgs = append([]uint(nil), ids...)
	if m.byIDsErr != nil {
		return nil, m.byIDsErr
	}
	return m.byIDs, nil
}

func (m *jobCategoryRepoMock) FindByName(context.Context, string) ([]models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) GetTopCategories(context.Context) ([]models.JobCategory, error) {
	if m.topCategoriesErr != nil {
		return nil, m.topCategoriesErr
	}
	return m.topCategories, nil
}

// 質問文が渡らない場合は大分類の一覧に当てる（従来の挙動）。
// 初回の職種選択は GenerateJobSelectionQuestion が大分類をその順で並べるため、
// この経路でも表示と一致する。
func TestNormalizeNumericAnswer_mapsTopCategory(t *testing.T) {
	v := &JobCategoryValidator{
		jobCategoryRepo: &jobCategoryRepoMock{
			topCategories: []models.JobCategory{
				{Name: "エンジニア"},
				{Name: "営業"},
			},
		},
	}

	got, err := v.normalizeNumericAnswer(context.Background(), "1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "エンジニア" {
		t.Fatalf("want エンジニア, got %s", got)
	}
}

func TestNormalizeNumericAnswer_mapsUndecidedOption(t *testing.T) {
	v := &JobCategoryValidator{
		jobCategoryRepo: &jobCategoryRepoMock{
			topCategories: []models.JobCategory{
				{Name: "エンジニア"},
				{Name: "営業"},
			},
		},
	}

	got, err := v.normalizeNumericAnswer(context.Background(), "3", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "まだ決めていない" {
		t.Fatalf("want まだ決めていない, got %s", got)
	}
}

func TestNormalizeNumericAnswer_keepsOriginalForOutOfRange(t *testing.T) {
	v := &JobCategoryValidator{
		jobCategoryRepo: &jobCategoryRepoMock{
			topCategories: []models.JobCategory{
				{Name: "エンジニア"},
				{Name: "営業"},
			},
		},
	}

	got, err := v.normalizeNumericAnswer(context.Background(), "9", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "9" {
		t.Fatalf("want 9, got %s", got)
	}
}

func TestNormalizeNumericAnswer_keepsOriginalForText(t *testing.T) {
	v := &JobCategoryValidator{
		jobCategoryRepo: &jobCategoryRepoMock{},
	}

	got, err := v.normalizeNumericAnswer(context.Background(), "エンジニア", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "エンジニア" {
		t.Fatalf("want エンジニア, got %s", got)
	}
}

func TestNormalizeNumericAnswer_returnsErrorOnTopCategoryFetchFailure(t *testing.T) {
	v := &JobCategoryValidator{
		jobCategoryRepo: &jobCategoryRepoMock{
			topCategoriesErr: errors.New("db error"),
		},
	}

	if _, err := v.normalizeNumericAnswer(context.Background(), "1", ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadMatchedCategories_OneLookupKeepsOrderAndSkipsMissing(t *testing.T) {
	repo := &jobCategoryRepoMock{
		byIDs: []models.JobCategory{
			{ID: 2, Name: "営業"},
			{ID: 9, Name: "研究"},
		},
	}
	v := &JobCategoryValidator{jobCategoryRepo: repo}

	got, err := v.loadMatchedCategories(context.Background(), []uint{9, 2, 9, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.byIDsCalls != 1 {
		t.Fatalf("FindByIDs calls = %d, want 1", repo.byIDsCalls)
	}
	if len(got) != 3 || got[0].Name != "研究" || got[1].Name != "営業" || got[2].Name != "研究" {
		t.Fatalf("got %#v", got)
	}
}

func TestLoadMatchedCategories_EmptySkipsLookup(t *testing.T) {
	repo := &jobCategoryRepoMock{}
	v := &JobCategoryValidator{jobCategoryRepo: repo}
	got, err := v.loadMatchedCategories(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
	if repo.byIDsCalls != 0 {
		t.Fatalf("FindByIDs calls = %d, want 0", repo.byIDsCalls)
	}
}

func TestLoadMatchedCategories_ReturnsLookupError(t *testing.T) {
	v := &JobCategoryValidator{jobCategoryRepo: &jobCategoryRepoMock{byIDsErr: errors.New("db down")}}
	if _, err := v.loadMatchedCategories(context.Background(), []uint{1}); err == nil {
		t.Fatal("expected error, got nil")
	}
}
