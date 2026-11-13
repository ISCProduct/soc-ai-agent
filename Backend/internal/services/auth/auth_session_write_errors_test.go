package auth

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/services/shared"

	"golang.org/x/crypto/bcrypt"
)

// #2002: ログイン時の書き込みエラーを握りつぶさない。

// failNthUpdateUserRepo は、n 回目の UpdateUser だけ失敗させる（それ以外は userRepoAuthStub と同じ）。
type failNthUpdateUserRepo struct {
	*userRepoAuthStub
	failOn int
	calls  int
}

func (r *failNthUpdateUserRepo) UpdateUser(user *models.User) error {
	r.calls++
	if r.calls == r.failOn {
		return errors.New("db down")
	}
	return r.userRepoAuthStub.UpdateUser(user)
}

// reVerificationJobsSpy は、再認証メールのジョブ投入を数える。
type reVerificationJobsSpy struct {
	shared.JobEnqueuer
	reVerifications int
}

func (s *reVerificationJobsSpy) EnqueueEmailReVerification(uint, string, string, string, string) error {
	s.reVerifications++
	return nil
}

func captureAuthLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func loginTestUser(t *testing.T, cost int, lastLogin *time.Time) *models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), cost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	now := time.Now()
	return &models.User{
		ID: 1, Email: "user@example.com", Password: string(hash), Name: "User",
		TargetLevel: "新卒", EmailVerifiedAt: &now, LastLoginAt: lastLogin,
	}
}

// 古い強度のハッシュの再ハッシュを保存できなくても、ログインは成功し、失敗がログに残る。
func TestLogin_RehashSaveFails_StillLogsIn(t *testing.T) {
	t.Setenv("USER_SECRET", "user-secret")
	buf := captureAuthLog(t)
	user := loginTestUser(t, bcrypt.MinCost, nil)
	repo := &failNthUpdateUserRepo{userRepoAuthStub: &userRepoAuthStub{user: user}, failOn: 1}

	resp, err := NewAuthService(repo, &pendingRepoAuthStub{}, nil).Login(LoginRequest{Email: user.Email, Password: "password123"}, 0)

	if err != nil || resp == nil || resp.UserToken == "" {
		t.Fatalf("パスワードは正しいのでログインは成功すべき: resp=%v err=%v", resp, err)
	}
	if got := buf.String(); !strings.Contains(got, "failed to save rehashed password user_id=1: db down") {
		t.Errorf("再ハッシュを保存できなかったことがログに出ていない: %q", got)
	}
	if strings.Contains(buf.String(), user.Email) {
		t.Errorf("ログにメールアドレスを出さない: %q", buf.String())
	}
	// 後の最終ログイン更新は成功する。そこで新しいハッシュが保存されると、ログと実際がずれる（#2141 のレビュー）
	if repo.calls != 2 {
		t.Fatalf("UpdateUser の呼び出しが %d 回（再ハッシュと最終ログイン更新の2回のはず）", repo.calls)
	}
	if cost, _ := bcrypt.Cost([]byte(repo.user.Password)); cost != bcrypt.MinCost {
		t.Errorf("再ハッシュの保存に失敗したのに、新しいハッシュ（cost=%d）が後から保存された", cost)
	}
}

func TestLogin_RehashSaved_NoExtraLog(t *testing.T) {
	t.Setenv("USER_SECRET", "user-secret")
	buf := captureAuthLog(t)
	user := loginTestUser(t, bcrypt.MinCost, nil)
	repo := &userRepoAuthStub{user: user}

	if _, err := NewAuthService(repo, &pendingRepoAuthStub{}, nil).Login(LoginRequest{Email: user.Email, Password: "password123"}, 0); err != nil {
		t.Fatalf("login: %v", err)
	}
	if cost, _ := bcrypt.Cost([]byte(repo.user.Password)); cost != bcryptCost {
		t.Errorf("再ハッシュが保存されていない: cost=%d", cost)
	}
	if strings.Contains(buf.String(), "[AuthService]") {
		t.Errorf("通常時に余計なログが出た: %q", buf.String())
	}
}

// 再認証のトークンを保存できないときは、使えないリンクのメールを送らずにエラーを返す。
func TestLogin_ReVerificationTokenSaveFails_NoEmail(t *testing.T) {
	t.Setenv("USER_SECRET", "user-secret")
	lastLogin := time.Now().Add(-config.ReVerificationInactiveDuration - time.Hour)
	user := loginTestUser(t, bcryptCost, &lastLogin)
	repo := &failNthUpdateUserRepo{userRepoAuthStub: &userRepoAuthStub{user: user}, failOn: 1}
	jobs := &reVerificationJobsSpy{}
	svc := NewAuthService(repo, &pendingRepoAuthStub{}, nil)
	svc.SetJobEnqueuer(jobs)

	_, err := svc.Login(LoginRequest{Email: user.Email, Password: "password123"}, 0)

	if err == nil || !strings.Contains(err.Error(), "failed to save re-verification token") {
		t.Fatalf("トークンを保存できなかったことをエラーで返すべき: %v", err)
	}
	if jobs.reVerifications != 0 {
		t.Errorf("保存できていないトークンで再認証メールを送った: %d 回", jobs.reVerifications)
	}
}

// 保存できたときは、今までどおり再認証メールを送り、re_verification_required を返す。
func TestLogin_ReVerificationTokenSaved_SendsEmail(t *testing.T) {
	t.Setenv("USER_SECRET", "user-secret")
	lastLogin := time.Now().Add(-config.ReVerificationInactiveDuration - time.Hour)
	user := loginTestUser(t, bcryptCost, &lastLogin)
	jobs := &reVerificationJobsSpy{}
	svc := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)
	svc.SetJobEnqueuer(jobs)

	_, err := svc.Login(LoginRequest{Email: user.Email, Password: "password123"}, 0)

	if err == nil || err.Error() != "re_verification_required" {
		t.Fatalf("re_verification_required を返すべき: %v", err)
	}
	if jobs.reVerifications != 1 {
		t.Errorf("再認証メールのジョブが %d 回（1回のはず）", jobs.reVerifications)
	}
}
