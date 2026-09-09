package auth

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeScoutDeleter は EnsureDeleted を持つ同期先。呼ばれたIDを記録する。
type fakeScoutDeleter struct {
	deleted []uint
	failFor map[uint]bool
}

func (f *fakeScoutDeleter) Sync(_ context.Context, _ uint) {}

func (f *fakeScoutDeleter) EnsureDeleted(_ context.Context, userID uint) error {
	f.deleted = append(f.deleted, userID)
	if f.failFor[userID] {
		return errors.New("chroma unavailable")
	}
	return nil
}

// EnsureDeleted を持たない同期先（旧実装相当）
type syncOnlySyncer struct{}

func (syncOnlySyncer) Sync(_ context.Context, _ uint) {}

func newReconcileService(t *testing.T, syncer ScoutIndexSyncer) (*UserDeletionService, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	s := NewUserDeletionService(db, nil, nil)
	s.SetScoutIndexSyncer(syncer)
	return s, mock
}

func expectPending(mock sqlmock.Sqlmock, userIDs ...uint) {
	rows := sqlmock.NewRows([]string{"id", "user_id", "purged_at"})
	for i, id := range userIDs {
		rows.AddRow(i+1, id, nil)
	}
	mock.ExpectQuery("withdrawn_users").WillReturnRows(rows)
}

// 退会時のベクトル削除はRAG障害でも退会を止めないためエラーを飲む。
// その取りこぼしを日次で消し直せることを確認する。
func TestReconcileScoutIndex_DeletesPendingWithdrawals(t *testing.T) {
	f := &fakeScoutDeleter{}
	s, mock := newReconcileService(t, f)
	expectPending(mock, 11, 22, 33)

	attempted, failed, err := s.ReconcileScoutIndex(context.Background())
	if err != nil {
		t.Fatalf("ReconcileScoutIndex() error = %v", err)
	}
	if attempted != 3 || failed != 0 {
		t.Errorf("attempted=%d failed=%d, want 3/0", attempted, failed)
	}
	if len(f.deleted) != 3 {
		t.Fatalf("削除を試みた件数 = %d, want 3 (%v)", len(f.deleted), f.deleted)
	}
	for i, want := range []uint{11, 22, 33} {
		if f.deleted[i] != want {
			t.Errorf("deleted[%d] = %d, want %d", i, f.deleted[i], want)
		}
	}
}

// 1件失敗しても残りを止めない。止めると後続のユーザーが永久に消えなくなる。
func TestReconcileScoutIndex_ContinuesAfterFailure(t *testing.T) {
	f := &fakeScoutDeleter{failFor: map[uint]bool{22: true}}
	s, mock := newReconcileService(t, f)
	expectPending(mock, 11, 22, 33)

	attempted, failed, err := s.ReconcileScoutIndex(context.Background())
	if err != nil {
		t.Fatalf("ReconcileScoutIndex() error = %v", err)
	}
	if attempted != 3 {
		t.Errorf("attempted = %d, want 3", attempted)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1 (失敗件数が見えないと放置に気づけない)", failed)
	}
	if len(f.deleted) != 3 {
		t.Errorf("失敗後も後続を処理していない: %v", f.deleted)
	}
}

// パージ済みは対象外。SQL に purged_at IS NULL が無いと、
// 物理削除済みのユーザーにまで毎日削除要求を送り続ける。
func TestReconcileScoutIndex_SkipsPurged(t *testing.T) {
	f := &fakeScoutDeleter{}
	s, mock := newReconcileService(t, f)
	mock.ExpectQuery("purged_at IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "purged_at"}))

	if _, _, err := s.ReconcileScoutIndex(context.Background()); err != nil {
		t.Fatalf("ReconcileScoutIndex() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("purged_at IS NULL による絞り込みが無い: %v", err)
	}
}

// 同期先が未設定・EnsureDeleted 非対応でも落ちない（RAG無効構成）。
func TestReconcileScoutIndex_NoSyncer(t *testing.T) {
	for _, tt := range []struct {
		name   string
		syncer ScoutIndexSyncer
	}{
		{"未設定", nil},
		{"EnsureDeleted 非対応", syncOnlySyncer{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newReconcileService(t, tt.syncer)
			attempted, failed, err := s.ReconcileScoutIndex(context.Background())
			if err != nil || attempted != 0 || failed != 0 {
				t.Errorf("= (%d, %d, %v), want (0, 0, nil)", attempted, failed, err)
			}
		})
	}
}

// 対象の取得に失敗したらエラーを返す。握り潰すと「0件だった」と区別できない。
func TestReconcileScoutIndex_QueryError(t *testing.T) {
	f := &fakeScoutDeleter{}
	s, mock := newReconcileService(t, f)
	mock.ExpectQuery("withdrawn_users").WillReturnError(errors.New("db down"))

	if _, _, err := s.ReconcileScoutIndex(context.Background()); err == nil {
		t.Error("ReconcileScoutIndex() error = nil, want error")
	}
	if len(f.deleted) != 0 {
		t.Errorf("取得に失敗したのに削除を試みている: %v", f.deleted)
	}
}
