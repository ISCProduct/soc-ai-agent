package repositories

import (
	"Backend/internal/models"
	"encoding/json"
	"gorm.io/gorm"
	"strings"
)

type PredefinedQuestionRepository struct {
	db *gorm.DB
}

func NewPredefinedQuestionRepository(db *gorm.DB) *PredefinedQuestionRepository {
	return &PredefinedQuestionRepository{db: db}
}

// helper: allowedForPhase checks AllowedPhases JSON; if empty -> allowed
func allowedForPhase(raw string, currentPhase string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	var phases []string
	if err := json.Unmarshal([]byte(raw), &phases); err != nil {
		// if parse error, be permissive
		return true
	}
	if currentPhase == "" {
		return true
	}
	for _, p := range phases {
		if p == currentPhase {
			return true
		}
	}
	return false
}

// FindActiveQuestions アクティブな質問を取得（currentPhase: 現在のフェーズ名）
func (r *PredefinedQuestionRepository) FindActiveQuestions(targetLevel string, industryID *uint, jobCategoryID *uint, currentPhase string) ([]*models.PredefinedQuestion, error) {
	var questions []*models.PredefinedQuestion

	query := r.db.Where("is_active = ?", true)

	if targetLevel != "" {
		query = query.Where("target_level IN (?)", []string{targetLevel, "両方"})
	}

	if industryID != nil {
		query = query.Where("industry_id IS NULL OR industry_id = ?", *industryID)
	}

	if jobCategoryID != nil {
		query = query.Where("job_category_id IS NULL OR job_category_id = ?", *jobCategoryID)
	}

	err := query.Order("priority DESC, id ASC").Find(&questions).Error
	if err != nil {
		return nil, err
	}

	// filter by AllowedPhases
	filtered := make([]*models.PredefinedQuestion, 0, len(questions))
	for _, q := range questions {
		if allowedForPhase(q.AllowedPhases, currentPhase) {
			filtered = append(filtered, q)
		}
	}

	return filtered, nil
}

// FindByID IDで質問を取得
func (r *PredefinedQuestionRepository) FindByID(id uint) (*models.PredefinedQuestion, error) {
	var question models.PredefinedQuestion
	err := r.db.First(&question, id).Error
	return &question, err
}

// Create 新しい質問を作成
func (r *PredefinedQuestionRepository) Create(question *models.PredefinedQuestion) error {
	return r.db.Create(question).Error
}

// Update 質問を更新
func (r *PredefinedQuestionRepository) Update(question *models.PredefinedQuestion) error {
	return r.db.Save(question).Error
}

// GetNextQuestion 次の質問を取得（まだ聞いていない質問から選択）
func (r *PredefinedQuestionRepository) GetNextQuestion(
	askedQuestionIDs []uint,
	targetLevel string,
	industryID *uint,
	jobCategoryID *uint,
	prioritizeCategory string,
	currentPhase string,
) (*models.PredefinedQuestion, error) {
	var question models.PredefinedQuestion

	query := r.db.Where("is_active = ?", true)

	// まだ聞いていない質問のみ
	if len(askedQuestionIDs) > 0 {
		query = query.Where("id NOT IN (?)", askedQuestionIDs)
	}

	// 対象レベル
	if targetLevel != "" {
		query = query.Where("target_level IN (?)", []string{targetLevel, "両方"})
	}

	// 業界フィルタ
	if industryID != nil {
		query = query.Where("industry_id IS NULL OR industry_id = ?", *industryID)
	}

	// 職種フィルタ
	if jobCategoryID != nil {
		query = query.Where("job_category_id IS NULL OR job_category_id = ?", *jobCategoryID)
	}

	// 優先カテゴリがあればそれを優先
	if prioritizeCategory != "" {
		var priorityQuestion models.PredefinedQuestion
		err := query.Where("category = ?", prioritizeCategory).
			Order("priority DESC, id ASC").
			First(&priorityQuestion).Error

		if err == nil && allowedForPhase(priorityQuestion.AllowedPhases, currentPhase) {
			return &priorityQuestion, nil
		}
	}

	// 優先度順に取得
	err := query.Order("priority DESC, id ASC").First(&question).Error
	if err != nil {
		return nil, err
	}

	if !allowedForPhase(question.AllowedPhases, currentPhase) {
		// not allowed for current phase, return nil to indicate none
		return nil, gorm.ErrRecordNotFound
	}

	return &question, nil
}
