package repositories

import (
	"errors"
	"fmt"
	"time"

	"Backend/internal/models"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrUserScoreRecalculationRunning = errors.New("user score recalculation job is already running")
var ErrUserScoreRecalculationBatchBusy = errors.New("user score recalculation batch is already running")
var ErrUserScoreRecalculationStatusChanged = errors.New("user score recalculation job status changed before update")

const mysqlDuplicateEntry = 1062

type UserScoreRecalculationRepository struct {
	db *gorm.DB
}

func NewUserScoreRecalculationRepository(db *gorm.DB) *UserScoreRecalculationRepository {
	return &UserScoreRecalculationRepository{db: db}
}

// Create はジョブを作成する。実行中・一時停止中のジョブは active_slot の一意制約で
// 1件に制限される。COUNT してから作る方式と違い、同時リクエストでも2件にならない。
func (r *UserScoreRecalculationRepository) Create(job *models.UserScoreRecalculationJob) error {
	err := r.db.Create(job).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry {
		return ErrUserScoreRecalculationRunning
	}
	return err
}

func (r *UserScoreRecalculationRepository) FindByID(id uint) (*models.UserScoreRecalculationJob, error) {
	var job models.UserScoreRecalculationJob
	if err := r.db.First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// CountSnapshots は元に戻せるセッションの数を返す。
func (r *UserScoreRecalculationRepository) CountSnapshots(jobID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.UserScoreRecalculationSnapshot{}).Where("job_id = ?", jobID).Count(&count).Error
	return count, err
}

// FindActive は実行中または一時停止中のジョブを返す。無ければ nil。
func (r *UserScoreRecalculationRepository) FindActive() (*models.UserScoreRecalculationJob, error) {
	var jobs []models.UserScoreRecalculationJob
	if err := r.db.
		Where("status IN ?", []string{models.UserScoreRecalculationJobRunning, models.UserScoreRecalculationJobPaused}).
		Limit(1).
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return &jobs[0], nil
}

func (r *UserScoreRecalculationRepository) UpdateStatus(id uint, status string) error {
	expectedStatus := models.UserScoreRecalculationJobRunning
	if status == models.UserScoreRecalculationJobRunning {
		expectedStatus = models.UserScoreRecalculationJobPaused
	} else if status != models.UserScoreRecalculationJobPaused {
		return fmt.Errorf("unsupported recalculation job status %q", status)
	}
	result := r.db.Model(&models.UserScoreRecalculationJob{}).
		Where("id = ? AND status = ?", id, expectedStatus).
		Updates(map[string]any{"status": status, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserScoreRecalculationStatusChanged
	}
	return nil
}

// Cancel は一時停止中のジョブを中止する。実行中のバッチが残っている間は中止しない。
// 中止すると次のジョブを作れるので、古いバッチと新しいジョブが同じセッションを
// 同時に書き換えないようにするため。
func (r *UserScoreRecalculationRepository) Cancel(id uint) error {
	now := time.Now()
	result := r.db.Model(&models.UserScoreRecalculationJob{}).
		Where("id = ? AND status = ? AND (batch_lease_until IS NULL OR batch_lease_until < ?)",
			id, models.UserScoreRecalculationJobPaused, now).
		Updates(map[string]any{"status": models.UserScoreRecalculationJobCancelled, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserScoreRecalculationStatusChanged
	}
	return nil
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

// Advance は1セッション分の進捗を保存する。failure は失敗の分類で、成功なら空。
func (r *UserScoreRecalculationRepository) Advance(id, cursor uint, token, failure string) error {
	updates := map[string]any{
		"cursor_score_id":    cursor,
		"processed_sessions": gorm.Expr("processed_sessions + 1"),
		"updated_at":         time.Now(),
	}
	if failure != "" {
		updates["failed_sessions"] = gorm.Expr("failed_sessions + 1")
		updates["last_error"] = failure
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
		case models.UserScoreRecalculationJobCompleted,
			models.UserScoreRecalculationJobPaused,
			models.UserScoreRecalculationJobCancelled:
			return nil
		default:
			return fmt.Errorf("job %d is not running", id)
		}
	}
	return nil
}
