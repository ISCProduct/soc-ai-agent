package mocks

import (
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// JobCategoryRepositoryMock JobCategoryRepositoryのモック実装
type JobCategoryRepositoryMock struct {
	mock.Mock
}

func (m *JobCategoryRepositoryMock) FindAll() ([]models.JobCategory, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.JobCategory), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *JobCategoryRepositoryMock) FindByID(id uint) (*models.JobCategory, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.JobCategory), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *JobCategoryRepositoryMock) FindByName(name string) ([]models.JobCategory, error) {
	args := m.Called(name)
	if v := args.Get(0); v != nil {
		return v.([]models.JobCategory), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *JobCategoryRepositoryMock) FindByIndustry(industryID uint) ([]models.JobCategory, error) {
	args := m.Called(industryID)
	if v := args.Get(0); v != nil {
		return v.([]models.JobCategory), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *JobCategoryRepositoryMock) GetTopCategories() ([]models.JobCategory, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.JobCategory), args.Error(1)
	}
	return nil, args.Error(1)
}

// GraduateEmploymentRepositoryMock GraduateEmploymentRepositoryのモック実装
type GraduateEmploymentRepositoryMock struct {
	mock.Mock
}

func (m *GraduateEmploymentRepositoryMock) Create(entry *models.GraduateEmployment) error {
	args := m.Called(entry)
	return args.Error(0)
}

func (m *GraduateEmploymentRepositoryMock) FindByID(id uint) (*models.GraduateEmployment, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.GraduateEmployment), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *GraduateEmploymentRepositoryMock) Update(entry *models.GraduateEmployment) error {
	args := m.Called(entry)
	return args.Error(0)
}

func (m *GraduateEmploymentRepositoryMock) List(companyID *uint, limit int) ([]models.GraduateEmployment, error) {
	args := m.Called(companyID, limit)
	if v := args.Get(0); v != nil {
		return v.([]models.GraduateEmployment), args.Error(1)
	}
	return nil, args.Error(1)
}
