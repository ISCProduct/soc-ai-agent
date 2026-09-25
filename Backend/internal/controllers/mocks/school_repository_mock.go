package mocks

import (
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// SchoolRepositoryMock SchoolRepositoryのモック実装(#980/#981/#982/#984の担当校スコープテスト用)
type SchoolRepositoryMock struct {
	mock.Mock
}

func (m *SchoolRepositoryMock) Create(school *models.School) error {
	args := m.Called(school)
	return args.Error(0)
}

func (m *SchoolRepositoryMock) Update(school *models.School) error {
	args := m.Called(school)
	return args.Error(0)
}

func (m *SchoolRepositoryMock) FindByID(id uint) (*models.School, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.School), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *SchoolRepositoryMock) FindByName(name string) (*models.School, error) {
	args := m.Called(name)
	if v := args.Get(0); v != nil {
		return v.(*models.School), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *SchoolRepositoryMock) List(limit, offset int) ([]models.School, int64, error) {
	args := m.Called(limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]models.School), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

func (m *SchoolRepositoryMock) AddMember(mem *models.AdminSchoolMembership) error {
	args := m.Called(mem)
	return args.Error(0)
}

func (m *SchoolRepositoryMock) RemoveMember(userID, schoolID uint) error {
	args := m.Called(userID, schoolID)
	return args.Error(0)
}

// ListSchoolsForAdmin は担当校一覧を返す。空スライス/nilを返すと「無制限admin」扱いになる。
func (m *SchoolRepositoryMock) ListSchoolsForAdmin(userID uint) ([]models.School, error) {
	args := m.Called(userID)
	if v := args.Get(0); v != nil {
		return v.([]models.School), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *SchoolRepositoryMock) AddCompanyApproval(a *models.SchoolCompanyApproval) error {
	args := m.Called(a)
	return args.Error(0)
}

func (m *SchoolRepositoryMock) RemoveCompanyApproval(schoolID, companyID uint) error {
	args := m.Called(schoolID, companyID)
	return args.Error(0)
}

func (m *SchoolRepositoryMock) IsCompanyApproved(schoolID, companyID uint) (bool, error) {
	args := m.Called(schoolID, companyID)
	return args.Bool(0), args.Error(1)
}

func (m *SchoolRepositoryMock) ListApprovedCompanyIDs(schoolID uint) ([]uint, error) {
	args := m.Called(schoolID)
	if v := args.Get(0); v != nil {
		return v.([]uint), args.Error(1)
	}
	return nil, args.Error(1)
}
