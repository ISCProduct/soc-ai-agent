package organization

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrMembershipNotFound   = errors.New("membership not found")
	ErrCrossOrganization    = errors.New("cross-organization access denied")
	ErrInvalidOrgRole       = errors.New("invalid organization role")
	ErrInvalidOrgSlug       = errors.New("invalid organization slug")
	ErrOrgSlugTaken         = errors.New("organization slug already taken")
	ErrUserAlreadyInOrg     = errors.New("user already belongs to an organization")
	ErrNameRequired         = errors.New("name is required")
	ErrInvalidOrgStatus     = errors.New("status must be active or disabled")
	ErrOrganizationDisabled = errors.New("organization is disabled")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,98}[a-z0-9])?$`)

// OrganizationService はテナント（組織）とメンバーシップを管理する。
type OrganizationService struct {
	repo repository.OrganizationRepository
}

func NewOrganizationService(repo repository.OrganizationRepository) *OrganizationService {
	return &OrganizationService{repo: repo}
}

type CreateOrganizationInput struct {
	Name string
	Slug string
}

type UpdateOrganizationInput struct {
	Name   *string
	Status *string
}

type AddMemberInput struct {
	OrganizationID uint
	UserID         uint
	Role           string
}

func (s *OrganizationService) Create(input CreateOrganizationInput) (*models.Organization, error) {
	name := strings.TrimSpace(input.Name)
	slug := repositories.NormalizeOrgSlug(input.Slug)
	if name == "" {
		return nil, ErrNameRequired
	}
	if !slugPattern.MatchString(slug) {
		return nil, ErrInvalidOrgSlug
	}
	existing, err := s.repo.FindBySlug(slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrOrgSlugTaken
	}
	org := &models.Organization{
		Name:   name,
		Slug:   slug,
		Status: models.OrgStatusActive,
	}
	if err := s.repo.Create(org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *OrganizationService) Update(id uint, input UpdateOrganizationInput) (*models.Organization, error) {
	org, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		org.Name = name
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if status != models.OrgStatusActive && status != models.OrgStatusDisabled {
			return nil, ErrInvalidOrgStatus
		}
		org.Status = status
	}
	if err := s.repo.Update(org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *OrganizationService) Get(id uint) (*models.Organization, error) {
	org, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}
	return org, nil
}

func (s *OrganizationService) List(limit, offset int) ([]models.Organization, int64, error) {
	return s.repo.List(limit, offset)
}

func (s *OrganizationService) ListMembers(organizationID uint) ([]models.OrganizationMembership, error) {
	org, err := s.repo.FindByID(organizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}
	return s.repo.ListMembers(organizationID)
}

func (s *OrganizationService) AddMember(input AddMemberInput) (*models.OrganizationMembership, error) {
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = models.OrgRoleMember
	}
	if !validOrgRole(role) {
		return nil, ErrInvalidOrgRole
	}
	org, err := s.repo.FindByID(input.OrganizationID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}
	existing, err := s.repo.FindMembershipByUserID(input.UserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserAlreadyInOrg
	}
	m := &models.OrganizationMembership{
		OrganizationID: input.OrganizationID,
		UserID:         input.UserID,
		Role:           role,
	}
	if err := s.repo.AddMemberTransactional(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *OrganizationService) UpdateMemberRole(organizationID, userID uint, role string) (*models.OrganizationMembership, error) {
	role = strings.TrimSpace(role)
	if !validOrgRole(role) {
		return nil, ErrInvalidOrgRole
	}
	m, err := s.repo.FindMembership(organizationID, userID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMembershipNotFound
	}
	m.Role = role
	if err := s.repo.UpdateMembership(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *OrganizationService) RemoveMember(organizationID, userID uint) error {
	m, err := s.repo.FindMembership(organizationID, userID)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrMembershipNotFound
	}
	if err := s.repo.RemoveMemberTransactional(organizationID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMembershipNotFound
		}
		return err
	}
	return nil
}

// ResolveOrganizationID はユーザーの所属組織IDを返す。無効組織は拒否する。
func (s *OrganizationService) ResolveOrganizationID(userID uint) (uint, error) {
	var orgID uint
	m, err := s.repo.FindMembershipByUserID(userID)
	if err != nil {
		return 0, err
	}
	if m != nil {
		orgID = m.OrganizationID
	} else {
		orgID, err = s.repo.GetUserOrganizationID(userID)
		if err != nil {
			return 0, err
		}
	}
	if orgID == 0 {
		return 0, ErrOrganizationNotFound
	}
	org, err := s.repo.FindByID(orgID)
	if err != nil {
		return 0, err
	}
	if org == nil {
		return 0, ErrOrganizationNotFound
	}
	if org.Status == models.OrgStatusDisabled {
		return 0, ErrOrganizationDisabled
	}
	return orgID, nil
}

// ResolveBySlug はサブドメインラベル(slug)から組織を解決する。無効組織は拒否する。
func (s *OrganizationService) ResolveBySlug(slug string) (*models.Organization, error) {
	org, err := s.repo.FindBySlug(repositories.NormalizeOrgSlug(slug))
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}
	if org.Status == models.OrgStatusDisabled {
		return nil, ErrOrganizationDisabled
	}
	return org, nil
}

// EnsureSameOrganization は actor と resource の組織が一致することを検証する。
func (s *OrganizationService) EnsureSameOrganization(actorOrgID, resourceOrgID uint) error {
	if actorOrgID == 0 || resourceOrgID == 0 {
		return ErrCrossOrganization
	}
	if actorOrgID != resourceOrgID {
		return ErrCrossOrganization
	}
	return nil
}

// AssertUserInOrganization は指定ユーザーが組織に属することを検証する。
func (s *OrganizationService) AssertUserInOrganization(organizationID, userID uint) error {
	user, err := s.repo.FindUserInOrganization(organizationID, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrCrossOrganization
	}
	return nil
}

// GetChatMessageForOrganization は別組織のチャットにアクセスできないことを保証する。
func (s *OrganizationService) GetChatMessageForOrganization(organizationID, messageID uint) (*models.ChatMessage, error) {
	msg, err := s.repo.FindChatMessageInOrganization(organizationID, messageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, ErrCrossOrganization
	}
	return msg, nil
}

// GetInterviewSessionForOrganization は別組織の面接セッションにアクセスできないことを保証する。
func (s *OrganizationService) GetInterviewSessionForOrganization(organizationID, sessionID uint) (*models.InterviewSession, error) {
	session, err := s.repo.FindInterviewSessionInOrganization(organizationID, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrCrossOrganization
	}
	return session, nil
}

// GetResumeDocumentForOrganization は別組織の履歴書にアクセスできないことを保証する。
func (s *OrganizationService) GetResumeDocumentForOrganization(organizationID, documentID uint) (*models.ResumeDocument, error) {
	doc, err := s.repo.FindResumeDocumentInOrganization(organizationID, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrCrossOrganization
	}
	return doc, nil
}

// AssignUserToDefaultOrganization は新規ユーザーをデフォルト組織へ所属させる。
func (s *OrganizationService) AssignUserToDefaultOrganization(userID uint, asAdmin bool) error {
	role := models.OrgRoleMember
	if asAdmin {
		role = models.OrgRoleOwner
	}
	_, err := s.AddMember(AddMemberInput{
		OrganizationID: models.DefaultOrganizationID,
		UserID:         userID,
		Role:           role,
	})
	if errors.Is(err, ErrUserAlreadyInOrg) {
		return nil
	}
	return err
}

func validOrgRole(role string) bool {
	switch role {
	case models.OrgRoleOwner, models.OrgRoleAdmin, models.OrgRoleMember:
		return true
	default:
		return false
	}
}

// FormatOrgError は API 用のエラーメッセージを返す。
func FormatOrgError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
