package hr

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubScoutProfiles struct {
	allow    bool
	allowErr error
	text     string
	textErr  error
}

func (s *stubScoutProfiles) GetScoutVisibility(uint) (bool, error) { return s.allow, s.allowErr }
func (s *stubScoutProfiles) ScoutProfileText(uint) (string, error) { return s.text, s.textErr }

type stubIndexer struct {
	indexed   []string
	deleted   []uint
	indexErr  error
	deleteErr error
}

func (s *stubIndexer) Index(_ context.Context, _ uint, text string) error {
	s.indexed = append(s.indexed, text)
	return s.indexErr
}

func (s *stubIndexer) Delete(_ context.Context, userID uint) error {
	s.deleted = append(s.deleted, userID)
	return s.deleteErr
}

func TestStudentIndexSyncer_Sync(t *testing.T) {
	tests := []struct {
		name        string
		profiles    *stubScoutProfiles
		wantIndexed []string
		wantDeleted []uint
	}{
		{
			name:        "同意ONなら最新テキストで登録する",
			profiles:    &stubScoutProfiles{allow: true, text: "取得資格: 基本情報"},
			wantIndexed: []string{"取得資格: 基本情報"},
		},
		{
			name:        "同意OFFならベクトルを削除する",
			profiles:    &stubScoutProfiles{allow: false},
			wantDeleted: []uint{7},
		},
		{
			name:        "公開できる情報が空なら削除する",
			profiles:    &stubScoutProfiles{allow: true, text: "   "},
			wantDeleted: []uint{7},
		},
		{
			name:     "同意状態が取れないときは何もしない",
			profiles: &stubScoutProfiles{allowErr: errors.New("db down")},
		},
		{
			name:     "テキスト構築に失敗したときは何もしない",
			profiles: &stubScoutProfiles{allow: true, textErr: errors.New("db down")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer := &stubIndexer{}
			NewStudentIndexSyncer(tt.profiles, indexer).Sync(context.Background(), 7)

			assert.Equal(t, tt.wantIndexed, nilIfEmptyStrings(indexer.indexed))
			assert.Equal(t, tt.wantDeleted, nilIfEmptyUints(indexer.deleted))
		})
	}
}

// TestStudentIndexSyncer_Sync_SwallowsRAGFailure は、RAGが落ちていても
// 呼び出し元（希望条件の保存・プロフィール更新）を失敗させないことを検証する。
func TestStudentIndexSyncer_Sync_SwallowsRAGFailure(t *testing.T) {
	indexer := &stubIndexer{indexErr: errors.New("rag down"), deleteErr: errors.New("rag down")}
	syncer := NewStudentIndexSyncer(&stubScoutProfiles{allow: true, text: "資格"}, indexer)

	require.NotPanics(t, func() { syncer.Sync(context.Background(), 7) })
	assert.Len(t, indexer.indexed, 1)
}

func TestStudentIndexSyncer_Sync_NoIndexerConfigured(t *testing.T) {
	// RAG未設定環境でも呼び出せる（プロフィール更新をブロックしない）
	require.NotPanics(t, func() {
		NewStudentIndexSyncer(&stubScoutProfiles{allow: true, text: "資格"}, nil).
			Sync(context.Background(), 7)
	})
}

func nilIfEmptyStrings(v []string) []string {
	if len(v) == 0 {
		return nil
	}
	return v
}

func nilIfEmptyUints(v []uint) []uint {
	if len(v) == 0 {
		return nil
	}
	return v
}
