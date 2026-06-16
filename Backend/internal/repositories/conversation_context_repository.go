package repositories

import (
	"Backend/internal/models"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type ConversationContextRepository struct {
	db *gorm.DB
}

func NewConversationContextRepository(db *gorm.DB) *ConversationContextRepository {
	return &ConversationContextRepository{db: db}
}

func (r *ConversationContextRepository) GetBySessionID(sessionID string) (*models.ConversationContext, error) {
	var ctx models.ConversationContext
	if err := r.db.Where("session_id = ?", sessionID).First(&ctx).Error; err != nil {
		return nil, err
	}
	return &ctx, nil
}

func (r *ConversationContextRepository) GetOrCreate(userID uint, sessionID string) (*models.ConversationContext, error) {
	ctx, err := r.GetBySessionID(sessionID)
	if err == nil {
		return ctx, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	newCtx := &models.ConversationContext{
		UserID:         userID,
		SessionID:      sessionID,
		IndustryIDs:    "[]",
		JobCategoryIDs: "[]",
		AnswerHistory:  "[]",
	}
	if err := r.db.Create(newCtx).Error; err != nil {
		return nil, err
	}
	return newCtx, nil
}

func (r *ConversationContextRepository) SetJobCategoryID(userID uint, sessionID string, jobCategoryID uint) error {
	ctx, err := r.GetOrCreate(userID, sessionID)
	if err != nil {
		return err
	}
	ids, err := json.Marshal([]uint{jobCategoryID})
	if err != nil {
		return err
	}
	return r.db.Model(ctx).Update("job_category_ids", string(ids)).Error
}

func (r *ConversationContextRepository) GetJobCategoryID(sessionID string) (uint, error) {
	ctx, err := r.GetBySessionID(sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if strings.TrimSpace(ctx.JobCategoryIDs) == "" {
		return 0, nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(ctx.JobCategoryIDs), &ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return ids[0], nil
}

// SetSessionSummary はセッションに対する短い LLM 要約を保存します。既存レコードがなければ作成します。
func (r *ConversationContextRepository) SetSessionSummary(userID uint, sessionID string, summary string) error {
	ctx, err := r.GetOrCreate(userID, sessionID)
	if err != nil {
		return err
	}
	return r.db.Model(ctx).Updates(map[string]any{"llm_summary": summary}).Error
}
