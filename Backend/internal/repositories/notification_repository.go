package repositories

import (
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationRepository はアプリ内通知（#2033）の永続化。
// どの操作も user_id で絞り、他人の通知を読んだり既読にしたりできないようにする。
type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create は通知を1件保存する。同じ (user_id, dedupe_key) がすでにあれば何もしない（二重通知を防ぐ）。
// 一意制約以外のエラーまで握り潰す INSERT IGNORE は使わない（interview_utterance_repository と同じ方針）。
func (r *NotificationRepository) Create(n *models.Notification) error {
	if n.DedupeKey == nil {
		return r.db.Create(n).Error
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "dedupe_key"}},
		DoUpdates: clause.Assignments(map[string]any{"id": gorm.Expr("id")}),
	}).Create(n).Error
}

// ListForUser は新しい順に最大 limit 件を返す。beforeID が0より大きければ、それより古いものだけ（続きの読み込み）。
func (r *NotificationRepository) ListForUser(userID uint, limit int, beforeID uint) ([]models.Notification, error) {
	return r.list(r.db.Where("user_id = ?", userID), limit, beforeID)
}

// ListUnreadForUser は未読だけを新しい順に最大 limit 件返す。excludeKinds の種類は除く（ホームのお知らせ用）。
func (r *NotificationRepository) ListUnreadForUser(userID uint, limit int, excludeKinds []string) ([]models.Notification, error) {
	q := r.db.Where("user_id = ? AND read_at IS NULL", userID)
	if len(excludeKinds) > 0 {
		q = q.Where("kind NOT IN ?", excludeKinds)
	}
	return r.list(q, limit, 0)
}

func (r *NotificationRepository) list(q *gorm.DB, limit int, beforeID uint) ([]models.Notification, error) {
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var out []models.Notification
	err := q.Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

// UnreadCount は未読の件数を返す。
func (r *NotificationRepository) UnreadCount(userID uint) (int64, error) {
	var n int64
	err := r.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

// MarkRead は本人の通知1件を既読にする。本人の通知でなければ（存在しなくても）0件を返す。
func (r *NotificationRepository) MarkRead(userID, id uint, at time.Time) (int64, error) {
	res := r.db.Model(&models.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, userID).
		Update("read_at", at)
	return res.RowsAffected, res.Error
}

// MarkAllRead は本人の未読をすべて既読にする。
func (r *NotificationRepository) MarkAllRead(userID uint, at time.Time) error {
	return r.db.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", at).Error
}

// ScheduleEventsBetween は from 以上 to 未満に予定がある予定（締切・面接など）を返す（前日通知用）。
func (r *NotificationRepository) ScheduleEventsBetween(from, to time.Time) ([]models.ScheduleEvent, error) {
	var out []models.ScheduleEvent
	err := r.db.Where("scheduled_at >= ? AND scheduled_at < ?", from, to).Find(&out).Error
	return out, err
}
