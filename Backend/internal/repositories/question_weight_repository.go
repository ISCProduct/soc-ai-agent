package repositories

import (
	"Backend/internal/models"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"gorm.io/gorm"
)

type QuestionWeightRepository struct {
	db *gorm.DB
}

func NewQuestionWeightRepository(db *gorm.DB) *QuestionWeightRepository {
	return &QuestionWeightRepository{db: db}
}

// CheckDuplicate 質問の重複チェック（質問文のハッシュで判定）
func (r *QuestionWeightRepository) CheckDuplicate(question string, weightCategory string) (bool, error) {
	var count int64
	err := r.db.Model(&models.QuestionWeight{}).
		Where("question = ? AND weight_category = ?", question, weightCategory).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 質問と重み係数を登録（重複チェック付き）
func (r *QuestionWeightRepository) Create(qw *models.QuestionWeight) error {
	exists, err := r.CheckDuplicate(qw.Question, qw.WeightCategory)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("同じ質問と重みカテゴリの組み合わせが既に存在します")
	}
	return r.db.Create(qw).Error
}

// FindByID IDで取得
func (r *QuestionWeightRepository) FindByID(id uint) (*models.QuestionWeight, error) {
	var qw models.QuestionWeight
	err := r.db.First(&qw, id).Error
	return &qw, err
}

// FindActiveByCategory カテゴリ別のアクティブな質問を取得
func (r *QuestionWeightRepository) FindActiveByCategory(category string) ([]models.QuestionWeight, error) {
	var questions []models.QuestionWeight
	err := r.db.Where("weight_category = ? AND is_active = ?", category, true).
		Find(&questions).Error
	return questions, err
}

// GetRandomQuestion ランダムに質問を1つ取得
func (r *QuestionWeightRepository) GetRandomQuestion(industryID, jobCategoryID uint) (*models.QuestionWeight, error) {
	var qw models.QuestionWeight
	err := r.db.Where("(industry_id = ? OR industry_id = 0 OR industry_id IS NULL) AND "+
		"(job_category_id = ? OR job_category_id = 0 OR job_category_id IS NULL) AND "+
		"is_active = ?", industryID, jobCategoryID, true).
		Order("RAND()").
		First(&qw).Error
	return &qw, err
}

// GetRandomQuestionExcluding 既に使用した質問を除外してランダムに取得
func (r *QuestionWeightRepository) GetRandomQuestionExcluding(industryID, jobCategoryID uint, excludeIDs []uint) (*models.QuestionWeight, error) {
	var qw models.QuestionWeight
	query := r.db.Where("(industry_id = ? OR industry_id = 0 OR industry_id IS NULL) AND "+
		"(job_category_id = ? OR job_category_id = 0 OR job_category_id IS NULL) AND "+
		"is_active = ?", industryID, jobCategoryID, true)

	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	err := query.Order("RAND()").First(&qw).Error
	return &qw, err
}

// GetRandomQuestionByCategory カテゴリを指定して質問を取得（既出を除外）
func (r *QuestionWeightRepository) GetRandomQuestionByCategory(category string, excludeIDs []uint) (*models.QuestionWeight, error) {
	var qw models.QuestionWeight
	query := r.db.Where("weight_category = ? AND is_active = ?", category, true)

	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	err := query.Order("RAND()").First(&qw).Error
	return &qw, err
}

// hashQuestion 質問文をハッシュ化
func hashQuestion(question string) string {
	hash := sha256.Sum256([]byte(question))
	return hex.EncodeToString(hash[:])
}
