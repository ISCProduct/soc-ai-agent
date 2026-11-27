package videoretention

import (
	"context"
	"errors"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"
)

func TestDue_UnlimitedIsNeverDeleted(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	recorded := now.AddDate(-2, 0, 0)
	if Due(now, recorded, nil) {
		t.Fatal("期限なしは消さない")
	}
	days := 90
	if !Due(now, now.AddDate(0, 0, -90), &days) {
		t.Fatal("90日ちょうどは期限超過")
	}
	if Due(now, now.AddDate(0, 0, -89), &days) {
		t.Fatal("89日はまだ残す")
	}
}

func TestResolve_NoSchoolUsesDefault(t *testing.T) {
	days, unlimited := Resolve(nil)
	if unlimited || days == nil || *days != 90 {
		t.Fatalf("days=%v unlimited=%v", days, unlimited)
	}
	days, unlimited = Resolve(&models.School{ID: 5})
	if !unlimited || days != nil {
		t.Fatalf("期限なし: days=%v unlimited=%v", days, unlimited)
	}
}

type fakeStore struct {
	grace time.Time
	due   []repositories.RetentionDueVideo
	gone  []uint
}

func (f *fakeStore) RetentionGraceUntil(context.Context) (time.Time, error) {
	return f.grace, nil
}
func (f *fakeStore) ListRetentionDue(context.Context, time.Time, int) ([]repositories.RetentionDueVideo, error) {
	return f.due, nil
}
func (f *fakeStore) DeleteInterviewVideo(_ context.Context, id uint) error {
	f.gone = append(f.gone, id)
	f.due = nil
	return nil
}

type fakeObjects struct {
	fail string
	keys []string
}

func (f *fakeObjects) DeleteObject(_ context.Context, key string) error {
	if key == f.fail {
		return errors.New("s3 down")
	}
	f.keys = append(f.keys, key)
	return nil
}

func TestPurge_WaitsForGrace(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{
		grace: now.Add(24 * time.Hour),
		due:   []repositories.RetentionDueVideo{{ID: 1, ObjectKey: "interview-videos/1/a.webm"}},
	}
	n, err := NewPurger(store, &fakeObjects{}).Purge(context.Background(), now)
	if err != nil || n != 0 || len(store.gone) != 0 {
		t.Fatalf("grace should keep the video: n=%d err=%v gone=%v", n, err, store.gone)
	}
}

func TestPurge_KeepsTheRowWhenObjectDeleteFails(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{
		grace: now.Add(-time.Hour),
		due: []repositories.RetentionDueVideo{
			{ID: 1, ObjectKey: "interview-videos/1/a.webm"},
			{ID: 2, ObjectKey: "interview-videos/1/b.webm"},
		},
	}
	objects := &fakeObjects{fail: "interview-videos/1/a.webm"}
	n, err := NewPurger(store, objects).Purge(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(store.gone) != 1 || store.gone[0] != 2 {
		t.Fatalf("deleted row of the failed object: n=%d gone=%v", n, store.gone)
	}
	if len(objects.keys) != 1 || objects.keys[0] != "interview-videos/1/b.webm" {
		t.Fatalf("objects: %v", objects.keys)
	}
}
