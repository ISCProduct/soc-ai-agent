package mapper_test

import (
	"testing"
	"time"

	"Backend/domain/mapper"
	"Backend/internal/models"
)

// TestAdminTokenNotBefore_SurvivesMapperRoundTrip は失効時刻が mapper の往復で
// 落ちないことを検証する（#1155）。
//
// UpdateUser は Save によるフルロー更新なので、mapper から落ちた瞬間に
// 「プロフィール更新で失効が消える」事故になる。
func TestAdminTokenNotBefore_SurvivesMapperRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 13, 1, 2, 3, 0, time.UTC)
	m := &models.User{ID: 7, Email: "admin@example.com", IsAdmin: true, AdminTokenNotBefore: &at}

	e := mapper.UserToEntity(m)
	if e.AdminTokenNotBefore == nil || !e.AdminTokenNotBefore.Equal(at) {
		t.Fatalf("model -> entity で失効時刻が落ちた: %v", e.AdminTokenNotBefore)
	}

	back := mapper.UserFromEntity(e)
	if back.AdminTokenNotBefore == nil || !back.AdminTokenNotBefore.Equal(at) {
		t.Fatalf("entity -> model で失効時刻が落ちた: %v", back.AdminTokenNotBefore)
	}
}
