package auth

import (
	"Backend/domain/entity"
	"Backend/internal/middleware"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type userRepoAuthStub struct {
	user *entity.User
}

func (r *userRepoAuthStub) CreateUser(user *entity.User) error {
	r.user = user
	return nil
}
func (r *userRepoAuthStub) GetUserByEmail(email string) (*entity.User, error) {
	if r.user != nil && r.user.Email == email {
		return r.user, nil
	}
	return nil, nil
}
func (r *userRepoAuthStub) GetUserByID(id uint) (*entity.User, error) {
	if r.user != nil && r.user.ID == id {
		return r.user, nil
	}
	return nil, nil
}
func (r *userRepoAuthStub) ListUsers() ([]entity.User, error) { return nil, nil }
func (r *userRepoAuthStub) ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]entity.User, int64, error) {
	return nil, 0, nil
}
func (r *userRepoAuthStub) UpdateUser(user *entity.User) error {
	r.user = user
	return nil
}
func (r *userRepoAuthStub) DeleteUser(id uint) error { return nil }
func (r *userRepoAuthStub) GetUserByVerificationToken(token string) (*entity.User, error) {
	return nil, nil
}
func (r *userRepoAuthStub) GetUserByPasswordResetToken(token string) (*entity.User, error) {
	return nil, nil
}
func (r *userRepoAuthStub) GetUserByOAuth(provider, oauthID string) (*entity.User, error) {
	return nil, nil
}

type pendingRepoAuthStub struct{}

func (r *pendingRepoAuthStub) Create(p *entity.PendingRegistration) error { return nil }
func (r *pendingRepoAuthStub) FindByToken(token string) (*entity.PendingRegistration, error) {
	return nil, nil
}
func (r *pendingRepoAuthStub) DeleteByEmail(email string) error { return nil }
func (r *pendingRepoAuthStub) DeleteExpired() error             { return nil }

func TestAuthServiceLoginUsesUserSecretForUserToken(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "admin-secret")
	t.Setenv("USER_SECRET", "user-secret")

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcryptCost)
	if err != nil {
		t.Fatalf("password hash failed: %v", err)
	}
	now := time.Now()
	user := &entity.User{
		ID:              1,
		Email:           "user@example.com",
		Password:        string(passwordHash),
		Name:            "User",
		TargetLevel:     "新卒",
		EmailVerifiedAt: &now,
	}
	service := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)

	resp, err := service.Login(LoginRequest{Email: user.Email, Password: "password123"}, 0)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if resp.UserToken == "" {
		t.Fatal("user token should be issued")
	}
	if !middleware.VerifyUserToken(resp.UserToken, user.ID, user.Email, os.Getenv("USER_SECRET")) {
		t.Fatal("user token should verify with USER_SECRET")
	}
	if middleware.VerifyUserToken(resp.UserToken, user.ID, user.Email, os.Getenv("ADMIN_SECRET")) {
		t.Fatal("user token must not verify with ADMIN_SECRET")
	}
}

func TestAuthServiceGetUserIssuesAdminToken(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "admin-secret")
	t.Setenv("USER_SECRET", "user-secret")

	user := &entity.User{
		ID:      7,
		Email:   "admin@example.com",
		Name:    "Admin",
		IsAdmin: true,
	}
	repo := &userRepoAuthStub{}
	repo.user = user
	service := NewAuthService(repo, &pendingRepoAuthStub{}, nil)

	resp, err := service.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("admin token should be issued on GetUser")
	}
	if !middleware.VerifyAdminToken(resp.Token, user.ID, user.Email, os.Getenv("ADMIN_SECRET")) {
		t.Fatal("admin token should verify")
	}
}
