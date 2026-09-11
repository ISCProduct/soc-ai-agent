package repository

import "Backend/internal/models"

// OrganizationRepository はテナント（組織）とメンバーシップの永続化インターフェース。
type OrganizationRepository interface {
	Create(org *models.Organization) error
	Update(org *models.Organization) error
	FindByID(id uint) (*models.Organization, error)
	FindBySlug(slug string) (*models.Organization, error)
	List(limit, offset int) ([]models.Organization, int64, error)

	CreateMembership(m *models.OrganizationMembership) error
	UpdateMembership(m *models.OrganizationMembership) error
	DeleteMembership(organizationID, userID uint) error
	FindMembership(organizationID, userID uint) (*models.OrganizationMembership, error)
	FindMembershipByUserID(userID uint) (*models.OrganizationMembership, error)
	ListMembers(organizationID uint) ([]models.OrganizationMembership, error)

	GetUserOrganizationID(userID uint) (uint, error)
	SetUserOrganizationID(userID, organizationID uint) error
	FindUserInOrganization(organizationID, userID uint) (*models.User, error)
	IsUserAdmin(userID uint) (bool, error)

	// AddMemberTransactional は membership 作成・users.organization_id 更新・関連行の org 付け替えを同一 TX で行う。
	AddMemberTransactional(m *models.OrganizationMembership) error
	// RemoveMemberTransactional は membership 削除と users.organization_id のデフォルト復帰を同一 TX で行う。
	RemoveMemberTransactional(organizationID, userID uint) error

	FindChatMessageInOrganization(organizationID, messageID uint) (*models.ChatMessage, error)
	FindInterviewSessionInOrganization(organizationID, sessionID uint) (*models.InterviewSession, error)
	FindResumeDocumentInOrganization(organizationID, documentID uint) (*models.ResumeDocument, error)
}
