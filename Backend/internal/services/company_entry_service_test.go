package services

import (
	"strings"
	"testing"

	"Backend/internal/services/email"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCompanyEntryTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

// #986: 会社が既に別ユーザーに所有されている場合、FirstOrCreateはエラーを返さず
// 既存レコード(別ユーザーのUserID)を返す。この場合、提出物をclaimed状態に
// 更新してはいけない(=UPDATEが発行されないこと)。
func TestClaimOwnershipOnRegister_AlreadyOwnedByAnotherUser(t *testing.T) {
	db, mock := newCompanyEntryTestDB(t)
	svc := &CompanyEntryService{db: db}

	companyID := uint(10)
	submissionID := uint(20)
	requestingUserID := uint(42)
	existingOwnerID := uint(99)

	mock.ExpectQuery("SELECT \\* FROM `company_ownerships`").
		WithArgs(companyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "user_id", "role"}).
			AddRow(1, companyID, existingOwnerID, "owner"))

	svc.ClaimOwnershipOnRegister(requestingUserID, &companyID, &submissionID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
}

// #986: 会社が未所有(新規)の場合は正常にクレームされ、提出物もclaimed状態に更新される。
func TestClaimOwnershipOnRegister_NewOwnership(t *testing.T) {
	db, mock := newCompanyEntryTestDB(t)
	svc := &CompanyEntryService{db: db}

	companyID := uint(11)
	submissionID := uint(21)
	requestingUserID := uint(42)

	mock.ExpectQuery("SELECT \\* FROM `company_ownerships`").
		WithArgs(companyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "user_id", "role"}))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_ownerships`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_entry_submissions`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc.ClaimOwnershipOnRegister(requestingUserID, &companyID, &submissionID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
}

func TestCompanyEntryService_SubmitValidation(t *testing.T) {
	svc := &CompanyEntryService{}

	tests := []struct {
		name    string
		in      CompanyEntryInput
		wantErr string
	}{
		{
			name:    "企業名必須",
			in:      CompanyEntryInput{ContactEmail: "hr@example.com", PrivacyConsent: true},
			wantErr: "name is required",
		},
		{
			name:    "メール必須",
			in:      CompanyEntryInput{Name: "テスト株式会社", PrivacyConsent: true},
			wantErr: "contact_email is required",
		},
		{
			name: "メール形式不正",
			in: CompanyEntryInput{
				Name: "テスト株式会社", ContactEmail: "not-an-email", PrivacyConsent: true,
			},
			wantErr: "contact_email is invalid",
		},
		{
			name: "同意必須",
			in: CompanyEntryInput{
				Name: "テスト株式会社", ContactEmail: "hr@example.com",
			},
			wantErr: "privacy_consent is required",
		},
		{
			name: "ハニーポット拒否",
			in: CompanyEntryInput{
				Name: "テスト株式会社", ContactEmail: "hr@example.com", PrivacyConsent: true, Honeypot: "bot",
			},
			wantErr: "rejected",
		},
		{
			name: "求人件数上限",
			in: CompanyEntryInput{
				Name:           "テスト株式会社",
				ContactEmail:   "hr@example.com",
				PrivacyConsent: true,
				JobPositions:   make([]CompanyEntryJobInput, maxEntryJobPositions+1),
			},
			wantErr: "job_positions must be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Submit(tt.in)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestEmailService_SendCompanyEntryThankYouAndInvite_LogFallback(t *testing.T) {
	svc := &email.EmailService{} // host empty → log only
	if err := svc.SendCompanyEntryThankYouAndInvite("hr@example.com", "テスト株式会社", "token123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.SendCompanyEntryThankYouAndInvite("hr@example.com", "", ""); err != nil {
		t.Fatalf("unexpected error without token: %v", err)
	}
}
