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

func (s *stubScoutProfiles) IsScoutVisible(uint) (bool, error)     { return s.allow, s.allowErr }
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

// EnsureDeleted は Sync と違い、削除の成否を呼び出し元へ返す（#1204）。
// 退会済みユーザーの取りこぼしを日次で回収する経路が、
// 成功したかどうかを判断できる必要がある。
func TestEnsureDeleted(t *testing.T) {
	t.Run("削除を実行して成功を返す", func(t *testing.T) {
		idx := &stubIndexer{}
		s := NewStudentIndexSyncer(&stubScoutProfiles{}, idx)

		require.NoError(t, s.EnsureDeleted(context.Background(), 42))
		assert.Equal(t, []uint{42}, idx.deleted, "削除が実行されていない")
	})

	t.Run("失敗を握り潰さない", func(t *testing.T) {
		idx := &stubIndexer{deleteErr: errors.New("chroma unavailable")}
		s := NewStudentIndexSyncer(&stubScoutProfiles{}, idx)

		err := s.EnsureDeleted(context.Background(), 42)
		assert.Error(t, err, "Sync と違い、成否を返さないと再試行の要否が判断できない")
		assert.Equal(t, []uint{42}, idx.deleted)
	})

	t.Run("公開可否を見ずに必ず削除する", func(t *testing.T) {
		// 退会済みでも IsScoutVisible の取得が失敗しうる。
		// そこで止まると Sync と同じ穴になるので、EnsureDeleted は参照しない。
		idx := &stubIndexer{}
		s := NewStudentIndexSyncer(&stubScoutProfiles{allow: true, allowErr: errors.New("db down")}, idx)

		require.NoError(t, s.EnsureDeleted(context.Background(), 7))
		assert.Equal(t, []uint{7}, idx.deleted)
	})

	t.Run("未設定でも落ちない", func(t *testing.T) {
		var s *StudentIndexSyncer
		require.NoError(t, s.EnsureDeleted(context.Background(), 1))
		require.NoError(t, NewStudentIndexSyncer(&stubScoutProfiles{}, nil).EnsureDeleted(context.Background(), 1))
	})
}
