package auth

import (
	"Backend/domain/entity"
	"Backend/internal/services/email"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestAuthServiceLogin_TenantMismatchRejected(t *testing.T) {
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
		OrganizationID:  1,
		EmailVerifiedAt: &now,
	}
	service := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)

	if _, err := service.Login(LoginRequest{Email: user.Email, Password: "password123"}, 2); err == nil {
		t.Fatal("expected tenant mismatch error, got nil")
	}

	if _, err := service.Login(LoginRequest{Email: user.Email, Password: "password123"}, 1); err != nil {
		t.Fatalf("expected login to succeed for matching tenant, got: %v", err)
	}
}

func TestAuthServiceRegister_AssignsTenantOrganization(t *testing.T) {
	repo := &userRepoAuthStub{}
	service := NewAuthService(repo, &pendingRepoAuthStub{}, &email.EmailService{})

	_, err := service.Register(RegisterRequest{
		Email:    "new-user@example.com",
		Password: "password123",
		Name:     "New User",
	}, 7)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if repo.user == nil {
		t.Fatal("expected user to be created")
	}
	if repo.user.OrganizationID != 7 {
		t.Fatalf("OrganizationID = %d, want 7", repo.user.OrganizationID)
	}
}

func TestAuthServiceCreateGuestUser_AssignsTenantOrganization(t *testing.T) {
	repo := &userRepoAuthStub{}
	service := NewAuthService(repo, &pendingRepoAuthStub{}, nil)

	if _, err := service.CreateGuestUser(3); err != nil {
		t.Fatalf("create guest failed: %v", err)
	}
	if repo.user == nil {
		t.Fatal("expected guest user to be created")
	}
	if repo.user.OrganizationID != 3 {
		t.Fatalf("OrganizationID = %d, want 3", repo.user.OrganizationID)
	}
}
