package repositories

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 一括版は単体版(ListFinishedSessionIDsByUser)と同じ条件(終了済み・未削除)で、ユーザーごとに振り分ける。
func TestListFinishedSessionIDsByUsers(t *testing.T) {
	const q = `SELECT id, user_id FROM .interview_sessions. WHERE user_id IN \(\?,\?,\?\) AND status = \? AND deleted_at IS NULL`
	tests := []struct {
		name    string
		userIDs []uint
		setup   func(m sqlmock.Sqlmock)
		want    map[uint][]uint
		wantErr bool
	}{
		{
			name:    "空ならクエリを投げない",
			userIDs: nil,
			setup:   func(sqlmock.Sqlmock) {},
			want:    map[uint][]uint{},
		},
		{
			name:    "ユーザーごとに振り分け、セッションの無いユーザーはキー無し",
			userIDs: []uint{1, 2, 3},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(q).WithArgs(1, 2, 3, "finished").
					WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).
						AddRow(10, 1).AddRow(20, 2).AddRow(11, 1))
			},
			want: map[uint][]uint{1: {10, 11}, 2: {20}},
		},
		{
			name:    "DBエラーはそのまま返す",
			userIDs: []uint{1, 2, 3},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(q).WillReturnError(errors.New("db down"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			tt.setup(mock)
			got, err := NewInterviewSessionRepository(db).ListFinishedSessionIDsByUsers(context.Background(), tt.userIDs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				_, has3 := got[3]
				assert.False(t, has3)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
