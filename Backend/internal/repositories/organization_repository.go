package repositories

import (
	"Backend/internal/models"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type OrganizationRepository struct {
	db *gorm.DB
}

func NewOrganizationRepository(db *gorm.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Create(org *models.Organization) error {
	return r.db.Create(org).Error
}

func (r *OrganizationRepository) Update(org *models.Organization) error {
	return r.db.Save(org).Error
}

func (r *OrganizationRepository) FindByID(id uint) (*models.Organization, error) {
	var org models.Organization
	if err := r.db.First(&org, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &org, nil
}

func (r *OrganizationRepository) FindBySlug(slug string) (*models.Organization, error) {
	var org models.Organization
	if err := r.db.Where("slug = ?", slug).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &org, nil
}

func (r *OrganizationRepository) List(limit, offset int) ([]models.Organization, int64, error) {
	if limit <= 0 {
		limit = 25
	}
	var total int64
	if err := r.db.Model(&models.Organization{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orgs []models.Organization
	if err := r.db.Order("id ASC").Limit(limit).Offset(offset).Find(&orgs).Error; err != nil {
		return nil, 0, err
	}
	return orgs, total, nil
}

func (r *OrganizationRepository) CreateMembership(m *models.OrganizationMembership) error {
	return r.db.Create(m).Error
}

func (r *OrganizationRepository) UpdateMembership(m *models.OrganizationMembership) error {
	return r.db.Save(m).Error
}

func (r *OrganizationRepository) DeleteMembership(organizationID, userID uint) error {
	return r.db.Where("organization_id = ? AND user_id = ?", organizationID, userID).
		Delete(&models.OrganizationMembership{}).Error
}

func (r *OrganizationRepository) FindMembership(organizationID, userID uint) (*models.OrganizationMembership, error) {
	var m models.OrganizationMembership
	if err := r.db.Where("organization_id = ? AND user_id = ?", organizationID, userID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *OrganizationRepository) FindMembershipByUserID(userID uint) (*models.OrganizationMembership, error) {
	var m models.OrganizationMembership
	if err := r.db.Where("user_id = ?", userID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *OrganizationRepository) ListMembers(organizationID uint) ([]models.OrganizationMembership, error) {
	var members []models.OrganizationMembership
	if err := r.db.Where("organization_id = ?", organizationID).Order("id ASC").Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// GetUserOrganizationID は users.organization_id を返す。
func (r *OrganizationRepository) GetUserOrganizationID(userID uint) (uint, error) {
	var orgID uint
	err := r.db.Model(&models.User{}).Select("organization_id").Where("id = ?", userID).Scan(&orgID).Error
	if err != nil {
		return 0, err
	}
	if orgID == 0 {
		return models.DefaultOrganizationID, nil
	}
	return orgID, nil
}

// SetUserOrganizationID はユーザーの所属組織を更新する。
func (r *OrganizationRepository) SetUserOrganizationID(userID, organizationID uint) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Update("organization_id", organizationID).Error
}

// FindUserInOrganization は同一組織のユーザーのみ返す（クロステナント遮断用）。
func (r *OrganizationRepository) FindUserInOrganization(organizationID, userID uint) (*models.User, error) {
	var user models.User
	if err := r.db.Where("id = ? AND organization_id = ?", userID, organizationID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindChatMessageInOrganization は組織スコープ付きでチャットメッセージを取得する。
func (r *OrganizationRepository) FindChatMessageInOrganization(organizationID, messageID uint) (*models.ChatMessage, error) {
	var msg models.ChatMessage
	if err := r.db.Where("id = ? AND organization_id = ?", messageID, organizationID).First(&msg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &msg, nil
}

// FindInterviewSessionInOrganization は組織スコープ付きで面接セッションを取得する。
func (r *OrganizationRepository) FindInterviewSessionInOrganization(organizationID, sessionID uint) (*models.InterviewSession, error) {
	var session models.InterviewSession
	if err := r.db.Where("id = ? AND organization_id = ?", sessionID, organizationID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// FindResumeDocumentInOrganization は組織スコープ付きで履歴書ドキュメントを取得する。
func (r *OrganizationRepository) FindResumeDocumentInOrganization(organizationID, documentID uint) (*models.ResumeDocument, error) {
	var doc models.ResumeDocument
	if err := r.db.Where("id = ? AND organization_id = ?", documentID, organizationID).First(&doc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func NormalizeOrgSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}
