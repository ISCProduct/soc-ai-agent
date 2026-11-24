package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// resetCaptureJobs は送られた再設定メールの平文トークンを受け取る（メールは送らない）。
type resetCaptureJobs struct {
	shared.JobEnqueuer
	token string
}

func (j *resetCaptureJobs) EnqueueEmailPasswordReset(_, token, _ string) error {
	j.token = token
	return nil
}

// resetRepoStub は再設定トークンのハッシュで引けるユーザーリポジトリ。
type resetRepoStub struct {
	userRepoAuthStub
	lookedUp string
}

func (r *resetRepoStub) GetUserByPasswordResetTokenHash(_ context.Context, hash string) (*models.User, error) {
	r.lookedUp = hash
	if r.user != nil && r.user.PasswordResetTokenHash != nil && *r.user.PasswordResetTokenHash == hash {
		return r.user, nil
	}
	return nil, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// 再設定トークンは平文で保存せず SHA-256 で保存・照合し、メールには平文を送る (SOCAIAGENT-693)。
func TestAuthService_PasswordResetTokenIsStoredHashed(t *testing.T) {
	repo := &resetRepoStub{userRepoAuthStub: userRepoAuthStub{user: &models.User{ID: 3, Email: "student@example.com"}}}
	svc := NewAuthService(repo, &pendingRepoAuthStub{}, nil)
	jobs := &resetCaptureJobs{}
	svc.SetJobEnqueuer(jobs)

	if err := svc.RequestPasswordReset("student@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if jobs.token == "" {
		t.Fatal("メールに平文トークンが渡っていない")
	}
	stored := repo.user.PasswordResetTokenHash
	if stored == nil || *stored != sha256Hex(jobs.token) {
		t.Fatalf("保存値がトークンの SHA-256 でない: %v", stored)
	}

	if err := svc.ResetPassword(jobs.token, "newpassword1"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if repo.lookedUp != sha256Hex(jobs.token) {
		t.Errorf("照合がハッシュで行われていない")
	}
	if repo.user.PasswordResetTokenHash != nil || repo.user.PasswordResetExpiresAt != nil {
		t.Errorf("使用済みトークンが消えていない")
	}
}

// 再設定のパスワードは登録と同じ検証を通り、bcrypt のエラー文を返さない (SOCAIAGENT-693)。
func TestAuthService_ResetPasswordValidatesPassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		want error
	}{
		{name: "7文字", pw: "abcdefg", want: shared.ErrPasswordTooShort},
		{name: "73バイト", pw: strings.Repeat("a", 73), want: shared.ErrPasswordTooLong},
		{name: "72バイト", pw: strings.Repeat("a", 72), want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := sha256Hex("tok")
			expires := time.Now().Add(time.Hour)
			repo := &resetRepoStub{userRepoAuthStub: userRepoAuthStub{user: &models.User{
				ID: 3, Email: "student@example.com", PasswordResetTokenHash: &hash, PasswordResetExpiresAt: &expires,
			}}}
			err := NewAuthService(repo, &pendingRepoAuthStub{}, nil).ResetPassword("tok", tt.pw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "bcrypt") {
				t.Errorf("bcrypt のエラー文が出ている: %v", err)
			}
		})
	}
}

// 登録も同じ上限を通る（72バイト超を bcrypt に渡さない）(SOCAIAGENT-693)。
func TestAuthService_RegisterRejectsTooLongPassword(t *testing.T) {
	svc := NewAuthService(&userRepoAuthStub{}, &pendingRepoAuthStub{}, nil)
	_, err := svc.Register(RegisterRequest{Email: "new@example.com", Password: strings.Repeat("a", 73), Name: "合成 太郎"}, 0, 0)
	if !errors.Is(err, shared.ErrPasswordTooLong) {
		t.Fatalf("err = %v, want ErrPasswordTooLong", err)
	}
}
