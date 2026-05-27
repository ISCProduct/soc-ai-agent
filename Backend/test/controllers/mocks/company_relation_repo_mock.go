package mocks

import (
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// CompanyRelationQueryRepositoryMock CompanyRelationQueryRepositoryのモック実装
type CompanyRelationQueryRepositoryMock struct {
	mock.Mock
}

func (m *CompanyRelationQueryRepositoryMock) GetByCompanyID(companyID uint) ([]models.CompanyRelation, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyRelation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRelationQueryRepositoryMock) GetAll() ([]models.CompanyRelation, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyRelation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRelationQueryRepositoryMock) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.(*models.CompanyMarketInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRelationQueryRepositoryMock) GetAllMarketInfo() ([]models.CompanyMarketInfo, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyMarketInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRelationQueryRepositoryMock) GetJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.([]models.CompanyJobPosition), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CompanyRelationQueryRepositoryMock) GetCompaniesFiltered(limit, offset int, industry, name, tech string) ([]models.Company, int64, error) {
	args := m.Called(limit, offset, industry, name, tech)
	if v := args.Get(0); v != nil {
		return v.([]models.Company), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

func (m *CompanyRelationQueryRepositoryMock) GetCompanyByID(id uint) (*models.Company, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.Company), args.Error(1)
	}
	return nil, args.Error(1)
}
