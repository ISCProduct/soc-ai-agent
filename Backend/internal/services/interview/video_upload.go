package interview

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/safego"
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// MaxVideoSize は面接動画1本の上限(500MB)。
// ルート側のボディ上限(routes/interview_routes.go)はこれに multipart のオーバーヘッドを足した値。
const MaxVideoSize = 500 << 20

// ErrVideoTooLarge は動画が MaxVideoSize を超えていることを表す。
var ErrVideoTooLarge = errors.New("video too large")

// VideoObjectStore は動画の保存先。*storage.S3UploadService が満たす。
type VideoObjectStore interface {
	UploadReader(ctx context.Context, key, mimeType string, r io.Reader) (fileID, url string, err error)
}

// VideoUploadService は面接動画のレコード作成と、保存先への非同期アップロードを担う。
type VideoUploadService struct {
	repo  repository.InterviewVideoRepository
	store VideoObjectStore
}

func NewVideoUploadService(repo repository.InterviewVideoRepository, store VideoObjectStore) *VideoUploadService {
	return &VideoUploadService{repo: repo, store: store}
}

// Start は動画レコード(status=uploading)を作り、アップロードを非同期で始める。
// file はこの関数が引き取り、アップロードの完了(または失敗)時に閉じる。
// 呼び出し側は、エラーが返ったときも返らなかったときも file を閉じなくてよい。
func (s *VideoUploadService) Start(ctx context.Context, sessionID, userID uint, size int64, mimeType string, file io.ReadCloser) (*models.InterviewVideo, error) {
	if size > MaxVideoSize {
		file.Close()
		return nil, ErrVideoTooLarge
	}
	if mimeType == "" {
		mimeType = "video/webm"
	}

	now := time.Now()
	stamp := now.Format("20060102_150405")
	s3Key := fmt.Sprintf("interview-videos/%d/%d_%s.webm", sessionID, userID, stamp)

	video := &models.InterviewVideo{
		SessionID:     sessionID,
		UserID:        userID,
		FileName:      fmt.Sprintf("interview_%d_%d_%s.webm", sessionID, userID, stamp),
		FileSizeBytes: size,
		MimeType:      mimeType,
		Status:        "uploading",
	}
	if err := s.repo.Create(ctx, video); err != nil {
		file.Close()
		return nil, err
	}

	// アップロードはリクエストより長く続くので、リクエストの ctx は使わない。
	safego.Go(func() {
		defer file.Close()
		bg := context.Background()
		fileID, url, err := s.store.UploadReader(bg, s3Key, video.MimeType, file)
		if err != nil {
			s.repo.UpdateStatus(bg, video.ID, "error", fmt.Sprintf("S3へのアップロードに失敗しました: %v", err), "", "", nil)
			return
		}
		uploadedAt := time.Now()
		s.repo.UpdateStatus(bg, video.ID, "done", "", fileID, url, &uploadedAt)
	})
	return video, nil
}
