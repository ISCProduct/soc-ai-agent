package auth

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/email"
	"Backend/internal/services/shared"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

// 認証メールの再送(ResendVerificationEmail)のテスト。
// 期限切れ・未着のとき、未認証ユーザーが自分で新しいトークンを受け取れることを確かめる。
// どの入力でも「何か起きたか」を呼び出し側に伝えない(エラーを返さない)ことも確かめる。

// resendJobsSpy は、認証メールのジョブ投入を記録する。
type resendJobsSpy struct {
	shared.JobEnqueuer
	calls    int
	lastTo   string
	lastTok  string
	enqueueE error
}

func (s *resendJobsSpy) EnqueueEmailVerification(_ uint, email, _, token, _ string) error {
	s.calls++
	s.lastTo, s.lastTok = email, token
	return s.enqueueE
}

// resendLimiterSpy は、渡されたキーを記録し、allow の値で通す/止めるを決める。
type resendLimiterSpy struct {
	allow bool
	keys  []string
}

func (l *resendLimiterSpy) Allow(key string) bool {
	l.keys = append(l.keys, key)
	return l.allow
}

func unverifiedUser() *models.User {
	exp := time.Now().Add(-time.Hour) // 期限切れ
	return &models.User{
		ID:                       7,
		Email:                    "user@example.com",
		Name:                     "テスト",
		EmailVerificationToken:   "old-token",
		EmailVerificationExpires: &exp,
	}
}

func TestResendVerificationEmail(t *testing.T) {
	verifiedAt := time.Now().Add(-48 * time.Hour)
	withdrawnAt := time.Now().Add(-time.Hour)

	tests := []struct {
		name       string
		user       *models.User // nil = 存在しない
		email      string
		wantSent   bool
		wantTokens bool // トークンが新しく発行されること
	}{
		{name: "未認証: 新しいトークンを発行して送る", user: unverifiedUser(), email: "user@example.com", wantSent: true, wantTokens: true},
		{name: "未認証: 大文字・前後の空白があっても同じユーザー", user: unverifiedUser(), email: "  User@Example.COM ", wantSent: true, wantTokens: true},
		{name: "存在しないメールアドレス: 何もしない", user: nil, email: "nobody@example.com"},
		{name: "空のメールアドレス: 何もしない", user: unverifiedUser(), email: ""},
		{name: "認証済み: 何もしない", user: func() *models.User { u := unverifiedUser(); u.EmailVerifiedAt = &verifiedAt; return u }(), email: "user@example.com"},
		{name: "OAuth ユーザー: 何もしない", user: func() *models.User { u := unverifiedUser(); u.OAuthProvider = "google"; return u }(), email: "user@example.com"},
		{name: "ゲスト: 何もしない", user: func() *models.User { u := unverifiedUser(); u.IsGuest = true; return u }(), email: "user@example.com"},
		{name: "退会済み: 何もしない", user: func() *models.User { u := unverifiedUser(); u.WithdrawnAt = &withdrawnAt; return u }(), email: "user@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &userRepoAuthStub{user: tt.user}
			jobs := &resendJobsSpy{}
			svc := NewAuthService(repo, &pendingRepoAuthStub{}, &email.EmailService{})
			svc.SetJobEnqueuer(jobs)

			before := ""
			if tt.user != nil {
				before = tt.user.EmailVerificationToken
			}
			if err := svc.ResendVerificationEmail(tt.email); err != nil {
				t.Fatalf("エラーを返さない: %v", err)
			}

			if sent := jobs.calls == 1; sent != tt.wantSent {
				t.Fatalf("送信ジョブ = %d 件, 送るべきか = %v", jobs.calls, tt.wantSent)
			}
			if tt.user == nil {
				return
			}
			if !tt.wantTokens {
				if tt.user.EmailVerificationToken != before {
					t.Fatalf("送らない場合はトークンを変えない: %q → %q", before, tt.user.EmailVerificationToken)
				}
				return
			}
			if tok := tt.user.EmailVerificationToken; tok == "" || tok == before {
				t.Fatalf("新しいトークンに置き換わる: %q → %q", before, tok)
			}
			if jobs.lastTok != tt.user.EmailVerificationToken {
				t.Fatalf("保存したトークンと送るトークンが同じ: 保存=%q 送信=%q", tt.user.EmailVerificationToken, jobs.lastTok)
			}
			if jobs.lastTo != "user@example.com" {
				t.Fatalf("送り先 = %q", jobs.lastTo)
			}
			exp := tt.user.EmailVerificationExpires
			if exp == nil || time.Until(*exp) < 23*time.Hour || time.Until(*exp) > 25*time.Hour {
				t.Fatalf("期限は約24時間後: %v", exp)
			}
			if tt.user.EmailVerifiedAt != nil {
				t.Fatalf("再送では認証済みにしない")
			}
		})
	}
}

