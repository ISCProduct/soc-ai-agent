package chat

import (
	"Backend/internal/logsafe"
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
	"unicode/utf8"
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
		} else {
			log.Printf("[JobValidation] failed to load stored job category: %v\n", err)
		}
	}
	// リクエスト未指定時はセッション保存値を優先
	if jobCategoryID == 0 {
		jobCategoryID = storedJobCategoryID
	}
	// クライアント側で保持している ID をセッションへ同期
	if jobCategoryID != 0 && s.conversationContextRepo != nil && storedJobCategoryID != jobCategoryID {
		if err := s.conversationContextRepo.SetJobCategoryID(req.UserID, req.SessionID, jobCategoryID); err != nil {
			return nil, fmt.Errorf("failed to store job category: %w", err)
		}
		storedJobCategoryID = jobCategoryID
	}

	// 最初の回答が相談の意図なら、職種の判定（AI 呼び出し）を飛ばす。意図を保存し、次の質問で職種を聞く。
	// 意図の回答は質問数に数えない（診断の1問目は、職種の回答）。
	if jobCategoryID == 0 && req.ConsultIntent != "" && IsValidConsultIntent(req.ConsultIntent) && s.shouldValidateJobCategory(history) {
		if resp, handled, err := s.handleConsultIntent(ctx, req); err != nil {
			return nil, err
		} else if handled {
			result.earlyResponse = resp
			return result, nil
		}
	}

	if jobCategoryID == 0 && s.shouldValidateJobCategory(history) {
		log.Printf("[JobValidation] Validating job category answer: chars=%d\n", logsafe.Chars(req.Message))
		// 直前に提示した質問文を渡す。番号だけの回答は、そのとき並んでいた
		// 選択肢に対して解釈しないと別の職種になる（#1330）。
		presentedQuestion := findLastAssistantQuestion(history)
		jobValidation, err := s.jobValidator.ValidateJobCategoryWithQuestion(ctx, req.Message, presentedQuestion)
		if err != nil {
			log.Printf("[JobValidation] Error: %v\n", err)
			// 判定エラーでも会話は続行（職種未設定のまま）
		} else if jobValidation != nil {
			if jobValidation.IsValid && len(jobValidation.MatchedCategories) > 0 {
				log.Printf("[JobValidation] Valid job category matched: %d categories\n", len(jobValidation.MatchedCategories))
				jobCategoryID = jobValidation.MatchedCategories[0].ID
				result.jobJustResolved = true
				if s.conversationContextRepo != nil {
					if err := s.conversationContextRepo.SetJobCategoryID(req.UserID, req.SessionID, jobCategoryID); err != nil {
						return nil, fmt.Errorf("failed to store resolved job category: %w", err)
					}
				}
			} else if jobValidation.NeedsClarification && jobValidation.SuggestedQuestion != "" {
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
					TotalQuestions:    defaultTargetQuestions,
					AnsweredQuestions: 0,
					JobCategoryID:     0,
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

// handleConsultIntent は、セッションの最初の回答が相談の意図のときの応答を作る。
// 意図を保存し、職種を聞く質問を返す。既に意図が保存されているセッションでは何もしない（最初の選択を正とする）。
func (s *ChatService) handleConsultIntent(ctx context.Context, req ChatRequest) (*ChatResponse, bool, error) {
	if s.conversationContextRepo != nil {
		stored, err := s.conversationContextRepo.GetConsultIntent(req.SessionID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to load consult intent: %w", err)
		}
		if stored != "" {
			return nil, false, nil
		}
		if err := s.conversationContextRepo.SetConsultIntent(req.UserID, req.SessionID, req.ConsultIntent); err != nil {
			return nil, false, fmt.Errorf("failed to store consult intent: %w", err)
		}
	}
	// 本文はログに出さない（文字数だけ）
	log.Printf("[ConsultIntent] saved intent=%s chars=%d\n", req.ConsultIntent, utf8.RuneCountInString(req.Message))

	jobQuestion, err := s.jobValidator.GenerateJobSelectionQuestion(ctx)
	if err != nil {
		log.Printf("[ConsultIntent] job selection question failed: %v\n", err)
		jobQuestion = "どの職種に興味がありますか？職種名で答えても構いません。まだ決めていない場合は、そう答えてください。"
	}
	response := "相談内容を教えてくれてありがとうございます。まず、どの職種に興味があるかを聞かせてください。\n\n" + jobQuestion

	assistantMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      "assistant",
		Content:   response,
	}
	if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
		return nil, false, fmt.Errorf("failed to save job question: %w", err)
	}
	return &ChatResponse{
		Response:          response,
		IsComplete:        false,
		TotalQuestions:    defaultTargetQuestions,
		AnsweredQuestions: 0,
		JudgmentPercent:   0,
	}, true, nil
}
