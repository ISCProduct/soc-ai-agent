package auth

import (
	"testing"
	"time"

	"Backend/domain/entity"
)

func TestPromotableGuest(t *testing.T) {
	withdrawn := time.Now()
	tests := []struct {
		name string
		user *entity.User
		ok   bool
	}{
		{"ゲストは昇格できる", &entity.User{ID: 1, IsGuest: true}, true},

		// 本登録済みを上書きできると、他人のアカウントを乗っ取れる。
		{"本登録済みは昇格できない", &entity.User{ID: 1, IsGuest: false}, false},
		{"管理者は昇格できない", &entity.User{ID: 1, IsGuest: true, IsAdmin: true}, false},
		{"退会済みは昇格できない", &entity.User{ID: 1, IsGuest: true, WithdrawnAt: &withdrawn}, false},
		{"nilは昇格できない", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PromotableGuest(tt.user)
			if tt.ok && err != nil {
				t.Errorf("昇格できるべきなのに拒否された: %v", err)
			}
			if !tt.ok && err == nil {
				t.Error("昇格を拒否すべきなのに通った")
			}
		})
	}
}

// TestApplyRegistrationToGuest_KeepsIdentity は #1374 の核心。
//
// user_id を変えないことが引き継ぎの前提。変わると診断結果・マッチ結果・
// チャット履歴が取り残され、教員の一覧にも届かない。
func TestApplyRegistrationToGuest_KeepsIdentity(t *testing.T) {
	created := time.Now().Add(-24 * time.Hour)
	schoolID := uint(77)
	guest := &entity.User{
		ID:             42,
		OrganizationID: 5,
		CreatedAt:      created,
		Email:          "guest_abc@temp.local",
		Name:           "Guest_abc",
		IsGuest:        true,
		SchoolName:     "既定の学校",
		SchoolID:       &schoolID,
	}

	newSchoolID := uint(99)
	applyRegistrationToGuest(guest, RegisterRequest{
		Email:      "student@example.com",
		Name:       "山田太郎",
		SchoolName: "新しい専門学校",
	}, "hashed", &newSchoolID)

	// 変わってはいけないもの
	if guest.ID != 42 {
		t.Errorf("user_id が変わっている: %d（診断結果が取り残される）", guest.ID)
	}
	if guest.OrganizationID != 5 {
		t.Errorf("organization_id が変わっている: %d", guest.OrganizationID)
	}
	if !guest.CreatedAt.Equal(created) {
		t.Error("created_at が変わっている")
	}

	// 変わるべきもの
	if guest.IsGuest {
		t.Error("ゲストのままになっている")
	}
	if guest.Email != "student@example.com" || guest.Name != "山田太郎" {
		t.Errorf("登録内容が反映されていない: %s / %s", guest.Email, guest.Name)
	}
	if guest.Password != "hashed" {
		t.Error("パスワードが設定されていない")
	}
	if guest.SchoolID == nil || *guest.SchoolID != 99 {
		t.Errorf("school_id が更新されていない: %v", guest.SchoolID)
	}
}

// 学校名が空なら既存の紐付けを消さない。
// 消すと担当校の教員から生徒が見えなくなる。
func TestApplyRegistrationToGuest_KeepsSchoolWhenBlank(t *testing.T) {
	schoolID := uint(77)
	guest := &entity.User{
		ID: 1, IsGuest: true,
		SchoolName: "既存の学校", SchoolID: &schoolID,
	}

	applyRegistrationToGuest(guest, RegisterRequest{
		Email: "s@example.com", Name: "名前", SchoolName: "  ",
	}, "hashed", nil)

	if guest.SchoolName != "既存の学校" {
		t.Errorf("学校名が消えている: %q", guest.SchoolName)
	}
	if guest.SchoolID == nil || *guest.SchoolID != 77 {
		t.Errorf("school_id が消えている: %v（教員から生徒が見えなくなる）", guest.SchoolID)
	}
}
