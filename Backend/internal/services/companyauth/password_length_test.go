package companyauth

import (
	"errors"
	"strings"
	"testing"

	"Backend/internal/services/shared"
)

// 企業側の登録・招待受諾・再設定も、学生と同じ共通検証を通る。72バイト超は DB に触れる前に弾く (SOCAIAGENT-693)。
func TestCompanyPasswordPaths_UseSharedValidation(t *testing.T) {
	long := strings.Repeat("a", 73)
	tests := []struct {
		name string
		call func(svc *CompanyUserService, pw string) error
	}{
		{name: "登録", call: func(svc *CompanyUserService, pw string) error {
			_, err := svc.Register(RegisterRequest{CompanyName: "合成株式会社", Name: "担当", Email: "hr@example.com", Password: pw})
			return err
		}},
		{name: "招待受諾", call: func(svc *CompanyUserService, pw string) error {
			_, err := svc.AcceptInvite(AcceptInviteRequest{Token: "tok", Password: pw})
			return err
		}},
		{name: "再設定", call: func(svc *CompanyUserService, pw string) error {
			_, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: pw})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newCompanyAuthTestDB(t)
			svc := newTestService(t, db)
			if err := tt.call(svc, long); !errors.Is(err, shared.ErrPasswordTooLong) {
				t.Fatalf("err = %v, want ErrPasswordTooLong", err)
			}
			if err := tt.call(svc, "short"); !errors.Is(err, shared.ErrPasswordTooShort) {
				t.Fatalf("err = %v, want ErrPasswordTooShort", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("検証前に DB に触れている: %v", err)
			}
		})
	}
}
