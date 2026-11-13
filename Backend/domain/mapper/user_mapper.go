package mapper

import (
	"Backend/domain/entity"
	"Backend/internal/models"
)

// PendingRegistrationToEntity models.PendingRegistration を entity.PendingRegistration に変換
func PendingRegistrationToEntity(m *models.PendingRegistration) *entity.PendingRegistration {
	if m == nil {
		return nil
	}
	return &entity.PendingRegistration{
		Token:        m.Token,
		Email:        m.Email,
		CompanyID:    m.CompanyID,
		SubmissionID: m.SubmissionID,
		ExpiresAt:    m.ExpiresAt,
		CreatedAt:    m.CreatedAt,
	}
}

// PendingRegistrationFromEntity entity.PendingRegistration を models.PendingRegistration に変換
func PendingRegistrationFromEntity(e *entity.PendingRegistration) *models.PendingRegistration {
	if e == nil {
		return nil
	}
	return &models.PendingRegistration{
		Token:        e.Token,
		Email:        e.Email,
		CompanyID:    e.CompanyID,
		SubmissionID: e.SubmissionID,
		ExpiresAt:    e.ExpiresAt,
		CreatedAt:    e.CreatedAt,
	}
}
