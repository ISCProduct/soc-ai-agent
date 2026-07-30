package services

import (
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *InterviewService) advanceQuestionPlan(
	ctx context.Context,
	session *models.InterviewSession,
	companyID uint,
	position, companyName, userAnswer string,
	isStart bool,
) (*questionDirective, error) {
	if s.questionStateRepo == nil || companyID == 0 {
		return nil, nil
	}

	if err := s.persistInterviewContext(session, companyID, position, companyName); err != nil {
		return nil, err
	}

	count, err := s.questionStateRepo.CountBySessionID(session.ID)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		customQuestions := s.fetchCustomQuestions(companyID, position)
		queue := BuildQuestionQueue(session.ID, customQuestions)
		if err := s.questionStateRepo.CreateBatch(queue); err != nil {
			return nil, err
		}
	}

	if !isStart && strings.TrimSpace(userAnswer) != "" {
		if asked, err := s.questionStateRepo.FindLatestAsked(session.ID); err != nil {
			return nil, err
		} else if asked != nil {
			asked.Status = questionStatusAnswered
			if err := s.questionStateRepo.Update(asked); err != nil {
				return nil, err
			}

			if NeedsDeepening(userAnswer, asked.Depth) {
				followUpText := BuildFollowUpQuestionText(asked.QuestionText, userAnswer)
				if generated, genErr := s.generateFollowUpQuestion(ctx, asked.QuestionText, userAnswer); genErr == nil && strings.TrimSpace(generated) != "" {
					followUpText = generated
				}
				parentID := asked.ID
				followUp := &models.InterviewQuestionState{
					SessionID:     session.ID,
					Source:        questionSourceFollowUp,
					Category:      asked.Category,
					QuestionText:  followUpText,
					Status:        questionStatusAsked,
					ParentStateID: &parentID,
					Depth:         asked.Depth + 1,
					SortOrder:     asked.SortOrder,
				}
				if err := s.questionStateRepo.Create(followUp); err != nil {
					return nil, err
				}
				return &questionDirective{
					Text:        followUpText,
					Source:      questionSourceFollowUp,
					Category:    asked.Category,
					IsDeepening: true,
				}, nil
			}
		}
	}

	states, err := s.questionStateRepo.FindBySessionID(session.ID)
	if err != nil {
		return nil, err
	}
	next := selectNextPending(states)
	if next == nil {
		return nil, nil
	}
	next.Status = questionStatusAsked
	if err := s.questionStateRepo.Update(next); err != nil {
		return nil, err
	}
	return &questionDirective{
		Text:     next.QuestionText,
		Source:   next.Source,
		Category: next.Category,
	}, nil
}

func (s *InterviewService) persistInterviewContext(session *models.InterviewSession, companyID uint, position, companyName string) error {
	if companyID == 0 {
		return nil
	}
	changed := false
	if session.CompanyID != companyID {
		session.CompanyID = companyID
		changed = true
	}
	if position != "" && session.Position != position {
		session.Position = position
		changed = true
	}
	if companyName != "" && session.CompanyName != companyName {
		session.CompanyName = companyName
		changed = true
	}
	if !changed {
		return nil
	}
	return s.sessionRepo.Update(session)
}

func (s *InterviewService) generateFollowUpQuestion(ctx context.Context, originalQuestion, userAnswer string) (string, error) {
	if s.openaiClient == nil {
		return "", errors.New("openai client not configured")
	}
	systemPrompt := "あなたは就活面接官です。応募者の直前回答を踏まえ、具体性を引き出す追質問を1文だけ日本語で作成してください。余計な説明は不要です。"
	userPrompt := fmt.Sprintf("元の質問: %s\n応募者回答: %s", originalQuestion, userAnswer)
	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.openaiClient.ResponsesWithMaxTokens(ctxTimeout, systemPrompt, userPrompt, 0.2, 120)
}
