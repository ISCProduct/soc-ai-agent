package services

import (
	"Backend/internal/models"
	"context"
	"log"
)

type jobCategoryResolutionResult struct {
	jobCategoryID   uint
	jobJustResolved bool
	earlyResponse   *ChatResponse
}

func (s *ChatService) resolveJobCategoryForChat(ctx context.Context, req ChatRequest, history []models.ChatMessage) (*jobCategoryResolutionResult, error) {
	result := &jobCategoryResolutionResult{}

	jobCategoryID := req.JobCategoryID
	storedJobCategoryID := uint(0)
	if s.conversationContextRepo != nil {
		if id, err := s.conversationContextRepo.GetJobCategoryID(req.SessionID); err == nil {
			storedJobCategoryID = id
		}
	}
	if jobCategoryID == 0 {
		jobCategoryID = storedJobCategoryID
	}
	if jobCategoryID != 0 && s.conversationContextRepo != nil && storedJobCategoryID != jobCategoryID {
		if err := s.conversationContextRepo.SetJobCategoryID(req.UserID, req.SessionID, jobCategoryID); err != nil {
			log.Printf("Warning: failed to store job category: %v\n", err)
		}
	}

	if jobCategoryID == 0 && s.shouldValidateJobCategory(history) {
		log.Printf("[JobValidation] Validating job category answer: %s\n", req.Message)
		jobValidation, err := s.jobValidator.ValidateJobCategory(ctx, req.Message)
		if err != nil {
			log.Printf("[JobValidation] Error: %v\n", err)
			// エラーでも続行
		} else if jobValidation != nil {
			if jobValidation.IsValid && len(jobValidation.MatchedCategories) > 0 {
				// 明確に職種が特定できた場合
				log.Printf("[JobValidation] Valid job category matched: %d categories\n", len(jobValidation.MatchedCategories))
				jobCategoryID = jobValidation.MatchedCategories[0].ID
				result.jobJustResolved = true
				if s.conversationContextRepo != nil {
					if err := s.conversationContextRepo.SetJobCategoryID(req.UserID, req.SessionID, jobCategoryID); err != nil {
						log.Printf("Warning: failed to store job category: %v\n", err)
					}
				}
			} else if jobValidation.NeedsClarification && jobValidation.SuggestedQuestion != "" {
				// 職種が曖昧な場合は選択肢を提示
				log.Printf("[JobValidation] Needs clarification, presenting options\n")

				assistantMsg := &models.ChatMessage{
					SessionID: req.SessionID,
					UserID:    req.UserID,
					Role:      "assistant",
					Content:   jobValidation.SuggestedQuestion,
				}
				if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
					log.Printf("Warning: failed to save assistant message: %v\n", err)
				}

				result.earlyResponse = &ChatResponse{
					Response:          jobValidation.SuggestedQuestion,
					IsComplete:        false,
					TotalQuestions:    15,
					AnsweredQuestions: 0,
				}
				return result, nil
			}
		}
	}

	if jobCategoryID != 0 {
		if err := s.completeJobAnalysisPhase(req.UserID, req.SessionID); err != nil {
			log.Printf("Warning: failed to complete job analysis phase: %v\n", err)
		}
	}

	result.jobCategoryID = jobCategoryID
	return result, nil
}
