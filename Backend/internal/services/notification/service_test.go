package notification

import (
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
)

// #2033: アプリ内通知。

// memStore は Store のメモリ実装。dedupe_key の一意制約と user_id の絞り込みを本物と同じにする。
type memStore struct {
	rows   []models.Notification
	events []models.ScheduleEvent
	fail   error
}

func (m *memStore) Create(n *models.Notification) error {
	if m.fail != nil {
		return m.fail
	}
	if n.DedupeKey != nil {
		for _, r := range m.rows {
			if r.UserID == n.UserID && r.DedupeKey != nil && *r.DedupeKey == *n.DedupeKey {
				return nil
			}
		}
	}
	n.ID = uint(len(m.rows) + 1)
	m.rows = append(m.rows, *n)
	return nil
}

func (m *memStore) ListForUser(userID uint, limit int, beforeID uint) ([]models.Notification, error) {
	var out []models.Notification
	for i := len(m.rows) - 1; i >= 0 && len(out) < limit; i-- {
		r := m.rows[i]
		if r.UserID == userID && (beforeID == 0 || r.ID < beforeID) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) UnreadCount(userID uint) (int64, error) {
	var n int64
	for _, r := range m.rows {
		if r.UserID == userID && r.ReadAt == nil {
			n++
		}
	}
	return n, nil
}

func (m *memStore) MarkRead(userID, id uint, at time.Time) (int64, error) {
	for i := range m.rows {
		if m.rows[i].ID == id && m.rows[i].UserID == userID && m.rows[i].ReadAt == nil {
			m.rows[i].ReadAt = &at
			return 1, nil
		}
	}
	return 0, nil
}

func (m *memStore) MarkAllRead(userID uint, at time.Time) error {
	for i := range m.rows {
		if m.rows[i].UserID == userID && m.rows[i].ReadAt == nil {
			m.rows[i].ReadAt = &at
		}
	}
	return nil
}

func (m *memStore) ScheduleEventsBetween(from, to time.Time) ([]models.ScheduleEvent, error) {
	var out []models.ScheduleEvent
	for _, e := range m.events {
		if !e.ScheduledAt.Before(from) && e.ScheduledAt.Before(to) {
			out = append(out, e)
		}
	}
	return out, nil
}

func TestNotify(t *testing.T) {
	store := &memStore{}
	s := NewService(store)

	s.Notify(Notice{UserID: 1, Kind: models.NotificationKindScout, Title: strings.Repeat("あ", 250), DedupeKey: "scout:1"})
	s.Notify(Notice{UserID: 1, Kind: models.NotificationKindScout, Title: "同じスカウト", DedupeKey: "scout:1"}) // 二重通知しない
	s.Notify(Notice{UserID: 0, Kind: "x", Title: "宛先なし"})                                                  // 作らない
	s.Notify(Notice{UserID: 1, Kind: "x", Title: "  "})                                                    // 見出しなしは作らない

	if len(store.rows) != 1 {
		t.Fatalf("件数 = %d, want 1", len(store.rows))
	}
	if got := len([]rune(store.rows[0].Title)); got != maxTitleRunes {
		t.Fatalf("見出しの長さ = %d, want %d（切り詰める）", got, maxTitleRunes)
	}

	// 保存に失敗しても呼び出し元を止めない（panic しない・エラーを返さない）
	NewService(&memStore{fail: errors.New("db down")}).Notify(Notice{UserID: 1, Kind: "x", Title: "t"})
	var nilSvc *Service
	nilSvc.Notify(Notice{UserID: 1, Kind: "x", Title: "t"})
}

func TestMarkRead_本人の通知だけ(t *testing.T) {
	store := &memStore{}
	s := NewService(store)
	s.Notify(Notice{UserID: 1, Kind: "x", Title: "本人"})
	s.Notify(Notice{UserID: 2, Kind: "x", Title: "他人"})

	if err := s.MarkRead(1, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("他人の通知を既読にできた: err = %v", err)
	}
	if err := s.MarkRead(1, 1); err != nil {
		t.Fatalf("本人の通知: %v", err)
	}
	if err := s.MarkRead(1, 1); err != nil {
		t.Fatalf("既読済みをもう一度既読にしても成功（冪等）: %v", err)
	}
	if n, _ := s.UnreadCount(1); n != 0 {
		t.Fatalf("未読 = %d", n)
	}
	if n, _ := s.UnreadCount(2); n != 1 {
		t.Fatalf("他人の未読が変わった: %d", n)
	}
	if err := s.MarkRead(1, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("存在しない通知: %v", err)
	}
}

func TestNotifyUpcomingSchedules(t *testing.T) {
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, jst)
	store := &memStore{events: []models.ScheduleEvent{
		{ID: 1, UserID: 5, Title: "A社 ES締切", ScheduledAt: now.Add(5 * time.Hour)},
		{ID: 2, UserID: 5, Title: "B社 面接", ScheduledAt: now.Add(30 * time.Hour)}, // 24時間より先は出さない
	}}
	s := NewService(store)
	s.now = func() time.Time { return now }

	s.NotifyUpcomingSchedules()
	s.NotifyUpcomingSchedules() // 定期処理が何度走っても1件
	if len(store.rows) != 1 {
		t.Fatalf("件数 = %d, want 1", len(store.rows))
	}
	r := store.rows[0]
	if r.UserID != 5 || r.Kind != models.NotificationKindDeadline || !strings.Contains(r.Title, "A社 ES締切") || r.Link != "/schedule" {
		t.Fatalf("通知 = %+v", r)
	}

	// 予定の日時を変えたら、改めて知らせる
	store.events[0].ScheduledAt = now.Add(6 * time.Hour)
	s.NotifyUpcomingSchedules()
	if len(store.rows) != 2 {
		t.Fatalf("日時を変えた予定の件数 = %d, want 2", len(store.rows))
	}
}

func TestApplicationStatusNotice(t *testing.T) {
	n, ok := ApplicationStatusNotice(3, 10, "offered", "サンプル株式会社")
	if !ok || n.UserID != 3 || n.Kind != models.NotificationKindApplication || n.DedupeKey != "application:10:offered" ||
		!strings.Contains(n.Body, "サンプル株式会社") {
		t.Fatalf("notice = %+v ok=%v", n, ok)
	}
	for _, st := range []string{"applied", "withdrawn", "accepted", "not_applied"} {
		if _, ok := ApplicationStatusNotice(3, 10, st, ""); ok {
			t.Fatalf("%s は学生自身の操作なので知らせない", st)
		}
	}
}
