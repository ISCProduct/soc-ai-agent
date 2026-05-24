package mocks

import (
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// CompanyRepositoryMock CompanyRepositoryのモック実装
type CompanyRepositoryMock struct {
	mock.Mock
}

func (m *CompanyRepositoryMock) FindAllActive(limit, offset int) ([]models.Company, error) {
	args := m.Called(limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]models.Company), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) CountActive() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *CompanyRepositoryMock) FindAllPublished(limit, offset int) ([]models.Company, error) {
	args := m.Called(limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]models.Company), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) CountPublished() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *CompanyRepositoryMock) FindByID(id uint) (*models.Company, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.Company), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) FindByName(name string) (*models.Company, error) {
	args := m.Called(name)
	if v := args.Get(0); v != nil {
		return v.(*models.Company), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) FindByCorporateNumber(corporateNumber string) (*models.Company, error) {
	args := m.Called(corporateNumber)
	if v := args.Get(0); v != nil {
		return v.(*models.Company), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error) {
	args := m.Called(companyID, jobPositionID)
	if v := args.Get(0); v != nil {
		return v.(*models.CompanyWeightProfile), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) Create(company *models.Company) error {
	args := m.Called(company)
	return args.Error(0)
}

func (m *CompanyRepositoryMock) Update(company *models.Company) error {
	args := m.Called(company)
	return args.Error(0)
}

func (m *CompanyRepositoryMock) FindJobPositionByCompanyAndTitle(companyID uint, title string) (*models.CompanyJobPosition, error) {
	args := m.Called(companyID, title)
	if v := args.Get(0); v != nil {
		return v.(*models.CompanyJobPosition), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) FindJobPositionByID(id uint) (*models.CompanyJobPosition, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.CompanyJobPosition), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) CreateJobPosition(position *models.CompanyJobPosition) error {
	args := m.Called(position)
	return args.Error(0)
}

func (m *CompanyRepositoryMock) UpdateJobPosition(position *models.CompanyJobPosition) error {
	args := m.Called(position)
	return args.Error(0)
}

func (m *CompanyRepositoryMock) FindJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyJobPosition), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) ListJobPositions(companyID *uint, limit int) ([]models.CompanyJobPosition, error) {
	args := m.Called(companyID, limit)
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyJobPosition), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRepositoryMock) CreateOrUpdateWeightProfile(profile *models.CompanyWeightProfile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *CompanyRepositoryMock) CountWeightProfiles() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}
