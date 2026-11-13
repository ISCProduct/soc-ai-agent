package company

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/email"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// #2000: 企業ゲスト投稿の状態更新に失敗しても、処理の結果は変えずにログで分かるようにする。

// captureCompanyEntryLog は、テスト中の log の出力を集めて返す。
func captureCompanyEntryLog(t *testing.T) *bytes.Buffer {
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

// companyEntryLines は、[CompanyEntry] のログ行だけを返す（メール送信のログなどは除く）。
func companyEntryLines(buf *bytes.Buffer) []string {
	var out []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "[CompanyEntry]") {
			out = append(out, l)
		}
	}
	return out
}

type entryUserRepoStub struct {
	repository.UserRepository
	user *models.User
}

func (s entryUserRepoStub) GetUserByEmail(string) (*models.User, error) { return s.user, nil }

type entryPendingRepoStub struct {
	repository.PendingRegistrationRepository
	deleteErr, createErr error
}

func (s entryPendingRepoStub) DeleteByEmail(string) error               { return s.deleteErr }
func (s entryPendingRepoStub) Create(*models.PendingRegistration) error { return s.createErr }

func newInviteTestService(t *testing.T, user *models.User, pending entryPendingRepoStub) (*CompanyEntryService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := newCompanyEntryTestDB(t)
	return &CompanyEntryService{
		db:          db,
		userRepo:    entryUserRepoStub{user: user},
		pendingRepo: pending,
		email:       &email.EmailService{}, // host が空なので送らずにログだけ出し、成功を返す
	}, mock
}

func TestSendInviteEmail_StatusUpdateFails_StillSentAndLogged(t *testing.T) {
	buf := captureCompanyEntryLog(t)
	svc, mock := newInviteTestService(t, &models.User{ID: 1}, entryPendingRepoStub{})
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions`").WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	ok := svc.sendInviteEmail(&models.CompanyEntrySubmission{ID: 7, ContactEmail: "hr@example.com"}, "テスト株式会社")

	if !ok {
		t.Fatal("メールは送れているので true を返すべき")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
	lines := companyEntryLines(buf)
	if len(lines) != 1 || !strings.Contains(lines[0], "failed to record status submission=7") || !strings.Contains(lines[0], "db down") {
		t.Errorf("記録の失敗がログに出ていない: %q", lines)
	}
	if strings.Contains(strings.Join(lines, "\n"), "hr@example.com") {
		t.Errorf("ログにメールアドレスを出さない: %q", lines)
	}
}

func TestSendInviteEmail_Normal_NoExtraLog(t *testing.T) {
	buf := captureCompanyEntryLog(t)
	svc, mock := newInviteTestService(t, &models.User{ID: 1}, entryPendingRepoStub{})
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if !svc.sendInviteEmail(&models.CompanyEntrySubmission{ID: 7, ContactEmail: "hr@example.com"}, "テスト株式会社") {
		t.Fatal("true を返すべき")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
	if lines := companyEntryLines(buf); len(lines) != 0 {
		t.Errorf("通常時に余計なログが出た: %q", lines)
	}
}

// 仮登録の削除と作成が失敗し、さらに送信失敗の記録も失敗する。どれもログに残り、false を返す。
// 作成の失敗理由（一意制約違反のメッセージ）にはメールアドレスやトークンが入るので、ログには出さない（#2136 のレビュー）。
func TestSendInviteEmail_PendingAndFailureRecordFail_Logged(t *testing.T) {
	buf := captureCompanyEntryLog(t)
	svc, mock := newInviteTestService(t, nil, entryPendingRepoStub{
		deleteErr: errors.New("delete failed"),
		createErr: errors.New("Error 1062: Duplicate entry 'hr@example.com' for key 'idx_email'"),
	})
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions`").WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	if svc.sendInviteEmail(&models.CompanyEntrySubmission{ID: 8, ContactEmail: "hr@example.com"}, "テスト株式会社") {
		t.Fatal("仮登録を作れなかったので false を返すべき")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
	got := strings.Join(companyEntryLines(buf), "\n")
	for _, want := range []string{
		"delete previous pending registration failed submission=8: delete failed",
		"failed to record invite email failure submission=8: db down",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ログに %q が無い:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hr@example.com") || strings.Contains(got, "Duplicate entry") {
		t.Errorf("送信失敗の理由（メールアドレス入り）をログに出した:\n%s", got)
	}
}

func TestClaimOwnershipOnRegister_StatusUpdateFails_Logged(t *testing.T) {
	buf := captureCompanyEntryLog(t)
	db, mock := newCompanyEntryTestDB(t)
	svc := &CompanyEntryService{db: db}
	companyID, submissionID, userID := uint(11), uint(21), uint(42)

	mock.ExpectQuery("SELECT \\* FROM `company_ownerships`").
		WithArgs(companyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "user_id", "role"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_ownerships`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions`").WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	svc.ClaimOwnershipOnRegister(userID, &companyID, &submissionID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
	lines := companyEntryLines(buf)
	if len(lines) != 1 || !strings.Contains(lines[0], "failed to mark submission=21 as claimed company=11 user=42: db down") {
		t.Errorf("claimed にできなかったことがログに出ていない: %q", lines)
	}
}

func TestClaimOwnershipOnRegister_Normal_NoExtraLog(t *testing.T) {
	buf := captureCompanyEntryLog(t)
	db, mock := newCompanyEntryTestDB(t)
	svc := &CompanyEntryService{db: db}
	companyID, submissionID := uint(11), uint(21)

	mock.ExpectQuery("SELECT \\* FROM `company_ownerships`").
		WithArgs(companyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "user_id", "role"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_ownerships`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc.ClaimOwnershipOnRegister(42, &companyID, &submissionID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
	if lines := companyEntryLines(buf); len(lines) != 0 {
		t.Errorf("通常時に余計なログが出た: %q", lines)
	}
}
