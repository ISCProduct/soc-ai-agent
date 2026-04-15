package services

import (
	"Backend/internal/models"
	"errors"
	"testing"
)

type jobCategoryRepoMock struct {
	topCategories    []models.JobCategory
	topCategoriesErr error
}

func (m *jobCategoryRepoMock) FindAll() ([]models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) FindByID(id uint) (*models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) FindByName(name string) ([]models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) FindByIndustry(industryID uint) ([]models.JobCategory, error) {
	return nil, nil
}

func (m *jobCategoryRepoMock) GetTopCategories() ([]models.JobCategory, error) {
	if m.topCategoriesErr != nil {
		return nil, m.topCategoriesErr
	}
	return m.topCategories, nil
}

func TestNormalizeNumericAnswer_mapsTopCategory(t *testing.T) {
	v := &JobCategoryValidator{
		jobCategoryRepo: &jobCategoryRepoMock{
			topCategories: []models.JobCategory{
				{Name: "エンジニア"},
				{Name: "営業"},
			},
		},
	}

	got, err := v.normalizeNumericAnswer("1")
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

	got, err := v.normalizeNumericAnswer("3")
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

	got, err := v.normalizeNumericAnswer("9")
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

	got, err := v.normalizeNumericAnswer("エンジニア")
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

	if _, err := v.normalizeNumericAnswer("1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
