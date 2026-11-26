package interview

import (
	"Backend/internal/models"
	"context"
	"fmt"
	"strings"
)

func (s *InterviewService) advanceQuestionPlan(
	ctx context.Context,
	session *models.InterviewSession,
	companyID uint,
	position, companyName, userAnswer string,
	isStart bool,
) (*questionDirective, error) {
	// companyID==0（WEB検索・手入力企業でDB未登録）でも標準トピック・深掘り機構は動かす。
	// カスタム質問取得のみ内部で companyID==0 をガードして空を返す（#881）。
	if s.questionStateRepo == nil {
		return nil, nil
	}

	if err := s.persistInterviewContext(session, companyID, position, companyName); err != nil {
		return nil, err
	}

	count, err := s.questionStateRepo.CountBySessionID(session.ID)
	if err != nil {
		return nil, err
	}
	totalQuestions := int(count)
	if count == 0 {
		customQuestions := s.fetchCustomQuestions(companyID, position)
		// 調査結果の読み取りに失敗しても質問キューは作る（候補なしか、今回のセッション限りの候補で続ける）。
		hints := s.confirmedResearchQuestions(ctx, companyName, position, companyQuestionHintsFromContext(ctx))
		queue := BuildQuestionQueue(session.ID, customQuestions, questionSourceAIResearch, hints...)
		totalQuestions = len(queue)
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

			if shouldDeepen(userAnswer, asked.Depth, s.skipScoreThreshold) {
				// 追質問の文は面接官の応答を作る1回の呼び出しで作る（ここで別に LLM を呼ぶと、
				// ターンの応答が1往復ぶん遅れる）。ここでは狙いの定型文だけ置き、実際の発話で上書きする。
				followUpText := BuildFollowUpQuestionText(asked.QuestionText, userAnswer)
				parentID := asked.ID
				followUp := &models.InterviewQuestionState{
					SessionID:         session.ID,
					Source:            questionSourceFollowUp,
					Category:          asked.Category,
					CompanyQuestionID: asked.CompanyQuestionID,
					QuestionText:      followUpText,
					Status:            questionStatusAsked,
					ParentStateID:     &parentID,
					Depth:             asked.Depth + 1,
					SortOrder:         asked.SortOrder,
				}
				if err := s.questionStateRepo.Create(followUp); err != nil {
					return nil, err
				}
				return &questionDirective{
					Text:              followUpText,
					Source:            questionSourceFollowUp,
					Category:          asked.Category,
					CompanyQuestionID: asked.CompanyQuestionID,
					IsDeepening:       true,
					TotalQuestions:    totalQuestions + 1,
					DeepeningFocus:    deepeningFocus(userAnswer),
					DeepeningOf:       asked.QuestionText,
					followUpState:     followUp,
				}, nil
			}
		}
	}

	states, err := s.questionStateRepo.FindBySessionID(session.ID)
	if err != nil {
		return nil, err
	}
	totalQuestions = len(states)
	next := selectNextPending(states)
	if next == nil {
		return nil, nil
	}
	next.Status = questionStatusAsked
	if err := s.questionStateRepo.Update(next); err != nil {
		return nil, err
	}
	return &questionDirective{
		Text:              next.QuestionText,
		Source:            next.Source,
		Category:          next.Category,
		CompanyQuestionID: next.CompanyQuestionID,
		TotalQuestions:    totalQuestions,
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

// deepeningFocus は深掘りで引き出す STAR の要素を指示する。
//
// 欠けている要素を明示して、追質問の狙いを固定する（#794）。
// 指定しないと、成果が語られていない回答にも「きっかけ」を聞き返すことがある。
func deepeningFocus(userAnswer string) string {
	if _, missing := STARFollowUpQuestion(userAnswer); missing != "" {
		label := STARLabel(missing)
		return fmt.Sprintf("直前の回答には「%s」が含まれていません。「%s」を引き出す質問にしてください。", label, label)
	}
	return ""
}
