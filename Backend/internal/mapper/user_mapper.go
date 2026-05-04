package mapper

import (
	"Backend/domain/entity"
	"Backend/internal/models"
)

// UserToEntity GORMモデルをドメインエンティティに変換する
func UserToEntity(m *models.User) *entity.User {
	if m == nil {
		return nil
	}
	return &entity.User{
		ID:                       m.ID,
		Email:                    m.Email,
		Password:                 m.Password,
		Name:                     m.Name,
		IsGuest:                  m.IsGuest,
		IsAdmin:                  m.IsAdmin,
		TargetLevel:              m.TargetLevel,
		SchoolName:               m.SchoolName,
		OAuthProvider:            m.OAuthProvider,
		OAuthID:                  m.OAuthID,
		AvatarURL:                m.AvatarURL,
		CertificationsAcquired:   m.CertificationsAcquired,
		CertificationsInProgress: m.CertificationsInProgress,
		EmailVerifiedAt:          m.EmailVerifiedAt,
		EmailVerificationToken:   m.EmailVerificationToken,
		EmailVerificationExpires: m.EmailVerificationExpires,
		LastLoginAt:              m.LastLoginAt,
		PasswordResetToken:       m.PasswordResetToken,
		PasswordResetExpiresAt:   m.PasswordResetExpiresAt,
		CreatedAt:                m.CreatedAt,
		UpdatedAt:                m.UpdatedAt,
	}
}

// UserFromEntity ドメインエンティティをGORMモデルに変換する
func UserFromEntity(e *entity.User) *models.User {
	if e == nil {
		return nil
	}
	return &models.User{
		ID:                       e.ID,
		Email:                    e.Email,
		Password:                 e.Password,
		Name:                     e.Name,
		IsGuest:                  e.IsGuest,
		IsAdmin:                  e.IsAdmin,
		TargetLevel:              e.TargetLevel,
		SchoolName:               e.SchoolName,
		OAuthProvider:            e.OAuthProvider,
		OAuthID:                  e.OAuthID,
		AvatarURL:                e.AvatarURL,
		CertificationsAcquired:   e.CertificationsAcquired,
		CertificationsInProgress: e.CertificationsInProgress,
		EmailVerifiedAt:          e.EmailVerifiedAt,
		EmailVerificationToken:   e.EmailVerificationToken,
		EmailVerificationExpires: e.EmailVerificationExpires,
		LastLoginAt:              e.LastLoginAt,
		PasswordResetToken:       e.PasswordResetToken,
		PasswordResetExpiresAt:   e.PasswordResetExpiresAt,
		CreatedAt:                e.CreatedAt,
		UpdatedAt:                e.UpdatedAt,
	}
}
