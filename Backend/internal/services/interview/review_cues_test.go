package interview

import (
	"Backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestReviewCues_UsesSessionStartAndHidesMissingAnchor(t *testing.T) {
	started := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	utterances := []models.InterviewUtterance{
		{ID: 7, Role: "user", CreatedAt: started.Add(42*time.Second + 400*time.Millisecond)},
		{ID: 8, Role: "ai", CreatedAt: started.Add(-2 * time.Second)},
	}

	got := ReviewCues(&started, utterances)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].OffsetSeconds == nil || *got[0].OffsetSeconds != 42 {
		t.Fatalf("user offset = %v, want 42", got[0].OffsetSeconds)
	}
	if got[1].OffsetSeconds == nil || *got[1].OffsetSeconds != 0 {
		t.Fatalf("early offset = %v, want 0", got[1].OffsetSeconds)
	}

	missing := ReviewCues(nil, utterances[:1])
	if missing[0].OffsetSeconds != nil {
		t.Fatal("開始時刻が無いのに再生位置を返した")
	}
}

type playbackRepo struct {
	byID      *models.InterviewVideo
	findErr   error
	bySession []models.InterviewVideo
}

func (r *playbackRepo) Create(context.Context, *models.InterviewVideo) error { return nil }
func (r *playbackRepo) UpdateStatus(context.Context, uint, string, string, string, string, *time.Time) error {
	return nil
}
func (r *playbackRepo) FindBySessionID(context.Context, uint) ([]models.InterviewVideo, error) {
	return r.bySession, nil
}
func (r *playbackRepo) FindByID(context.Context, uint) (*models.InterviewVideo, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.byID, nil
}

type playbackStore struct {
	key string
}

func (s *playbackStore) UploadReader(_ context.Context, _, _ string, r io.Reader) (string, string, error) {
	_, _ = io.Copy(io.Discard, r)
	return "", "", nil
}

func (s *playbackStore) PresignGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	s.key = key
	return "https://example.test/presigned", nil
}

func TestListForUser_DropsOtherUsersAndStorageKeys(t *testing.T) {
	repo := &playbackRepo{bySession: []models.InterviewVideo{
		{ID: 1, SessionID: 9, UserID: 3, Status: "done", MimeType: "video/webm", DriveFileID: "secret-key"},
		{ID: 2, SessionID: 9, UserID: 4, Status: "done", DriveFileID: "other-key"},
	}}
	svc := NewVideoUploadService(repo, &playbackStore{})

	got, err := svc.ListForUser(context.Background(), 9, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("videos = %+v", got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-key") || strings.Contains(string(raw), "drive_file") {
		t.Fatalf("storage key leaked: %s", raw)
	}
}

func TestPresignForUser_RejectsOtherSessionAndOtherUser(t *testing.T) {
	store := &playbackStore{}
	repo := &playbackRepo{byID: &models.InterviewVideo{
		ID: 1, SessionID: 9, UserID: 3, Status: "done", DriveFileID: "secret-key",
	}}
	svc := NewVideoUploadService(repo, store)

	if _, _, err := svc.PresignForUser(context.Background(), 9, 1, 4); err != ErrVideoNotFound {
		t.Fatalf("other user err = %v", err)
	}
	if _, _, err := svc.PresignForUser(context.Background(), 8, 1, 3); err != ErrVideoNotFound {
		t.Fatalf("other session err = %v", err)
	}
	url, _, err := svc.PresignForUser(context.Background(), 9, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://example.test/presigned" || store.key != "secret-key" {
		t.Fatalf("url=%s key=%s", url, store.key)
	}
}

func TestPresignForUser_NotFoundVsRepositoryError(t *testing.T) {
	dbErr := errors.New("connection refused")
	tests := []struct {
		name    string
		findErr error
		wantNF  bool
	}{
		{"レコードが無い場合は ErrVideoNotFound", gorm.ErrRecordNotFound, true},
		{"DB障害は ErrVideoNotFound にしない", dbErr, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewVideoUploadService(&playbackRepo{findErr: tt.findErr}, &playbackStore{})
			_, _, err := svc.PresignForUser(context.Background(), 9, 1, 3)
			if err == nil {
				t.Fatal("エラーが返らなかった")
			}
			if errors.Is(err, ErrVideoNotFound) != tt.wantNF {
				t.Fatalf("errors.Is(ErrVideoNotFound) = %v, want %v (err=%v)", !tt.wantNF, tt.wantNF, err)
			}
			if !tt.wantNF && !errors.Is(err, dbErr) {
				t.Fatalf("元のエラーが辿れない: %v", err)
			}
		})
	}
}
