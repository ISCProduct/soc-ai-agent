package auth

import (
	"Backend/internal/middleware"
	"Backend/internal/models"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type userRepoAuthStub struct {
	user *models.User
}

func (r *userRepoAuthStub) CreateUser(user *models.User) error {
	r.user = user
	return nil
}
func (r *userRepoAuthStub) GetUserByEmail(email string) (*models.User, error) {
	if r.user != nil && r.user.Email == email {
		return r.user, nil
	}
	return nil, nil
}
func (r *userRepoAuthStub) GetUserByID(id uint) (*models.User, error) {
	if r.user != nil && r.user.ID == id {
		return r.user, nil
	}
	return nil, nil
}
func (r *userRepoAuthStub) ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]models.User, int64, error) {
	return nil, 0, nil
}
func (r *userRepoAuthStub) UpdateUser(user *models.User) error {
	r.user = user
	return nil
}
func (r *userRepoAuthStub) DeleteUser(id uint) error { return nil }
func (r *userRepoAuthStub) GetUserByVerificationToken(token string) (*models.User, error) {
	return nil, nil
}
func (r *userRepoAuthStub) GetUserByPasswordResetToken(token string) (*models.User, error) {
	return nil, nil
}
func (r *userRepoAuthStub) GetUserByOAuth(provider, oauthID string) (*models.User, error) {
	return nil, nil
}

type pendingRepoAuthStub struct{}

func (r *pendingRepoAuthStub) Create(p *models.PendingRegistration) error { return nil }
func (r *pendingRepoAuthStub) FindByToken(token string) (*models.PendingRegistration, error) {
	return nil, nil
}
func (r *pendingRepoAuthStub) DeleteByEmail(email string) error { return nil }
func (r *pendingRepoAuthStub) DeleteExpired(now time.Time, limit int) (int64, error) {
	return 0, nil
}

func TestAuthServiceLoginUsesUserSecretForUserToken(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "admin-secret")
	t.Setenv("USER_SECRET", "user-secret")

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcryptCost)
	if err != nil {
		t.Fatalf("password hash failed: %v", err)
	}
	now := time.Now()
	user := &models.User{
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

	user := &models.User{
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
	if _, err := middleware.ParseAdminToken(resp.Token, user.ID, user.Email, os.Getenv("ADMIN_SECRET")); err != nil {
		t.Fatal("admin token should verify")
	}
}
