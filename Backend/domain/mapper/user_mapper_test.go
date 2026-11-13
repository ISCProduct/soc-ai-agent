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

// TestConsentFlags_SurviveMapperRoundTrip は同意のフラグが mapper の往復で落ちないことを検証する（#2180 のレビュー）。
//
// UpdateUser は Save によるフルロー更新なので、mapper から落ちると、プロフィールの保存や
// 再ログインのたびに同意が false に戻る（学習利用の同意を取り消したことになる）。
func TestConsentFlags_SurviveMapperRoundTrip(t *testing.T) {
	m := &models.User{ID: 7, Email: "student@example.com", AllowCollectiveInsight: true, AllowScoutVisibility: true, AllowTrainingUse: true}

	back := mapper.UserFromEntity(mapper.UserToEntity(m))
	if !back.AllowTrainingUse {
		t.Errorf("往復で学習利用の同意が落ちた")
	}
	if !back.AllowScoutVisibility || !back.AllowCollectiveInsight {
		t.Errorf("往復でほかの同意が落ちた: scout=%v collective=%v", back.AllowScoutVisibility, back.AllowCollectiveInsight)
	}
}
