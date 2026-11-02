package repositories

import (
	"errors"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// ESReviewRepository は ES添削の履歴（#1531）。
type ESReviewRepository struct {
	db *gorm.DB
}

func NewESReviewRepository(db *gorm.DB) *ESReviewRepository {
	return &ESReviewRepository{db: db}
}

func (r *ESReviewRepository) Create(review *models.ESReview) error {
	return r.db.Create(review).Error
}

// FindPreviousByQuestion は同一設問の直前の添削を1件返す。無ければ (nil, nil)。
//
// excludeID には今回保存した行を渡す。保存の後に引くので、渡さないと自分自身が
// 「前回」になって差分が常にゼロになる。
func (r *ESReviewRepository) FindPreviousByQuestion(userID uint, questionKey string, excludeID uint) (*models.ESReview, error) {
	var review models.ESReview
	err := r.db.Where("user_id = ? AND question_key = ? AND id < ?", userID, questionKey, excludeID).
		Order("id DESC").
		First(&review).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &review, nil
}

// ListByUser は本人の添削履歴を新しい順に返す（学生自身の画面用）。
func (r *ESReviewRepository) ListByUser(userID uint, limit int) ([]models.ESReview, error) {
	if limit <= 0 {
		limit = 20
	}
	var reviews []models.ESReview
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Limit(limit).Find(&reviews).Error
	return reviews, err
}

// ListForSchoolStudent は教員向けに、担当校の在籍学生1名の添削履歴を新しい順に返す。
//
// 学校だけで絞ると他学園の学生が混ざる。users.school_id はプロフィールの学校名
// （本人の自己申告）から解決されるため、学校名を他学園の校名に書き替えた学生が
// その学校の履歴として見えてしまう。学校の学園と本人の学園の一致を要求し、
// さらに保存時点の学園（es_reviews.organization_id）とも突き合わせる。
//
// 在籍中の student だけを対象にするのは、教員向けの一覧（ListStudentsPaged）と
// 見える範囲を揃えるため。一覧に出ない学生の履歴だけ見えるのはおかしい。
func (r *ESReviewRepository) ListForSchoolStudent(studentUserID, schoolID uint, limit int) ([]models.ESReview, error) {
	if limit <= 0 {
		limit = 50
	}
	var reviews []models.ESReview
	err := r.db.Table("es_reviews AS e").
		Select("e.*").
		Joins("JOIN users u ON u.id = e.user_id").
		Joins("JOIN schools s ON s.id = u.school_id AND s.organization_id = u.organization_id").
		Where("e.user_id = ? AND u.school_id = ? AND e.organization_id = u.organization_id", studentUserID, schoolID).
		Where("u.role = ? AND u.is_guest = ? AND u.is_admin = ? AND u.withdrawn_at IS NULL",
			models.UserRoleStudent, false, false).
		Order("e.id DESC").
		Limit(limit).
		Find(&reviews).Error
	return reviews, err
}
