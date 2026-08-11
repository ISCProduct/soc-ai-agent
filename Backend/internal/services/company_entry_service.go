package services

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/config"
	"Backend/internal/models"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrCompanyEntrySubmissionNotFound = errors.New("submission not found")
	ErrCompanyEntryEmailSendFailed    = errors.New("failed to send email")
)

const (
	maxEntryJobPositions = 50
	maxEntryGraduates    = 30
)

// CompanyEntryInput は人事ゲスト投稿の入力（#754）。
type CompanyEntryInput struct {
	Name             string
	Description      string
	Industry         string
	Location         string
	WebsiteURL       string
	LogoURL          string
	CorporateNumber  string
	EmployeeCount    int
	FoundedYear      int
	AverageAge       float64
	FemaleRatio      float64
	Culture          string
	WorkStyle        string
	WelfareDetails   string
	TechStack        string
	DevelopmentStyle string
	MainBusiness     string
	JobPositions     []CompanyEntryJobInput
	WeightProfile    *CompanyEntryWeightInput
	Graduates        []CompanyEntryGraduateInput

	ContactEmail   string
	ContactName    string
	PrivacyConsent bool
	Honeypot       string // ボット対策。値が入っていれば拒否
	SourceIP       string
}

type CompanyEntryJobInput struct {
	Title           string
	Description     string
	JobCategoryID   uint
	MinSalary       int
	MaxSalary       int
	EmploymentType  string
	WorkLocation    string
	RemoteOption    bool
	RequiredSkills  string
	PreferredSkills string
}

type CompanyEntryWeightInput struct {
	TechnicalOrientation  int
	TeamworkOrientation   int
	LeadershipOrientation int
	CreativityOrientation int
	StabilityOrientation  int
	GrowthOrientation     int
	WorkLifeBalance       int
	ChallengeSeeking      int
	DetailOrientation     int
	CommunicationSkill    int
}

type CompanyEntryGraduateInput struct {
	GraduateName   string
	GraduationYear int
	SchoolName     string
	Department     string
	HiredAt        string
	Note           string
}

// CompanyEntryResult は投稿結果。
type CompanyEntryResult struct {
	CompanyID    uint
	SubmissionID uint
	Message      string
	EmailQueued  bool
}

// CompanyEntryService は企業ゲスト投稿と招待メールを扱う。
type CompanyEntryService struct {
	db          *gorm.DB
	userRepo    repository.UserRepository
	pendingRepo repository.PendingRegistrationRepository
	email       *EmailService
}

func NewCompanyEntryService(
	db *gorm.DB,
	userRepo repository.UserRepository,
	pendingRepo repository.PendingRegistrationRepository,
	email *EmailService,
) *CompanyEntryService {
	return &CompanyEntryService{
		db:          db,
		userRepo:    userRepo,
		pendingRepo: pendingRepo,
		email:       email,
	}
}

