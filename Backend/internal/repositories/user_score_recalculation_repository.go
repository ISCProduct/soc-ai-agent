package repositories

import (
	"errors"
	"fmt"
	"time"

	"Backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrUserScoreRecalculationRunning = errors.New("user score recalculation job is already running")
var ErrUserScoreRecalculationBatchBusy = errors.New("user score recalculation batch is already running")

type UserScoreRecalculationRepository struct {
	db *gorm.DB
}

func NewUserScoreRecalculationRepository(db *gorm.DB) *UserScoreRecalculationRepository {
	return &UserScoreRecalculationRepository{db: db}
}

func (r *UserScoreRecalculationRepository) Create(job *models.UserScoreRecalculationJob) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var active int64
		if err := tx.Model(&models.UserScoreRecalculationJob{}).
			Where("status IN ?", []string{models.UserScoreRecalculationJobRunning, models.UserScoreRecalculationJobPaused}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrUserScoreRecalculationRunning
		}
		return tx.Create(job).Error
	})
}

func (r *UserScoreRecalculationRepository) FindByID(id uint) (*models.UserScoreRecalculationJob, error) {
	var job models.UserScoreRecalculationJob
	if err := r.db.First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *UserScoreRecalculationRepository) UpdateStatus(id uint, status string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		expectedStatus := models.UserScoreRecalculationJobRunning
		if status == models.UserScoreRecalculationJobRunning {
			expectedStatus = models.UserScoreRecalculationJobPaused
		} else if status != models.UserScoreRecalculationJobPaused {
			return fmt.Errorf("unsupported recalculation job status %q", status)
		}
		if status == models.UserScoreRecalculationJobRunning {
			var active int64
			if err := tx.Model(&models.UserScoreRecalculationJob{}).
				Where("status = ? AND id <> ?", models.UserScoreRecalculationJobRunning, id).
				Count(&active).Error; err != nil {
				return err
			}
			if active > 0 {
				return ErrUserScoreRecalculationRunning
			}
		}
		result := tx.Model(&models.UserScoreRecalculationJob{}).
			Where("id = ? AND status = ?", id, expectedStatus).
			Updates(map[string]any{"status": status, "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var current models.UserScoreRecalculationJob
			if err := tx.First(&current, id).Error; err != nil {
				return err
			}
			return fmt.Errorf("recalculation job %d changed status before update", id)
		}
		return nil
	})
}

func (r *UserScoreRecalculationRepository) ClaimBatch(id uint) (string, error) {
	token := uuid.NewString()
	now := time.Now()
	result := r.db.Model(&models.UserScoreRecalculationJob{}).
		Where("id = ? AND status = ? AND (batch_lease_until IS NULL OR batch_lease_until < ?)",
			id, models.UserScoreRecalculationJobRunning, now).
		Updates(map[string]any{
			"batch_lease_token": token,
			"batch_lease_until": now.Add(10 * time.Minute),
			"updated_at":        now,
		})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.Model(&models.UserScoreRecalculationJob{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return "", gorm.ErrRecordNotFound
		}
		return "", ErrUserScoreRecalculationBatchBusy
	}
	return token, nil
}

func (r *UserScoreRecalculationRepository) ReleaseBatch(id uint, token string) error {
	return r.db.Model(&models.UserScoreRecalculationJob{}).
		Where("id = ? AND batch_lease_token = ?", id, token).
		Updates(map[string]any{"batch_lease_token": nil, "batch_lease_until": nil, "updated_at": time.Now()}).
		Error
}

func (r *UserScoreRecalculationRepository) Advance(id, cursor uint, token string, failed bool, failure error) error {
	updates := map[string]any{
		"cursor_score_id":    cursor,
		"processed_sessions": gorm.Expr("processed_sessions + 1"),
		"updated_at":         time.Now(),
	}
	if failed {
		updates["failed_sessions"] = gorm.Expr("failed_sessions + 1")
		updates["last_error"] = failure.Error()
	}
	result := r.db.Model(&models.UserScoreRecalculationJob{}).
		Where("id = ? AND batch_lease_token = ?", id, token).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserScoreRecalculationRepository) CompleteIfFinished(id uint, finished bool) error {
	if !finished {
		return nil
	}
	result := r.db.Model(&models.UserScoreRecalculationJob{}).
		Where("id = ? AND status = ?", id, models.UserScoreRecalculationJobRunning).
		Updates(map[string]any{"status": models.UserScoreRecalculationJobCompleted, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		job, err := r.FindByID(id)
		if err != nil {
			return err
		}
		switch job.Status {
		case models.UserScoreRecalculationJobCompleted, models.UserScoreRecalculationJobPaused:
			return nil
		default:
			return fmt.Errorf("job %d is not running", id)
		}
	}
	return nil
}
