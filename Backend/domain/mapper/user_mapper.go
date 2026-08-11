package mapper

import (
	"Backend/domain/entity"
	"Backend/internal/models"
)

// UserToEntity models.User を entity.User に変換
func UserToEntity(m *models.User) *entity.User {
	if m == nil {
		return nil
	}
	return &entity.User{
		ID:                       m.ID,
		OrganizationID:           m.OrganizationID,
		Email:                    m.Email,
		Password:                 m.Password,
		Name:                     m.Name,
		IsGuest:                  m.IsGuest,
		IsAdmin:                  m.IsAdmin,
		TargetLevel:              m.TargetLevel,
		SchoolName:               m.SchoolName,
		SchoolID:                 m.SchoolID,
		OAuthProvider:            m.OAuthProvider,
		OAuthID:                  m.OAuthID,
		AvatarURL:                m.AvatarURL,
		CertificationsAcquired:   m.CertificationsAcquired,
		CertificationsInProgress: m.CertificationsInProgress,
		EmailVerifiedAt:          m.EmailVerifiedAt,
		EmailVerificationToken:   m.EmailVerificationToken,
		LastLoginAt:              m.LastLoginAt,
		PasswordResetToken:       m.PasswordResetToken,
		PasswordResetExpiresAt:   m.PasswordResetExpiresAt,
		AllowCollectiveInsight:   m.AllowCollectiveInsight,
		WithdrawnAt:              m.WithdrawnAt,
		CreatedAt:                m.CreatedAt,
		UpdatedAt:                m.UpdatedAt,
	}
}

// UserFromEntity entity.User を models.User に変換
func UserFromEntity(e *entity.User) *models.User {
	if e == nil {
		return nil
	}
	return &models.User{
		ID:                       e.ID,
		OrganizationID:           e.OrganizationID,
		Email:                    e.Email,
		Password:                 e.Password,
		Name:                     e.Name,
		IsGuest:                  e.IsGuest,
		IsAdmin:                  e.IsAdmin,
		TargetLevel:              e.TargetLevel,
		SchoolName:               e.SchoolName,
		SchoolID:                 e.SchoolID,
		OAuthProvider:            e.OAuthProvider,
		OAuthID:                  e.OAuthID,
		AvatarURL:                e.AvatarURL,
		CertificationsAcquired:   e.CertificationsAcquired,
		CertificationsInProgress: e.CertificationsInProgress,
		EmailVerifiedAt:          e.EmailVerifiedAt,
		EmailVerificationToken:   e.EmailVerificationToken,
		LastLoginAt:              e.LastLoginAt,
		PasswordResetToken:       e.PasswordResetToken,
		PasswordResetExpiresAt:   e.PasswordResetExpiresAt,
		AllowCollectiveInsight:   e.AllowCollectiveInsight,
		WithdrawnAt:              e.WithdrawnAt,
		CreatedAt:                e.CreatedAt,
		UpdatedAt:                e.UpdatedAt,
	}
}

// PendingRegistrationToEntity models.PendingRegistration を entity.PendingRegistration に変換
func PendingRegistrationToEntity(m *models.PendingRegistration) *entity.PendingRegistration {
	if m == nil {
		return nil
	}
	return &entity.PendingRegistration{
		Token:     m.Token,
		Email:     m.Email,
		ExpiresAt: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
	}
}

// PendingRegistrationFromEntity entity.PendingRegistration を models.PendingRegistration に変換
func PendingRegistrationFromEntity(e *entity.PendingRegistration) *models.PendingRegistration {
	if e == nil {
		return nil
	}
	return &models.PendingRegistration{
		Token:     e.Token,
		Email:     e.Email,
		ExpiresAt: e.ExpiresAt,
		CreatedAt: e.CreatedAt,
	}
}
