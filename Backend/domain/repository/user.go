// Package repository defines repository interfaces for the domain layer.
// Concrete implementations live in internal/repositories/.
package repository

import (
	"time"

	"Backend/internal/models"
)

// UserRepository はユーザー永続化の抽象インターフェース。
type UserRepository interface {
	CreateUser(user *models.User) error
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id uint) (*models.User, error)
	// schoolID が非nilの場合は個別校で絞り込む。
	ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]models.User, int64, error)
	UpdateUser(user *models.User) error
	DeleteUser(id uint) error
	GetUserByVerificationToken(token string) (*models.User, error)
	// RenewEmailVerification はメール認証トークンと期限だけを、未認証(email_verified_at が NULL)の行に限って更新する。
	// 更新した場合に true を返す。認証済み・存在しない行では false(エラーではない)。
	RenewEmailVerification(userID uint, token string, expires time.Time) (bool, error)
	GetUserByPasswordResetToken(token string) (*models.User, error)
	GetUserByOAuth(provider, oauthID string) (*models.User, error)
}

// PendingRegistrationRepository は仮登録の永続化インターフェース。
type PendingRegistrationRepository interface {
	Create(p *models.PendingRegistration) error
	FindByToken(token string) (*models.PendingRegistration, error)
	DeleteByEmail(email string) error
	// DeleteExpired は期限切れ（expires_at <= now）の仮登録を最大 limit 件だけ消し、消した件数を返す。
	DeleteExpired(now time.Time, limit int) (int64, error)
}