// Submit は draft 企業・求人を作成し、感謝＋本登録メールを送る。
// 公開は管理者 publish のみ（ゲスト投稿を自動公開しない）。
func (s *CompanyEntryService) Submit(in CompanyEntryInput) (*CompanyEntryResult, error) {
	if strings.TrimSpace(in.Honeypot) != "" {
		return nil, errors.New("rejected")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	email := strings.TrimSpace(strings.ToLower(in.ContactEmail))
	if email == "" {
		return nil, errors.New("contact_email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.New("contact_email is invalid")
	}
	if !in.PrivacyConsent {
		return nil, errors.New("privacy_consent is required")
	}
	if len(in.JobPositions) > maxEntryJobPositions {
		return nil, fmt.Errorf("job_positions must be %d or fewer", maxEntryJobPositions)
	}
	if len(in.Graduates) > maxEntryGraduates {
		return nil, fmt.Errorf("graduates must be %d or fewer", maxEntryGraduates)
	}

	now := time.Now()
	company := &models.Company{
		Name:             name,
		Description:      in.Description,
		Industry:         in.Industry,
		Location:         in.Location,
		WebsiteURL:       in.WebsiteURL,
		LogoURL:          in.LogoURL,
		CorporateNumber:  in.CorporateNumber,
		EmployeeCount:    in.EmployeeCount,
		FoundedYear:      in.FoundedYear,
		AverageAge:       in.AverageAge,
		FemaleRatio:      in.FemaleRatio,
		Culture:          in.Culture,
		WorkStyle:        in.WorkStyle,
		WelfareDetails:   in.WelfareDetails,
		TechStack:        in.TechStack,
		DevelopmentStyle: in.DevelopmentStyle,
		MainBusiness:     in.MainBusiness,
		SourceType:       "manual",
		DataStatus:       "draft",
		IsProvisional:    true,
		IsActive:         true,
		SourceFetchedAt:  &now,
	}

	submission := &models.CompanyEntrySubmission{
		ContactEmail:     email,
		ContactName:      strings.TrimSpace(in.ContactName),
		PrivacyConsentAt: now,
		SourceIP:         in.SourceIP,
		Status:           "submitted",
		EmailStatus:      "pending",
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(company).Error; err != nil {
			return fmt.Errorf("create company: %w", err)
		}
		submission.CompanyID = company.ID

		for _, jp := range in.JobPositions {
			if strings.TrimSpace(jp.Title) == "" {
				continue
			}
			position := &models.CompanyJobPosition{
				CompanyID:       company.ID,
				Title:           strings.TrimSpace(jp.Title),
				Description:     jp.Description,
				JobCategoryID:   jp.JobCategoryID,
				MinSalary:       jp.MinSalary,
				MaxSalary:       jp.MaxSalary,
				EmploymentType:  jp.EmploymentType,
				WorkLocation:    jp.WorkLocation,
				RemoteOption:    jp.RemoteOption,
				RequiredSkills:  jp.RequiredSkills,
				PreferredSkills: jp.PreferredSkills,
				IsActive:        true,
				DataStatus:      "draft",
			}
			if err := tx.Create(position).Error; err != nil {
				return fmt.Errorf("create job position: %w", err)
			}
		}

		if in.WeightProfile != nil {
			profile := &models.CompanyWeightProfile{
				CompanyID:             company.ID,
				TechnicalOrientation:  in.WeightProfile.TechnicalOrientation,
				TeamworkOrientation:   in.WeightProfile.TeamworkOrientation,
				LeadershipOrientation: in.WeightProfile.LeadershipOrientation,
				CreativityOrientation: in.WeightProfile.CreativityOrientation,
				StabilityOrientation:  in.WeightProfile.StabilityOrientation,
				GrowthOrientation:     in.WeightProfile.GrowthOrientation,
				WorkLifeBalance:       in.WeightProfile.WorkLifeBalance,
				ChallengeSeeking:      in.WeightProfile.ChallengeSeeking,
				DetailOrientation:     in.WeightProfile.DetailOrientation,
				CommunicationSkill:    in.WeightProfile.CommunicationSkill,
			}
			if err := tx.Save(profile).Error; err != nil {
				return fmt.Errorf("create weight profile: %w", err)
			}
		}

		for _, g := range in.Graduates {
			var hiredAt *time.Time
			if strings.TrimSpace(g.HiredAt) != "" {
				if parsed, parseErr := time.Parse("2006-01-02", g.HiredAt); parseErr == nil {
					hiredAt = &parsed
				}
			}
			entry := &models.GraduateEmployment{
				CompanyID:      company.ID,
				GraduateName:   strings.TrimSpace(g.GraduateName),
				GraduationYear: g.GraduationYear,
				SchoolName:     strings.TrimSpace(g.SchoolName),
				Department:     strings.TrimSpace(g.Department),
				HiredAt:        hiredAt,
				Note:           strings.TrimSpace(g.Note),
			}
			if err := tx.Create(entry).Error; err != nil {
				return fmt.Errorf("create graduate employment: %w", err)
			}
		}

		if err := tx.Create(submission).Error; err != nil {
			return fmt.Errorf("create submission: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	emailQueued := s.sendInviteEmail(submission, company.Name)

	return &CompanyEntryResult{
		CompanyID:    company.ID,
		SubmissionID: submission.ID,
		EmailQueued:  emailQueued,
		Message:      "送信が完了しました。内容を確認の上、掲載審査を行います。ご登録のメールアドレスに確認メールをお送りしました。",
	}, nil
}

func (s *CompanyEntryService) sendInviteEmail(submission *models.CompanyEntrySubmission, companyName string) bool {
	inviteToken := ""
	existing, err := s.userRepo.GetUserByEmail(submission.ContactEmail)
	if err != nil {
		log.Printf("[CompanyEntry] check existing user: %v", err)
	}

	if existing == nil {
		_ = s.pendingRepo.DeleteByEmail(submission.ContactEmail)
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			s.markEmailFailed(submission.ID, "token generate failed")
			return false
		}
		inviteToken = base64.URLEncoding.EncodeToString(b)
		companyID := submission.CompanyID
		submissionID := submission.ID
		pending := &entity.PendingRegistration{
			Token:        inviteToken,
			Email:        submission.ContactEmail,
			CompanyID:    &companyID,
			SubmissionID: &submissionID,
			ExpiresAt:    time.Now().Add(config.PendingRegistrationTokenTTL),
		}
		if err := s.pendingRepo.Create(pending); err != nil {
			s.markEmailFailed(submission.ID, err.Error())
			return false
		}
		submission.InviteToken = &inviteToken
		_ = s.db.Model(submission).Update("invite_token", inviteToken).Error
	}

	if err := s.email.SendCompanyEntryThankYouAndInvite(submission.ContactEmail, companyName, inviteToken); err != nil {
		s.markEmailFailed(submission.ID, err.Error())
		return false
	}
	now := time.Now()
	_ = s.db.Model(&models.CompanyEntrySubmission{}).Where("id = ?", submission.ID).Updates(map[string]any{
		"email_status":     "sent",
		"email_sent_at":    now,
		"email_last_error": "",
		"email_attempts":   gorm.Expr("email_attempts + 1"),
	}).Error
	return true
}

func (s *CompanyEntryService) markEmailFailed(submissionID uint, msg string) {
	_ = s.db.Model(&models.CompanyEntrySubmission{}).Where("id = ?", submissionID).Updates(map[string]any{
		"email_status":     "failed",
		"email_last_error": msg,
		"email_attempts":   gorm.Expr("email_attempts + 1"),
	}).Error
}

// ResendEmail は管理者による招待メール再送。
func (s *CompanyEntryService) ResendEmail(submissionID uint) error {
	var submission models.CompanyEntrySubmission
	if err := s.db.First(&submission, submissionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCompanyEntrySubmissionNotFound
		}
		return err
	}
	var company models.Company
	if err := s.db.Select("id", "name").First(&company, submission.CompanyID).Error; err != nil {
		return err
	}
	if !s.sendInviteEmail(&submission, company.Name) {
		return ErrCompanyEntryEmailSendFailed
	}
	return nil
}

// ClaimOwnershipOnRegister は仮登録トークンに紐づく企業をユーザーへクレームする。
func (s *CompanyEntryService) ClaimOwnershipOnRegister(userID uint, companyID, submissionID *uint) {
	if companyID == nil || *companyID == 0 {
		return
	}
	ownership := &models.CompanyOwnership{
		CompanyID: *companyID,
		UserID:    userID,
		Role:      "owner",
	}
	if err := s.db.Where("company_id = ?", *companyID).FirstOrCreate(ownership).Error; err != nil {
		log.Printf("[CompanyEntry] claim ownership failed company=%d user=%d: %v", *companyID, userID, err)
		return
	}
	if submissionID != nil && *submissionID != 0 {
		_ = s.db.Model(&models.CompanyEntrySubmission{}).Where("id = ?", *submissionID).Update("status", "claimed").Error
	}
}
