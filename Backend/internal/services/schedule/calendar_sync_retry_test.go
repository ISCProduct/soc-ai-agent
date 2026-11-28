package schedule

import (
	"Backend/internal/models"
	"context"
	"errors"
	"testing"
	"time"
)

type memScheduleRepo struct {
	events map[uint]*models.ScheduleEvent
	nextID uint
}

func (r *memScheduleRepo) Create(event *models.ScheduleEvent) error {
	if r.events == nil {
		r.events = map[uint]*models.ScheduleEvent{}
	}
	r.nextID++
	event.ID = r.nextID
	copied := *event
	r.events[event.ID] = &copied
	return nil
}

func (r *memScheduleRepo) FindByID(id uint) (*models.ScheduleEvent, error) {
	ev, ok := r.events[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	copied := *ev
	return &copied, nil
}

func (r *memScheduleRepo) Update(event *models.ScheduleEvent) error {
	copied := *event
	r.events[event.ID] = &copied
	return nil
}

func (r *memScheduleRepo) Delete(id uint) error {
	delete(r.events, id)
	return nil
}

func (r *memScheduleRepo) ListByUser(uint) ([]models.ScheduleEvent, error) {
	return nil, nil
}

func (r *memScheduleRepo) ListByUserAndRange(uint, time.Time, time.Time) ([]models.ScheduleEvent, error) {
	return nil, nil
}

type flakyCalendar struct {
	calls int
	fail  int
	done  chan struct{}
}

func (f *flakyCalendar) CreateEvent(context.Context, uint, *models.ScheduleEvent) error {
	f.calls++
	if f.calls <= f.fail {
		return &retryableCalendarError{msg: "unexpected status 503"}
	}
	if f.done != nil {
		close(f.done)
	}
	return nil
}

func (f *flakyCalendar) UpdateEvent(context.Context, uint, *models.ScheduleEvent) error { return nil }
func (f *flakyCalendar) DeleteEvent(context.Context, uint, string) error                { return nil }

func TestCreateRetriesTransientCalendarFailure(t *testing.T) {
	orig := calendarSyncPause
	calendarSyncPause = func(int) {}
	t.Cleanup(func() { calendarSyncPause = orig })

	fake := &flakyCalendar{fail: 2, done: make(chan struct{})}
	svc := NewScheduleService(&memScheduleRepo{})
	svc.calendarSync = fake

	_, err := svc.Create(1, "テスト株式会社", "interview", "一次", time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("予定の保存が失敗した: %v", err)
	}
	select {
	case <-fake.done:
	case <-time.After(2 * time.Second):
		t.Fatal("カレンダー同期が成功までやり直さなかった")
	}
	if fake.calls != 3 {
		t.Fatalf("同期の呼び出し回数 = %d, 期待 3（2回失敗して3回目で成功）", fake.calls)
	}
}

func TestSyncCalendarStopsOnPermanentFailure(t *testing.T) {
	orig := calendarSyncPause
	calendarSyncPause = func(int) {}
	t.Cleanup(func() { calendarSyncPause = orig })

	calls := 0
	syncCalendar("CreateEvent", 1, func() error {
		calls++
		return errors.New("google calendar not connected")
	})
	if calls != 1 {
		t.Fatalf("恒久的な失敗なのに %d 回呼んだ", calls)
	}
}

func TestCalendarStatusErrorIsRetryableOnlyForTransientHTTP(t *testing.T) {
	if !isRetryableCalendar(calendarStatusError(503)) || !isRetryableCalendar(calendarStatusError(429)) {
		t.Fatal("503 と 429 は再試行する")
	}
	if isRetryableCalendar(calendarStatusError(400)) || isRetryableCalendar(calendarStatusError(404)) {
		t.Fatal("4xx（429以外）は再試行しない")
	}
}
