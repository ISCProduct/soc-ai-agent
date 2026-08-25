package interview

import (
	"Backend/internal/services/shared"
	"context"
	"log"
	"strings"
)

// interviewFallbackReplyText はChat失敗時に返す面接官としての言い換え応答。
// 生のAPIエラーをユーザーに見せず、面接を自然に継続させるためのフォールバック(#910)。
const interviewFallbackReplyText = "すみません、うまく聞き取れませんでした。もう一度お答えいただけますか？"

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
		return nil, shared.ErrForbidden
	}
	if session.Status == "finished" {
		return nil, shared.ErrSessionFinished
	}

	// STT: Whisper でユーザー音声をテキスト化。
	// 破損音声・無音・タイムアウト等でTranscribe自体が失敗しても、ターンを
	// 中断させず「聞き取れなかった」扱いで継続する（既存の空文字フォールバックに合流、#910）。
	userText, err := s.openaiClient.Transcribe(ctx, audioData, "audio.webm")
	if err != nil {
		log.Printf("[Interview] transcribe error: %v", err)
		userText = ""
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
	// Chat失敗時も面接官としての言い換え応答にフォールバックし、ターンを中断させない（#910）
	aiText, err := s.openaiClient.ChatInterview(ctx, systemPrompt, history)
	if err != nil {
		log.Printf("[Interview] chat error: %v", err)
		aiText = interviewFallbackReplyText
	}

	// TTS: AI返答を音声化（企業名は読み仮名に置換して誤読を防ぐ。表示用のaiTextはそのまま保持）。
	// TTS失敗時もターンは中断させず、音声なし（テキストのみ）で返す（#910）。
	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	audio, err := s.openaiClient.TTS(ctx, applyCompanyReadingForTTS(aiText, companyName, companyReading), voice)
	if err != nil {
		log.Printf("[Interview] tts error: %v", err)
		audio = nil
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
		return nil, shared.ErrForbidden
	}
	if session.Status == "finished" {
		return nil, shared.ErrSessionFinished
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
	// Chat失敗時も面接開始を中断させず、定型の冒頭挨拶にフォールバックする（#910）
	aiText, err := s.openaiClient.ChatInterview(ctx, systemPromptStart, []map[string]string{
		{"role": "user", "content": "面接を開始してください。最初の自己紹介・志望動機の質問からお願いします。"},
	})
	if err != nil {
		log.Printf("[Interview] chat error: %v", err)
		aiText = "本日はよろしくお願いします。早速ですが、簡単に自己紹介をお願いできますか？"
	}

	// TTS: 企業名は読み仮名に置換して誤読を防ぐ（表示用のaiTextはそのまま保持）。
	// TTS失敗時もターンは中断させず、音声なし（テキストのみ）で返す（#910）。
	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	audio, err := s.openaiClient.TTS(ctx, applyCompanyReadingForTTS(aiText, companyName, companyReading), voice)
	if err != nil {
		log.Printf("[Interview] tts error: %v", err)
		audio = nil
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

// applyCompanyReadingForTTS はTTS読み上げ用に、AI発話中の企業名（漢字等の表記）を
// 読み仮名に置換する。Chatモデルが読み仮名を無視して漢字表記のまま出力しても、
// TTSには常に正しい読みが渡るようにするための対策（表示用テキストは変更しない）。
func applyCompanyReadingForTTS(text, companyName, companyReading string) string {
	companyName = strings.TrimSpace(companyName)
	companyReading = strings.TrimSpace(companyReading)
	if companyName == "" || companyReading == "" || companyName == companyReading {
		return text
	}
	return strings.ReplaceAll(text, companyName, companyReading)
}
