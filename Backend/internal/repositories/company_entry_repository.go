package repositories

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CompanyEntryRepository は repository.CompanyEntryRepository の GORM 実装。
type CompanyEntryRepository struct {
	db *gorm.DB
}

func NewCompanyEntryRepository(db *gorm.DB) *CompanyEntryRepository {
	return &CompanyEntryRepository{db: db}
}

func (r *CompanyEntryRepository) CreateDraft(d repository.CompanyEntryDraft) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(d.Company).Error; err != nil {
			return fmt.Errorf("create company: %w", err)
		}
		companyID := d.Company.ID
		d.Submission.CompanyID = companyID

		for _, position := range d.JobPositions {
			position.CompanyID = companyID
			if err := tx.Create(position).Error; err != nil {
				return fmt.Errorf("create job position: %w", err)
			}
		}

		if d.WeightProfile != nil {
			d.WeightProfile.CompanyID = companyID
			if err := tx.Save(d.WeightProfile).Error; err != nil {
				return fmt.Errorf("create weight profile: %w", err)
			}
		}

		for _, entry := range d.Graduates {
			entry.CompanyID = companyID
			if err := tx.Create(entry).Error; err != nil {
				return fmt.Errorf("create graduate employment: %w", err)
			}
		}

		if err := tx.Create(d.Submission).Error; err != nil {
			return fmt.Errorf("create submission: %w", err)
		}
		return nil
	})
}

func (r *CompanyEntryRepository) FindSubmission(submissionID uint) (*models.CompanyEntrySubmission, error) {
	var submission models.CompanyEntrySubmission
	if err := r.db.First(&submission, submissionID).Error; err != nil {
		return nil, err
	}
	return &submission, nil
}

func (r *CompanyEntryRepository) FindCompanyName(companyID uint) (*models.Company, error) {
	var company models.Company
	if err := r.db.Select("id", "name").First(&company, companyID).Error; err != nil {
		return nil, err
	}
	return &company, nil
}

func (r *CompanyEntryRepository) MarkInviteEmailSent(submissionID uint, at time.Time) error {
	return r.db.Model(&models.CompanyEntrySubmission{}).Where("id = ?", submissionID).Updates(map[string]any{
		"email_status":     "sent",
		"email_sent_at":    at,
		"email_last_error": "",
		"email_attempts":   gorm.Expr("email_attempts + 1"),
	}).Error
}

func (r *CompanyEntryRepository) MarkInviteEmailFailed(submissionID uint, msg string) error {
	return r.db.Model(&models.CompanyEntrySubmission{}).Where("id = ?", submissionID).Updates(map[string]any{
		"email_status":     "failed",
		"email_last_error": msg,
		"email_attempts":   gorm.Expr("email_attempts + 1"),
	}).Error
}

func (r *CompanyEntryRepository) ClaimOwnership(companyID, userID uint) (*models.CompanyOwnership, error) {
	ownership := &models.CompanyOwnership{
		CompanyID: companyID,
		UserID:    userID,
		Role:      "owner",
	}
	if err := r.db.Where("company_id = ?", companyID).FirstOrCreate(ownership).Error; err != nil {
		return nil, err
	}
	return ownership, nil
}

func (r *CompanyEntryRepository) MarkSubmissionClaimed(submissionID uint) error {
	return r.db.Model(&models.CompanyEntrySubmission{}).Where("id = ?", submissionID).Update("status", "claimed").Error
}
