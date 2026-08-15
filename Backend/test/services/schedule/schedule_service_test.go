package schedule_test

// スケジュールサービスのユニットテスト (Issue #188)
//
// 実行: cd Backend && go test ./test/services/... -run Schedule -v

import (
	"Backend/internal/models"
	"Backend/internal/services/schedule"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- mock ScheduleRepository ----

type mockScheduleRepo struct {
	events map[uint]*models.ScheduleEvent
	nextID uint
	errOn  string // メソッド名でエラーを注入
}

func newMockScheduleRepo() *mockScheduleRepo {
	return &mockScheduleRepo{events: map[uint]*models.ScheduleEvent{}, nextID: 1}
}

func (r *mockScheduleRepo) Create(event *models.ScheduleEvent) error {
	if r.errOn == "Create" {
		return errors.New("db error")
	}
	event.ID = r.nextID
	r.nextID++
	r.events[event.ID] = event
	return nil
}

func (r *mockScheduleRepo) FindByID(id uint) (*models.ScheduleEvent, error) {
	if r.errOn == "FindByID" {
		return nil, errors.New("db error")
	}
	ev, ok := r.events[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	copy := *ev
	return &copy, nil
}

func (r *mockScheduleRepo) Update(event *models.ScheduleEvent) error {
	if r.errOn == "Update" {
		return errors.New("db error")
	}
	r.events[event.ID] = event
	return nil
}

func (r *mockScheduleRepo) Delete(id uint) error {
	if r.errOn == "Delete" {
		return errors.New("db error")
	}
	delete(r.events, id)
	return nil
}

func (r *mockScheduleRepo) ListByUser(userID uint) ([]models.ScheduleEvent, error) {
	var result []models.ScheduleEvent
	for _, ev := range r.events {
		if ev.UserID == userID {
			result = append(result, *ev)
		}
	}
	return result, nil
}

func (r *mockScheduleRepo) ListByUserAndRange(userID uint, from, to time.Time) ([]models.ScheduleEvent, error) {
	return r.ListByUser(userID)
}

// ---- tests ----

func TestScheduleService_Create_Success(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	ev, err := svc.Create(1, "株式会社テスト", "es", "ES提出", time.Now().Add(24*time.Hour), "")
	require.NoError(t, err)
	assert.Equal(t, uint(1), ev.UserID)
	assert.Equal(t, "株式会社テスト", ev.CompanyName)
}

func TestScheduleService_Create_MissingCompanyName(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	_, err := svc.Create(1, "  ", "es", "ES提出", time.Now().Add(24*time.Hour), "")
	assert.Error(t, err, "company_nameが空の場合はエラー")
	assert.Contains(t, err.Error(), "company_name")
}

func TestScheduleService_Create_MissingScheduledAt(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	_, err := svc.Create(1, "テスト", "es", "ES提出", time.Time{}, "")
	assert.Error(t, err, "scheduled_atがゼロの場合はエラー")
	assert.Contains(t, err.Error(), "scheduled_at")
}

func TestScheduleService_Get_ForbiddenForOtherUser(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	ev, err := svc.Create(1, "テスト", "es", "ES提出", time.Now().Add(time.Hour), "")
	require.NoError(t, err)

	// 別ユーザーからアクセス
	_, err = svc.Get(2, ev.ID)
	assert.Error(t, err, "他ユーザーのイベントは取得不可")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestScheduleService_Get_Success(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	ev, err := svc.Create(1, "テスト", "es", "ES提出", time.Now().Add(time.Hour), "")
	require.NoError(t, err)

	got, err := svc.Get(1, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, "テスト", got.CompanyName)
}

func TestScheduleService_Delete_ForbiddenForOtherUser(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	ev, err := svc.Create(1, "テスト", "es", "ES提出", time.Now().Add(time.Hour), "")
	require.NoError(t, err)

	err = svc.Delete(2, ev.ID)
	assert.Error(t, err, "他ユーザーのイベントは削除不可")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestScheduleService_Delete_Success(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	ev, err := svc.Create(1, "テスト", "es", "ES提出", time.Now().Add(time.Hour), "")
	require.NoError(t, err)

	err = svc.Delete(1, ev.ID)
	require.NoError(t, err)

	// 削除後は取得できない
	_, err = svc.Get(1, ev.ID)
	assert.Error(t, err)
}

func TestScheduleService_Update_ForbiddenForOtherUser(t *testing.T) {
	repo := newMockScheduleRepo()
	svc := schedule.NewScheduleService(repo)

	ev, err := svc.Create(1, "テスト", "es", "ES提出", time.Now().Add(time.Hour), "")
	require.NoError(t, err)

	_, err = svc.Update(2, ev.ID, "新テスト", "", "", time.Time{}, "")
	assert.Error(t, err, "他ユーザーのイベントは更新不可")
	assert.Contains(t, err.Error(), "forbidden")
}
