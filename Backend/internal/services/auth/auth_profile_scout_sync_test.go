package auth

import (
	"context"
	"testing"

	"Backend/domain/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scoutSyncerSpy struct {
	syncedUserIDs []uint
}

func (s *scoutSyncerSpy) Sync(_ context.Context, userID uint) {
	s.syncedUserIDs = append(s.syncedUserIDs, userID)
}

// TestUpdateProfile_SyncsScoutIndex は、資格情報だけを更新した場合でも
// スカウト検索のベクトルが再同期されることを検証する (#1094)。
// 資格はベクトル化対象に含まれるため、希望条件の保存契機だけでは古いままになる。
func TestUpdateProfile_SyncsScoutIndex(t *testing.T) {
	t.Setenv("USER_SECRET", "user-secret")

	user := &entity.User{ID: 7, Email: "student@example.com", Name: "学生", TargetLevel: "新卒"}
	service := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)
	spy := &scoutSyncerSpy{}
	service.SetScoutIndexSyncer(spy)

	_, err := service.UpdateProfile(UpdateProfileRequest{
		UserID:                 7,
		CertificationsAcquired: "基本情報技術者",
	})
	require.NoError(t, err)

	assert.Equal(t, []uint{7}, spy.syncedUserIDs)
}

// TestUpdateProfile_WithoutSyncerConfigured は、RAG未設定環境でも
// プロフィール更新が成功することを検証する。
func TestUpdateProfile_WithoutSyncerConfigured(t *testing.T) {
	t.Setenv("USER_SECRET", "user-secret")

	user := &entity.User{ID: 7, Email: "student@example.com", Name: "学生", TargetLevel: "新卒"}
	service := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)

	resp, err := service.UpdateProfile(UpdateProfileRequest{
		UserID:                 7,
		CertificationsAcquired: "基本情報技術者",
	})
	require.NoError(t, err)
	assert.Equal(t, "基本情報技術者", resp.CertificationsAcquired)
}

// TestUpdateProfile_DoesNotSyncOnFailure は、保存に失敗した場合に
// インデックス同期を行わないことを検証する。
func TestUpdateProfile_DoesNotSyncOnFailure(t *testing.T) {
	service := NewAuthService(&userRepoAuthStub{}, &pendingRepoAuthStub{}, nil)
	spy := &scoutSyncerSpy{}
	service.SetScoutIndexSyncer(spy)

	_, err := service.UpdateProfile(UpdateProfileRequest{UserID: 999})
	require.Error(t, err)
	assert.Empty(t, spy.syncedUserIDs)
}
