package repositories

import (
	"Backend/internal/models"
	"time"

	"gorm.io/gorm"
)

type InterviewSessionRepository struct {
	db *gorm.DB
}

func NewInterviewSessionRepository(db *gorm.DB) *InterviewSessionRepository {
	return &InterviewSessionRepository{db: db}
}

func (r *InterviewSessionRepository) Create(session *models.InterviewSession) error {
	if err := fillOrganizationID(r.db, session.UserID, &session.OrganizationID); err != nil {
		return err
	}
	return r.db.Create(session).Error
}

func (r *InterviewSessionRepository) FindByID(id uint) (*models.InterviewSession, error) {
	var session models.InterviewSession
	if err := r.db.First(&session, id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *InterviewSessionRepository) Update(session *models.InterviewSession) error {
	return r.db.Save(session).Error
}

func (r *InterviewSessionRepository) ListByUser(userID uint, limit int, offset int) ([]models.InterviewSession, error) {
	var sessions []models.InterviewSession
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *InterviewSessionRepository) ListAll(limit int, offset int) ([]models.InterviewSession, error) {
	var sessions []models.InterviewSession
	query := r.db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *InterviewSessionRepository) CountByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.InterviewSession{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *InterviewSessionRepository) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&models.InterviewSession{}).Count(&count).Error
	return count, err
}

func (r *InterviewSessionRepository) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	var sessions []models.InterviewSession
	query := r.db.Where("user_id = ? AND status = ?", userID, "finished").Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// UserSessionStat はユーザー単位の集計統計
type UserSessionStat struct {
	UserID        uint
	SessionCount  int64
	LastSessionAt *time.Time
}

// GetUserStatsBatch は指定ユーザーIDリストの終了済みセッション統計を一括取得する
func (r *InterviewSessionRepository) GetUserStatsBatch(userIDs []uint) (map[uint]UserSessionStat, error) {
	type row struct {
		UserID        uint
		SessionCount  int64
		LastSessionAt *time.Time
	}
	var rows []row
	err := r.db.Model(&models.InterviewSession{}).
		Select("user_id, COUNT(*) as session_count, MAX(ended_at) as last_session_at").
		Where("user_id IN ? AND status = ? AND deleted_at IS NULL", userIDs, "finished").
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]UserSessionStat, len(rows))
	for _, r := range rows {
		result[r.UserID] = UserSessionStat{
			UserID:        r.UserID,
			SessionCount:  r.SessionCount,
			LastSessionAt: r.LastSessionAt,
		}
	}
	return result, nil
}

// ListFinishedSessionIDsByUser は指定ユーザーの終了済みセッションIDを返す
func (r *InterviewSessionRepository) ListFinishedSessionIDsByUser(userID uint) ([]uint, error) {
	var sessions []models.InterviewSession
	if err := r.db.Select("id").Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, "finished").Find(&sessions).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	return ids, nil
}

func (r *InterviewSessionRepository) CountByUserAndDay(userID uint, day time.Time) (int64, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.Add(24 * time.Hour)
	var count int64
	err := r.db.Model(&models.InterviewSession{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).
		Count(&count).Error
	return count, err
}
