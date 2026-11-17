package interview

import (
	"Backend/internal/models"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeVideoRepo struct {
	createErr error
	created   *models.InterviewVideo
	statusCh  chan string // UpdateStatus の status を流す
}

func (r *fakeVideoRepo) Create(_ context.Context, v *models.InterviewVideo) error {
	if r.createErr != nil {
		return r.createErr
	}
	v.ID = 42
	r.created = v
	return nil
}

func (r *fakeVideoRepo) UpdateStatus(_ context.Context, _ uint, status, _ string, _, _ string, _ *time.Time) error {
	r.statusCh <- status
	return nil
}

func (r *fakeVideoRepo) FindBySessionID(context.Context, uint) ([]models.InterviewVideo, error) {
	return nil, nil
}

func (r *fakeVideoRepo) FindByID(context.Context, uint) (*models.InterviewVideo, error) {
	return nil, nil
}

type fakeStore struct {
	err     error
	gotKey  string
	gotMIME string
}

func (s *fakeStore) UploadReader(_ context.Context, key, mimeType string, r io.Reader) (string, string, error) {
	s.gotKey, s.gotMIME = key, mimeType
	io.Copy(io.Discard, r)
	return "file-1", "https://example.test/file-1", s.err
}

type closeSpy struct {
	io.Reader
	closed atomic.Bool
}

func (c *closeSpy) Close() error { c.closed.Store(true); return nil }

func waitStatus(t *testing.T, ch chan string) string {
	t.Helper()
	select {
	case st := <-ch:
		return st
	case <-time.After(3 * time.Second):
		t.Fatal("UpdateStatus が呼ばれなかった")
		return ""
	}
}

func TestVideoUploadService_Start_TooLargeIsRejectedBeforeCreate(t *testing.T) {
	repo := &fakeVideoRepo{statusCh: make(chan string, 1)}
	svc := NewVideoUploadService(repo, &fakeStore{})
	f := &closeSpy{Reader: strings.NewReader("x")}

	_, err := svc.Start(context.Background(), 1, 2, MaxVideoSize+1, "video/webm", f)

	if !errors.Is(err, ErrVideoTooLarge) {
		t.Fatalf("err = %v, want ErrVideoTooLarge", err)
	}
	if repo.created != nil {
		t.Fatal("上限超過なのにレコードを作っている")
	}
	if !f.closed.Load() {
		t.Fatal("拒否したときも file を閉じる")
	}
}

func TestVideoUploadService_Start_CreateErrorClosesFile(t *testing.T) {
	repo := &fakeVideoRepo{createErr: errors.New("db down"), statusCh: make(chan string, 1)}
	svc := NewVideoUploadService(repo, &fakeStore{})
	f := &closeSpy{Reader: strings.NewReader("x")}

	if _, err := svc.Start(context.Background(), 1, 2, 1, "", f); err == nil {
		t.Fatal("Create の失敗が返らない")
	}
	if !f.closed.Load() {
		t.Fatal("Create に失敗したときも file を閉じる")
	}
}

func TestVideoUploadService_Start_UploadsAndMarksDone(t *testing.T) {
	repo := &fakeVideoRepo{statusCh: make(chan string, 1)}
	store := &fakeStore{}
	svc := NewVideoUploadService(repo, store)
	f := &closeSpy{Reader: strings.NewReader("video-bytes")}

	v, err := svc.Start(context.Background(), 7, 3, 11, "", f) // MIME 未指定
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != 42 || v.Status != "uploading" || v.MimeType != "video/webm" {
		t.Fatalf("record = %+v, want id=42 status=uploading mime=video/webm(既定)", v)
	}
	if got := waitStatus(t, repo.statusCh); got != "done" {
		t.Fatalf("status = %q, want done", got)
	}
	if !strings.HasPrefix(store.gotKey, "interview-videos/7/3_") || !strings.HasSuffix(store.gotKey, ".webm") {
		t.Fatalf("s3 key = %q, want interview-videos/7/3_<時刻>.webm", store.gotKey)
	}
	if store.gotMIME != "video/webm" {
		t.Fatalf("mime = %q", store.gotMIME)
	}
}

func TestVideoUploadService_Start_UploadFailureMarksError(t *testing.T) {
	repo := &fakeVideoRepo{statusCh: make(chan string, 1)}
	svc := NewVideoUploadService(repo, &fakeStore{err: errors.New("s3 down")})
	f := &closeSpy{Reader: strings.NewReader("x")}

	if _, err := svc.Start(context.Background(), 1, 2, 1, "video/mp4", f); err != nil {
		t.Fatal(err)
	}
	if got := waitStatus(t, repo.statusCh); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	// UpdateStatus の後に file が閉じられるまで少し待つ
	deadline := time.Now().Add(time.Second)
	for !f.closed.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !f.closed.Load() {
		t.Fatal("アップロードに失敗しても file を閉じる")
	}
}
