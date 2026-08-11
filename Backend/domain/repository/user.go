// Package repository defines repository interfaces for the domain layer.
// Concrete implementations live in internal/repositories/.
package repository

import "Backend/domain/entity"

// UserRepository はユーザー永続化の抽象インターフェース。
type UserRepository interface {
	CreateUser(user *entity.User) error
	GetUserByEmail(email string) (*entity.User, error)
	GetUserByID(id uint) (*entity.User, error)
	ListUsers() ([]entity.User, error)
	// schoolID が非nilの場合は個別校で絞り込む。
	ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]entity.User, int64, error)
	UpdateUser(user *entity.User) error
	DeleteUser(id uint) error
	GetUserByVerificationToken(token string) (*entity.User, error)
	GetUserByPasswordResetToken(token string) (*entity.User, error)
	GetUserByOAuth(provider, oauthID string) (*entity.User, error)
}

// PendingRegistrationRepository は仮登録の永続化インターフェース。
type PendingRegistrationRepository interface {
	Create(p *entity.PendingRegistration) error
	FindByToken(token string) (*entity.PendingRegistration, error)
	DeleteByEmail(email string) error
	DeleteExpired() error
}
