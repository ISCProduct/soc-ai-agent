package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"context"
	"errors"
	"fmt"
	"log"
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
		axes := s.selectInterviewAxes(companyID, position)
		queue := BuildQuestionQueueWithAxes(session.ID, customQuestions, axes, companyQuestionHintsFromContext(ctx))
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

			if NeedsDeepening(userAnswer, asked.Depth) {
				followUpText := BuildFollowUpQuestionText(asked.QuestionText, userAnswer)
				targetAxis := ""
				if asked.TargetAxis != nil {
					targetAxis = *asked.TargetAxis
				}
				brief := s.lookupCompanyProfile(companyID, companyName, position)
				if generated, genErr := s.generateFollowUpQuestion(ctx, asked.QuestionText, userAnswer, brief, targetAxis); genErr == nil && strings.TrimSpace(generated) != "" {
					// 就職差別につながる質問が生成されたら採用せず、テンプレートの追質問のままにする。
					// 追質問は追加のLLM呼び出しを増やさないため、再生成はしない。
					if ContainsDiscriminatoryQuestion(generated) {
						log.Printf("[Interview] follow-up discarded (discriminatory) session=%d", session.ID)
					} else {
						followUpText = generated
					}
				}
				parentID := asked.ID
				followUp := &models.InterviewQuestionState{
					SessionID:         session.ID,
					Source:            questionSourceFollowUp,
					Category:          asked.Category,
					CompanyQuestionID: asked.CompanyQuestionID,
					TargetAxis:        asked.TargetAxis,
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
					TargetAxis:        targetAxis,
					IsDeepening:       true,
					TotalQuestions:    totalQuestions + 1,
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
	nextAxis := ""
	if next.TargetAxis != nil {
		nextAxis = *next.TargetAxis
	}
	return &questionDirective{
		Text:              next.QuestionText,
		Source:            next.Source,
		Category:          next.Category,
		CompanyQuestionID: next.CompanyQuestionID,
		TargetAxis:        nextAxis,
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

func (s *InterviewService) generateFollowUpQuestion(ctx context.Context, originalQuestion, userAnswer, companyBrief, targetAxis string) (string, error) {
	if s.openaiClient == nil {
		return "", errors.New("openai client not configured")
	}
	systemPrompt := "あなたは就活面接官です。応募者の直前回答を踏まえ、具体性を引き出す追質問を1文だけ日本語で作成してください。必要に応じて、取り組みのきっかけや継続の方法にも触れてください。" +
		"企業情報が渡された場合も、企業が求める人物像を断定せず（「御社は〜を求めている」と言わない）、企業情報に書かれた事実だけを使ってください。" +
		"家族構成・出身地・国籍・宗教・思想など、適性・能力と関係のない事項は聞かないでください。余計な説明は不要です。"
	userPrompt := buildFollowUpUserPrompt(originalQuestion, userAnswer, companyBrief, targetAxis)
	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.openaiClient.ResponsesWithMaxTokens(ctxTimeout, systemPrompt, userPrompt, 0.2, 120)
}

// buildFollowUpUserPrompt は追質問生成のユーザープロンプトを組み立てる。
//
// STAR で欠けている要素を明示して、追質問の狙いを固定する（#794）。
// 指定しないと、成果が語られていない回答にも「きっかけ」を聞き返すことがある。
//
// companyBrief（DB の検証済み企業情報）と targetAxis（軸スロットの深掘りのとき）を渡す（#2274）。
// 企業情報は DB 由来でも Web 取得の文面を含み得るので、非信頼テキストとして囲む（#1600）。
// 軸の名前は面接官への指示にだけ使い、応募者へ伝えさせない。
func buildFollowUpUserPrompt(originalQuestion, userAnswer, companyBrief, targetAxis string) string {
	prompt := fmt.Sprintf("元の質問: %s\n応募者回答:\n%s", originalQuestion, shared.WrapUntrustedText(userAnswer, "応募者回答"))
	if _, missing := STARFollowUpQuestion(userAnswer); missing != "" {
		label := STARLabel(missing)
		prompt += fmt.Sprintf("\n\nこの回答には「%s」が含まれていません。「%s」を引き出す質問にしてください。", label, label)
	}
	if axis := axisByKey(targetAxis); axis.Key != "" {
		prompt += fmt.Sprintf("\n\nこの質問は「%s」を示す行動を見るためのものです。その観点（%s）で深掘りしてください。観点の名前は応募者に伝えないでください。", axis.Label, axis.Look)
	}
	if wrapped := shared.WrapUntrustedText(companyBrief, "企業情報"); wrapped != "" {
		prompt += "\n\n【企業情報】\n" + wrapped
	}
	return prompt
}