func TestResendVerificationEmail_RateLimited(t *testing.T) {
	user := unverifiedUser()
	repo := &userRepoAuthStub{user: user}
	jobs := &resendJobsSpy{}
	limiter := &resendLimiterSpy{allow: false}
	svc := NewAuthService(repo, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetJobEnqueuer(jobs)
	svc.SetResendLimiter(limiter)

	if err := svc.ResendVerificationEmail(" User@Example.com"); err != nil {
		t.Fatalf("上限超過でもエラーを返さない(存在を漏らさない): %v", err)
	}
	if jobs.calls != 0 {
		t.Fatalf("上限超過では送らない: %d 件", jobs.calls)
	}
	if user.EmailVerificationToken != "old-token" {
		t.Fatalf("上限超過ではトークンを変えない: %q", user.EmailVerificationToken)
	}
	// 上限はメールアドレス単位(大文字小文字・空白を揃えたキー)
	if len(limiter.keys) != 1 || limiter.keys[0] != "user@example.com" {
		t.Fatalf("制限のキー = %v", limiter.keys)
	}
}

func TestResendVerificationEmail_RateLimiterAllowsThrough(t *testing.T) {
	jobs := &resendJobsSpy{}
	svc := NewAuthService(&userRepoAuthStub{user: unverifiedUser()}, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetJobEnqueuer(jobs)
	svc.SetResendLimiter(&resendLimiterSpy{allow: true})

	if err := svc.ResendVerificationEmail("user@example.com"); err != nil {
		t.Fatal(err)
	}
	if jobs.calls != 1 {
		t.Fatalf("上限内なら送る: %d 件", jobs.calls)
	}
}

// 存在しないメールアドレスでも、上限は消費する(連打で存在の有無を探られないよう、
// 「存在する人だけ回数が減る」状態を作らない)。
func TestResendVerificationEmail_NonexistentEmailConsumesLimit(t *testing.T) {
	limiter := &resendLimiterSpy{allow: true}
	svc := NewAuthService(&userRepoAuthStub{}, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetResendLimiter(limiter)

	if err := svc.ResendVerificationEmail("nobody@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(limiter.keys) != 1 {
		t.Fatalf("存在しなくても上限を1回消費する: %v", limiter.keys)
	}
}

// ジョブ投入に失敗しても、呼び出し側にはエラーを返さない。
// 送信先が未設定のメール送信器でも、プロセスが落ちない(フォールバックはログ出力)。
func TestResendVerificationEmail_EnqueueFailureDoesNotLeak(t *testing.T) {
	jobs := &resendJobsSpy{enqueueE: errors.New("queue down")}
	svc := NewAuthService(&userRepoAuthStub{user: unverifiedUser()}, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetJobEnqueuer(jobs)

	if err := svc.ResendVerificationEmail("user@example.com"); err != nil {
		t.Fatalf("送信失敗はエラーとして返さない: %v", err)
	}
	if jobs.calls != 1 {
		t.Fatalf("投入を試みる: %d", jobs.calls)
	}
}

// renewFailRepo は、RenewEmailVerification だけを失敗させる。
type renewFailRepo struct{ *userRepoAuthStub }

func (renewFailRepo) RenewEmailVerification(uint, string, time.Time) (bool, error) {
	return false, errors.New("db down")
}

// トークンを保存できなかった場合は、メールを送らない。
// 届いたリンクが使えない(DB にトークンが無い)ので、送るだけ無駄になる(#2002 と同じ考え方)。
func TestResendVerificationEmail_SaveFailureDoesNotSend(t *testing.T) {
	repo := renewFailRepo{&userRepoAuthStub{user: unverifiedUser()}}
	jobs := &resendJobsSpy{}
	svc := NewAuthService(repo, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetJobEnqueuer(jobs)

	if err := svc.ResendVerificationEmail("user@example.com"); err == nil {
		t.Fatal("保存に失敗したことは、サービスの戻り値で分かる(呼び出し側は握りつぶす)")
	}
	if jobs.calls != 0 {
		t.Fatalf("保存に失敗したら送らない: %d", jobs.calls)
	}
}

// 読み込んだあとに認証が完了していた場合(同時に VerifyEmail が走った)は、何も書かず、送らない。
// 以前の、行を丸ごと保存する実装では、認証済みの印(email_verified_at)を巻き戻していた。
func TestResendVerificationEmail_VerifiedMeanwhileDoesNothing(t *testing.T) {
	user := unverifiedUser()
	repo := &verifiedAfterReadRepo{userRepoAuthStub: &userRepoAuthStub{user: user}}
	jobs := &resendJobsSpy{}
	svc := NewAuthService(repo, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetJobEnqueuer(jobs)

	if err := svc.ResendVerificationEmail("user@example.com"); err != nil {
		t.Fatal(err)
	}
	if jobs.calls != 0 {
		t.Fatalf("認証済みになっていたら送らない: %d", jobs.calls)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("認証済みの印を巻き戻してはいけない")
	}
	if user.EmailVerificationToken != "old-token" {
		t.Fatalf("トークンを書き換えない: %q", user.EmailVerificationToken)
	}
}

// verifiedAfterReadRepo は、GetUserByEmail で読んだ直後に、別の処理が認証を完了させた状況を作る。
// 読んだ側のコピーは未認証のまま、保存先(元の行)だけが認証済みになる。
type verifiedAfterReadRepo struct{ *userRepoAuthStub }

func (r *verifiedAfterReadRepo) GetUserByEmail(email string) (*models.User, error) {
	u, err := r.userRepoAuthStub.GetUserByEmail(email)
	if u == nil || err != nil {
		return u, err
	}
	snapshot := *u // 読んだ時点のコピー(未認証)
	now := time.Now()
	u.EmailVerifiedAt = &now // その直後に認証が完了した
	return &snapshot, nil
}

// 実の UserRepository と sqlmock で、新しいトークンと24時間の期限が
// DB への UPDATE に載ることを確かめる。#2261 で期限が保存されるようになった前提の、
// 再送経路の確認。UPDATE は users の全カラムを書くので、GORM が組み立てた引数を
// コールバックで拾い、トークンと期限が含まれることを見る。
func TestResendVerificationEmail_PersistsTokenAndExpiry(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	var vars []any
	if err := db.Callback().Update().After("gorm:update").Register("capture_resend_update", func(tx *gorm.DB) {
		vars = append(vars, tx.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewAuthService(repositories.NewUserRepository(db), &pendingRepoAuthStub{}, &email.EmailService{})
	jobs := &resendJobsSpy{}
	svc.SetJobEnqueuer(jobs)

	old := time.Now().Add(-time.Hour)
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("user@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "email_verification_token", "email_verification_expires"}).
			AddRow(7, "user@example.com", "テスト", "old-token", old))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.ResendVerificationEmail("user@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}

	var hasToken, hasExpiry bool
	for _, v := range vars {
		switch x := v.(type) {
		case string:
			if x == jobs.lastTok && x != "" && x != "old-token" {
				hasToken = true
			}
		case time.Time:
			if time.Until(x) > 23*time.Hour && time.Until(x) < 25*time.Hour {
				hasExpiry = true
			}
		case *time.Time:
			if x != nil && time.Until(*x) > 23*time.Hour && time.Until(*x) < 25*time.Hour {
				hasExpiry = true
			}
		}
	}
	if !hasToken {
		t.Fatalf("UPDATE に、送ったものと同じ新しいトークンが載る: vars=%v", vars)
	}
	if !hasExpiry {
		t.Fatalf("UPDATE に約24時間後の期限が載る: vars=%v", vars)
	}
}

// 再送のメールアドレス単位の上限は、main.go で注入する。
// 注入し忘れると、IP 単位の上限だけになり、同じ人のメールボックスへ何通も送れてしまう。
func TestMainWiresResendLimiter(t *testing.T) {
	raw, err := os.ReadFile("../../../cmd/server/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if want := "authService.SetResendLimiter(middleware.VerificationResendEmailRateLimiter)"; !strings.Contains(string(raw), want) {
		t.Errorf("main.go に %q が無い", want)
	}
}

// 入力検証: 長さの上限・形式を満たさないものは、制限器にも DB にもジョブにも通さず、nil を返す。
// 認証不要の入口なので、巨大な文字列を制限器のキー(1時間残る)にさせない。

// countingUserRepo は、GetUserByEmail が呼ばれた回数を数える。
type countingUserRepo struct {
	*userRepoAuthStub
	lookups int
}

func (r *countingUserRepo) GetUserByEmail(email string) (*models.User, error) {
	r.lookups++
	return r.userRepoAuthStub.GetUserByEmail(email)
}

func TestResendVerificationEmail_InvalidInputNeverReachesLimiterOrDB(t *testing.T) {
	tooLong := strings.Repeat("a", 255) // 255 文字(@ が無くても長さで先に弾く)
	tooLongButValidShape := strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 62) + ".com"

	tests := []struct {
		name  string
		input string
	}{
		{"10万文字", strings.Repeat("a", 100000)},
		{"10万文字(@あり)", strings.Repeat("a", 50000) + "@" + strings.Repeat("b", 49999)},
		{"255文字", tooLong},
		{"形式は正しいが255文字", tooLongButValidShape},
		{"@が無い", "user.example.com"},
		{"ローカル部が空", "@example.com"},
		{"ドメインが空", "user@"},
		{"@が2つ", "user@@example.com"},
		{"空白を含む", "us er@example.com"},
		{"表示名つき", "Name <user@example.com>"},
		{"複数指定", "user@example.com, other@example.com"},
		{"空白のみ", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &resendLimiterSpy{allow: true}
			repo := &countingUserRepo{userRepoAuthStub: &userRepoAuthStub{user: unverifiedUser()}}
			jobs := &resendJobsSpy{}
			svc := NewAuthService(repo, &pendingRepoAuthStub{}, &email.EmailService{})
			svc.SetJobEnqueuer(jobs)
			svc.SetResendLimiter(limiter)

			if err := svc.ResendVerificationEmail(tt.input); err != nil {
				t.Fatalf("エラーを返さない(応答を変えない): %v", err)
			}
			if len(limiter.keys) != 0 {
				t.Fatalf("制限器を通さない(キーが残る): %d 件", len(limiter.keys))
			}
			if repo.lookups != 0 {
				t.Fatalf("DB を検索しない: %d 回", repo.lookups)
			}
			if jobs.calls != 0 {
				t.Fatalf("送らない: %d 件", jobs.calls)
			}
		})
	}
}

// 上限ちょうど(254 文字)の、形式の正しいメールアドレスは通す。
func TestResendVerificationEmail_AcceptsMaxLengthAddress(t *testing.T) {
	addr := strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	if len(addr) != 254 {
		t.Fatalf("テストデータの長さ = %d, want 254", len(addr))
	}
	limiter := &resendLimiterSpy{allow: true}
	svc := NewAuthService(&userRepoAuthStub{}, &pendingRepoAuthStub{}, &email.EmailService{})
	svc.SetResendLimiter(limiter)

	if err := svc.ResendVerificationEmail(addr); err != nil {
		t.Fatal(err)
	}
	if len(limiter.keys) != 1 || limiter.keys[0] != addr {
		t.Fatalf("254 文字ちょうどは受け付ける: keys=%d", len(limiter.keys))
	}
}

// 実の UserRepository で、更新が「対象のカラムだけ・未認証の行だけ」であることを確かめる。
// 0 行しか更新されなかった(認証済みだった)場合は、メールを送らない。
func TestResendVerificationEmail_RealRepoUpdatesOnlyUnverifiedRows(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	var sqls []string
	if err := db.Callback().Update().After("gorm:update").Register("capture_resend_sql", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewAuthService(repositories.NewUserRepository(db), &pendingRepoAuthStub{}, &email.EmailService{})
	jobs := &resendJobsSpy{}
	svc.SetJobEnqueuer(jobs)

	mock.ExpectQuery("SELECT \\* FROM `users` WHERE email = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs("user@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name"}).AddRow(7, "user@example.com", "テスト"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET").WillReturnResult(sqlmock.NewResult(0, 0)) // 0 行 = 既に認証済み
	mock.ExpectCommit()

	if err := svc.ResendVerificationEmail("user@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
	if jobs.calls != 0 {
		t.Fatalf("0 行しか更新されなかったら送らない: %d", jobs.calls)
	}
	if len(sqls) != 1 {
		t.Fatalf("UPDATE は 1 回: %v", sqls)
	}
	q := sqls[0]
	if !strings.Contains(q, "email_verified_at IS NULL") {
		t.Fatalf("未認証の行だけを更新する条件が無い: %s", q)
	}
	for _, col := range []string{"`name`", "`password`", "`email_verified_at`=", "`is_admin`"} {
		if strings.Contains(q, col) {
			t.Fatalf("対象のカラム以外を書かない(%s を含む): %s", col, q)
		}
	}
}
