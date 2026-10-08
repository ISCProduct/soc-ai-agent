package models_test

import (
	"testing"

	"Backend/internal/models"
)

func TestUserRoleHelpers(t *testing.T) {
	cases := []struct {
		name          string
		user          models.User
		wantStaff     bool
		wantAdminArea bool
	}{
		{"student", models.User{Role: models.UserRoleStudent}, false, false},
		{"pure staff", models.User{Role: models.UserRoleStaff}, true, true},
		{"platform admin", models.User{IsAdmin: true}, false, true},
		{"admin+staff", models.User{IsAdmin: true, Role: models.UserRoleStaff}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.user.HasStaffRole(); got != tc.wantStaff {
				t.Errorf("HasStaffRole=%v want %v", got, tc.wantStaff)
			}
			if got := tc.user.CanAccessAdminArea(); got != tc.wantAdminArea {
				t.Errorf("CanAccessAdminArea=%v want %v", got, tc.wantAdminArea)
			}
		})
	}
}
