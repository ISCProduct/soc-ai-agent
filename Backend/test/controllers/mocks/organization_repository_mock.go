package mocks

import (
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// OrganizationRepositoryMock OrganizationRepositoryのモック実装
// (entitlement/admin plan判定のfail-closedテスト用)
type OrganizationRepositoryMock struct {
	mock.Mock
}

func (m *OrganizationRepositoryMock) Create(org *models.Organization) error {
	return m.Called(org).Error(0)
}

func (m *OrganizationRepositoryMock) Update(org *models.Organization) error {
	return m.Called(org).Error(0)
}

func (m *OrganizationRepositoryMock) FindByID(id uint) (*models.Organization, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.Organization), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) FindBySlug(slug string) (*models.Organization, error) {
	args := m.Called(slug)
	if v := args.Get(0); v != nil {
		return v.(*models.Organization), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) List(limit, offset int) ([]models.Organization, int64, error) {
	args := m.Called(limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]models.Organization), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

func (m *OrganizationRepositoryMock) CreateMembership(mem *models.OrganizationMembership) error {
	return m.Called(mem).Error(0)
}

func (m *OrganizationRepositoryMock) UpdateMembership(mem *models.OrganizationMembership) error {
	return m.Called(mem).Error(0)
}

func (m *OrganizationRepositoryMock) DeleteMembership(organizationID, userID uint) error {
	return m.Called(organizationID, userID).Error(0)
}

func (m *OrganizationRepositoryMock) FindMembership(organizationID, userID uint) (*models.OrganizationMembership, error) {
	args := m.Called(organizationID, userID)
	if v := args.Get(0); v != nil {
		return v.(*models.OrganizationMembership), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) FindMembershipByUserID(userID uint) (*models.OrganizationMembership, error) {
	args := m.Called(userID)
	if v := args.Get(0); v != nil {
		return v.(*models.OrganizationMembership), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) ListMembers(organizationID uint) ([]models.OrganizationMembership, error) {
	args := m.Called(organizationID)
	if v := args.Get(0); v != nil {
		return v.([]models.OrganizationMembership), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) GetUserOrganizationID(userID uint) (uint, error) {
	args := m.Called(userID)
	return args.Get(0).(uint), args.Error(1)
}

func (m *OrganizationRepositoryMock) SetUserOrganizationID(userID, organizationID uint) error {
	return m.Called(userID, organizationID).Error(0)
}

func (m *OrganizationRepositoryMock) FindUserInOrganization(organizationID, userID uint) (*models.User, error) {
	args := m.Called(organizationID, userID)
	if v := args.Get(0); v != nil {
		return v.(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) IsUserAdmin(userID uint) (bool, error) {
	args := m.Called(userID)
	return args.Bool(0), args.Error(1)
}

func (m *OrganizationRepositoryMock) AddMemberTransactional(mem *models.OrganizationMembership) error {
	return m.Called(mem).Error(0)
}

func (m *OrganizationRepositoryMock) RemoveMemberTransactional(organizationID, userID uint) error {
	return m.Called(organizationID, userID).Error(0)
}

func (m *OrganizationRepositoryMock) FindChatMessageInOrganization(organizationID, messageID uint) (*models.ChatMessage, error) {
	args := m.Called(organizationID, messageID)
	if v := args.Get(0); v != nil {
		return v.(*models.ChatMessage), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) FindInterviewSessionInOrganization(organizationID, sessionID uint) (*models.InterviewSession, error) {
	args := m.Called(organizationID, sessionID)
	if v := args.Get(0); v != nil {
		return v.(*models.InterviewSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *OrganizationRepositoryMock) FindResumeDocumentInOrganization(organizationID, documentID uint) (*models.ResumeDocument, error) {
	args := m.Called(organizationID, documentID)
	if v := args.Get(0); v != nil {
		return v.(*models.ResumeDocument), args.Error(1)
	}
	return nil, args.Error(1)
}
