package auth

import (
	"Backend/internal/models"
	"errors"

	"gorm.io/gorm"
)

// DeleteAccount ユーザーアカウントとその全データを削除する（個人情報保護法第28条対応）
func (s *AuthService) DeleteAccount(userID uint) error {
	if s.deletion == nil {
		s.rebuildDeletionService()
	}
	if s.deletion == nil {
		return errors.New("database not configured")
	}
	// リフレッシュトークンの失効は DeleteUser のトランザクション内で行われる (#616)
	return s.deletion.DeleteUser(userID, UserDeletionActor{Kind: "self"})
}

func collectUserSessionIDs(tx *gorm.DB, userID uint) ([]string, error) {
	sessions := map[string]struct{}{}
	collect := func(model any) error {
		var ids []string
		if err := tx.Model(model).
			Where("user_id = ?", userID).
			Distinct().
			Pluck("session_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if id != "" {
				sessions[id] = struct{}{}
			}
		}
		return nil
	}

	if err := collect(&models.ChatMessage{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserWeightScore{}); err != nil {
		return nil, err
	}
	if err := collect(&models.ConversationContext{}); err != nil {
		return nil, err
	}
	if err := collect(&models.AIGeneratedQuestion{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserAnalysisProgress{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserEmbedding{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserCompanyMatch{}); err != nil {
		return nil, err
	}
	if err := collect(&models.VariantAssignment{}); err != nil {
		return nil, err
	}
	if err := collect(&models.ResumeDocument{}); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(sessions))
	for id := range sessions {
		result = append(result, id)
	}
	return result, nil
}

func collectInterviewSessionIDs(tx *gorm.DB, userID uint) ([]uint, error) {
	var sessionIDs []uint
	if err := tx.Model(&models.InterviewSession{}).
		Where("user_id = ?", userID).
		Pluck("id", &sessionIDs).Error; err != nil {
		return nil, err
	}
	return sessionIDs, nil
}

func collectResumeDocumentIDs(tx *gorm.DB, userID uint) ([]uint, error) {
	var documentIDs []uint
	if err := tx.Model(&models.ResumeDocument{}).
		Where("user_id = ?", userID).
		Pluck("id", &documentIDs).Error; err != nil {
		return nil, err
	}
	return documentIDs, nil
}

func collectResumeReviewIDs(tx *gorm.DB, documentIDs []uint) ([]uint, error) {
	var reviewIDs []uint
	if err := tx.Model(&models.ResumeReview{}).
		Where("document_id IN ?", documentIDs).
		Pluck("id", &reviewIDs).Error; err != nil {
		return nil, err
	}
	return reviewIDs, nil
}
