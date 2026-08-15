package interview

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

// Turn はユーザー音声を受け取り、STT→Chat→TTSを実行してTurnResultを返します
func (s *InterviewService) Turn(
	ctx context.Context,
	userID uint,
	sessionID uint,
	audioData []byte,
	history []map[string]string,
	companyName,
	companyReading,
	position,
	companyInfo,
	companyType string,
	companyID uint,
	turnCount int,
	remainingSeconds int,
	questionIndex int,
	totalQuestions int,
	questionElapsedSeconds int,
	questionDurationSeconds int,
) (*TurnResult, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}

	// STT: Whisper でユーザー音声をテキスト化
	userText, err := s.openaiClient.Transcribe(ctx, audioData, "audio.webm")
	if err != nil {
		return nil, fmt.Errorf("transcribe error: %w", err)
	}
	if strings.TrimSpace(userText) == "" {
		userText = "（聞き取れませんでした）"
	}

	// WEB検索・手入力で company_id=0 でも、企業名が DB 登録と一致すれば解決する (#567)
	companyID = s.resolveCompanyID(companyID, companyName)

	// 読み仮名: 共有DB優先。無い場合のみモデル知識（Searchではない）
	if companyName != "" && companyReading == "" {
		companyReading = s.resolveCompanyReading(ctx, companyID, companyName)
	}

	// 企業情報: 共有キャッシュ優先（general/sier 問わず）。無ければクライアント文面をフォールバック
	companyInfo = s.resolveCompanyInfo(companyID, companyName, companyInfo)

	// 企業別カスタム質問とGitHubスキルスコアを取得
	customQuestions := s.fetchCustomQuestions(companyID, position)
	skillScores := s.fetchSkillScores(userID)
	directive, err := s.advanceQuestionPlan(ctx, session, companyID, position, companyName, userText, false)
	if err != nil {
		log.Printf("[Interview] advanceQuestionPlan error: %v", err)
	}

	// 履歴にユーザー発言を追加
	history = append(history, map[string]string{"role": "user", "content": userText})

	// Chat: 面接官として返答生成
	systemPrompt := buildInterviewSystemPrompt(
		companyName,
		companyReading,
		position,
		companyInfo,
		companyType,
		customQuestions,
		skillScores,
		turnCount,
		remainingSeconds,
		questionIndex,
		totalQuestions,
		questionElapsedSeconds,
		questionDurationSeconds,
		directive,
	)
	if s.crossFeature != nil {
		if profileCtx := s.crossFeature.BuildInterviewContextFromUser(userID); profileCtx != "" {
			systemPrompt = profileCtx + "\n" + systemPrompt
		}
	}
	aiText, err := s.openaiClient.ChatInterview(ctx, systemPrompt, history)
	if err != nil {
		return nil, fmt.Errorf("chat error: %w", err)
	}

	// TTS: AI返答を音声化
	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	audio, err := s.openaiClient.TTS(ctx, aiText, voice)
	if err != nil {
		return nil, fmt.Errorf("tts error: %w", err)
	}

	result := &TurnResult{
		UserText:               userText,
		AIText:                 aiText,
		Audio:                  audio,
		ResolvedCompanyID:      companyID,
		CustomQuestionsEnabled: companyID > 0 && s.questionStateRepo != nil,
	}
	if directive != nil {
		result.QuestionSource = directive.Source
		result.QuestionCategory = directive.Category
		result.IsDeepening = directive.IsDeepening
	}
	return result, nil
}

// StartTurn は面接開始の最初のAI発話を生成します
func (s *InterviewService) StartTurn(
	ctx context.Context,
	userID uint,
	sessionID uint,
	companyName,
	companyReading,
	position,
	companyInfo,
	companyType string,
	companyID uint,
	questionIndex int,
	totalQuestions int,
	questionElapsedSeconds int,
	questionDurationSeconds int,
) (*TurnResult, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}

	// WEB検索・手入力で company_id=0 でも、企業名が DB 登録と一致すれば解決する (#567)
	companyID = s.resolveCompanyID(companyID, companyName)

	// 読み仮名: 共有DB優先。無い場合のみモデル知識（Searchではない）
	if companyName != "" && companyReading == "" {
		companyReading = s.resolveCompanyReading(ctx, companyID, companyName)
	}

	// 企業情報: 共有キャッシュ優先（general/sier 問わず）。無ければクライアント文面をフォールバック
	companyInfo = s.resolveCompanyInfo(companyID, companyName, companyInfo)

	// 企業別カスタム質問とGitHubスキルスコアを取得
	customQuestions := s.fetchCustomQuestions(companyID, position)
	skillScores := s.fetchSkillScores(userID)
	directive, err := s.advanceQuestionPlan(ctx, session, companyID, position, companyName, "", true)
	if err != nil {
		log.Printf("[Interview] advanceQuestionPlan error: %v", err)
	}

	systemPromptStart := buildInterviewSystemPrompt(
		companyName,
		companyReading,
		position,
		companyInfo,
		companyType,
		customQuestions,
		skillScores,
		0,
		0,
		questionIndex,
		totalQuestions,
		questionElapsedSeconds,
		questionDurationSeconds,
		directive,
	)
	if s.crossFeature != nil {
		if profileCtx := s.crossFeature.BuildInterviewContextFromUser(userID); profileCtx != "" {
			systemPromptStart = profileCtx + "\n" + systemPromptStart
		}
	}
	aiText, err := s.openaiClient.ChatInterview(ctx, systemPromptStart, []map[string]string{
		{"role": "user", "content": "面接を開始してください。最初の自己紹介・志望動機の質問からお願いします。"},
	})
	if err != nil {
		return nil, fmt.Errorf("chat error: %w", err)
	}

	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	audio, err := s.openaiClient.TTS(ctx, aiText, voice)
	if err != nil {
		return nil, fmt.Errorf("tts error: %w", err)
	}

	result := &TurnResult{
		AIText:                 aiText,
		Audio:                  audio,
		ResolvedCompanyID:      companyID,
		CustomQuestionsEnabled: companyID > 0 && s.questionStateRepo != nil,
	}
	if directive != nil {
		result.QuestionSource = directive.Source
		result.QuestionCategory = directive.Category
		result.IsDeepening = directive.IsDeepening
	}
	return result, nil
}
