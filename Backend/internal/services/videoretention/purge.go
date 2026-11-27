package videoretention

import (
	"context"
	"log"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"
)

const purgeBatch = 100

// ObjectDeleter は録画の S3 オブジェクトを消す。未設定なら、ファイルがある行は消さない。
type ObjectDeleter interface {
	DeleteObject(ctx context.Context, key string) error
}

// Store は期限を過ぎた録画を読み、行を消す。
type Store interface {
	RetentionGraceUntil(ctx context.Context) (time.Time, error)
	ListRetentionDue(ctx context.Context, now time.Time, limit int) ([]repositories.RetentionDueVideo, error)
	DeleteInterviewVideo(ctx context.Context, id uint) error
}

// Purger は学校の保持期間を過ぎた面接動画を消す。退会者の動画は退会の削除に任せる。
type Purger struct {
	store   Store
	objects ObjectDeleter
}

func NewPurger(store Store, objects ObjectDeleter) *Purger {
	return &Purger{store: store, objects: objects}
}

// Due は、その保持日数を過ぎていれば true。days が nil のときは期限なしで消さない。
func Due(now, recorded time.Time, days *int) bool {
	if days == nil || *days <= 0 {
		return false
	}
	return !recorded.After(now.AddDate(0, 0, -*days))
}

// Resolve は学生に見せる保持期間。学校が無い、学校の行が無いときは既定の 90 日。
func Resolve(school *models.School) (days *int, unlimited bool) {
	if school == nil || school.ID == 0 {
		value := models.DefaultInterviewVideoRetentionDays
		return &value, false
	}
	if school.InterviewVideoRetentionDays == nil {
		return nil, true
	}
	value := *school.InterviewVideoRetentionDays
	return &value, false
}

// Purge は猶予が終わるまで何も消さない。オブジェクトの削除に失敗した行は残し、次の回でやり直す。
func (p *Purger) Purge(ctx context.Context, now time.Time) (int, error) {
	if p == nil || p.store == nil {
		return 0, nil
	}
	grace, err := p.store.RetentionGraceUntil(ctx)
	if err != nil {
		return 0, err
	}
	if grace.IsZero() || now.Before(grace) {
		return 0, nil
	}
	deleted := 0
	for {
		due, err := p.store.ListRetentionDue(ctx, now, purgeBatch)
		if err != nil {
			return deleted, err
		}
		if len(due) == 0 {
			return deleted, nil
		}
		progress := false
		for _, video := range due {
			if video.ObjectKey != "" {
				if p.objects == nil {
					log.Printf("[VideoRetention] object deleter is not configured, skip video=%d", video.ID)
					continue
				}
				if err := p.objects.DeleteObject(ctx, video.ObjectKey); err != nil {
					log.Printf("[VideoRetention] object delete failed video=%d: %v", video.ID, err)
					continue
				}
			}
			if err := p.store.DeleteInterviewVideo(ctx, video.ID); err != nil {
				log.Printf("[VideoRetention] row delete failed video=%d: %v", video.ID, err)
				continue
			}
			deleted++
			progress = true
		}
		if !progress || len(due) < purgeBatch {
			return deleted, nil
		}
	}
}
