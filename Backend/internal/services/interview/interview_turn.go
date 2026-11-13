package interview

import (
	"Backend/internal/aifailover"
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"context"
	"log"
	"strings"
	"time"
)

// interviewFallbackReplyText はChat失敗時に返す面接官としての言い換え応答。
// 生のAPIエラーをユーザーに見せず、面接を自然に継続させるためのフォールバック(#910)。
const interviewFallbackReplyText = "すみません、うまく聞き取れませんでした。もう一度お答えいただけますか？"

// 面接官の文脈に渡す履歴の上限（#1636）。
// 30分の面接でも往復は20前後なので、直近40件で文脈は足りる。
// 1件の長さは SaveUtterance の上限と揃える。
const (
	maxHistoryMessages = 40
	maxHistoryRunes    = maxUtteranceRunes
)

// historyFromUtterances は保存済みの発話を ChatInterview の履歴形式に変換する。
// 直近 maxHistoryMessages 件に絞り、1件は maxHistoryRunes で切り詰める。
func historyFromUtterances(utterances []models.InterviewUtterance) []map[string]string {
	if len(utterances) > maxHistoryMessages {
		utterances = utterances[len(utterances)-maxHistoryMessages:]
	}
	history := make([]map[string]string, 0, len(utterances)+1)
	for _, u := range utterances {
		role := "user"
		if u.Role == "ai" {
			role = "assistant"
		}
		text := []rune(u.Text)
		if len(text) > maxHistoryRunes {
			text = text[:maxHistoryRunes]
		}
		history = append(history, map[string]string{"role": role, "content": string(text)})
	}
	return history
}

// Turn はユーザー音声を受け取り、STT→Chat→TTSを実行してTurnResultを返します
func (s *InterviewService) Turn(
	ctx context.Context,
	userID uint,
	sessionID uint,
	audioData []byte,
	companyName, companyReading string,
	companyReadingResolved bool,
	position, companyInfo, companyType string,
	companyID uint,
	turnCount int,
	remainingSeconds int,
	questionIndex int,
	totalQuestions int,
	questionElapsedSeconds int,
	questionDurationSeconds int,
) (*TurnResult, error) {
	ctx, cancelBudget := s.withTurnBudget(ctx)
	defer cancelBudget()
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

	type companyContextResult struct {
		id              uint
		reading         string
		readingResolved bool
		info            string
	}
	companyContextCh := make(chan companyContextResult, 1)
	go func() {
		result := companyContextResult{
			id:              companyID,
			reading:         companyReading,
			readingResolved: companyReadingResolved || strings.TrimSpace(companyReading) != "",
			info:            companyInfo,
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[Interview] company context panic: %v", recovered)
			}
			companyContextCh <- result
		}()
		result.id = s.resolveCompanyID(companyID, companyName)
		if companyName != "" && result.reading == "" && !result.readingResolved {
			result.reading = s.resolveCompanyReading(ctx, result.id, companyName)
			result.readingResolved = true
		}
		result.info = s.resolveCompanyInfoForPosition(result.id, companyName, position, companyInfo)
	}()

	// STT: Whisper でユーザー音声をテキスト化。
	// 破損音声・無音・タイムアウト等でTranscribe自体が失敗しても、ターンを
	// 中断させず「聞き取れなかった」扱いで継続する（既存の空文字フォールバックに合流、#910）。
	//
	// モデルごとの品質・費用を後から突き合わせられるよう、
	// 音声サイズ・応答時間・成否を記録する（音声R&D Task 1）。
	// 発話本文・認識本文は記録しない。
	// 面接コンテキストの語を補助語として渡す（音声R&D Task 4）。
	// 実測では固有名詞が改善し、無関係な語での幻覚は起きなかった。
	// 特に「御社」は mini が補助語なしだと8回中0回しか正しく取れない。
	// 形式はバイト列から判定する。以前は "audio.webm" 固定だったが、
	// MediaRecorder が出せる形式はブラウザによって違う（Safari は MP4）。
	// API は拡張子で形式を判断するため、食い違うと復号に失敗するか
	// 誤って解釈され、認識精度が落ちる。
	audioFormat := DetectAudioFormat(audioData)
	// 補助語に使う企業名・読みは DB 解決済みの値だけ。クライアント直値は使わない
	// （BuildSTTHints のコメント参照、#1600）。
	hintName, hintReading := s.sttHintCompany(companyID, companyName)
	sttHints := BuildSTTHints(hintName, hintReading, companyInfo)
	sttStart := time.Now()
	userText, sttOutcome, err := s.transcribe(ctx, audioData, audioFormat.AudioFilename(), sttHints)
	obs := ObserveTranscribe(sessionID, turnCount, len(audioData), audioFormat.MIME, sttStart, userText, err)
	// 利用量は OpenAI のモデルで記録し、観測ログはローカルの結果を OpenAI のモデルの値として残さない（#1833）
	usageObs := obs
	if sttOutcome.Local {
		obs.Model = localSTTModel
	}

	// 問題が疑われる結果だけ高精度モデルへ再送する（音声R&D Task 5）。
	// 通常の発話は再送しない。再送率がそのまま追加費用になる。
	// ローカルへ縮退した（OpenAI が障害中の）ターンは、OpenAI の高精度モデルへの再送をしない。
	// 同じ障害で失敗するまで待つことになり、面接のターンが止まるため（#1833）。
	if reason := NeedsSTTFallback(userText, err != nil, obs.AudioSeconds); reason != FallbackNone && !FallbackIsRedundant() && !sttOutcome.Local {
		// 費用は「再送したか」で決まるので、採用可否ではなくここで記録する
		obs.FellBack = true
		// 再送は「取れれば良い」もの。既定の60秒を待つと、そのターンの応答が
		// STTだけで最悪120秒かかる。面接の体感を優先して短く打ち切る。
		retryCtx, cancelRetry := context.WithTimeout(ctx, sttFallbackTimeout)
		retried, retryErr := s.openaiClient.TranscribeWithModel(
			retryCtx, audioData, audioFormat.AudioFilename(), sttHints, FallbackModel,
		)
		cancelRetry()
		// 再送に失敗しても面接は止めない。元の結果のまま続ける
		applied := ShouldUseFallbackResult(retried, retryErr)
		if applied {
			userText, err = retried, nil
			obs.ResultChars = len([]rune(strings.TrimSpace(retried)))
			obs.Succeeded = true
		}
		log.Printf("[Interview] stt fallback session=%d turn=%d reason=%s applied=%t",
			sessionID, turnCount, reason, applied)
	}
	LogSTTObservation(obs)
	// 利用量として記録する（#1294）。STT のレスポンスにはトークンが入らないため、
	// 既に計測している音声秒数とレイテンシをそのまま渡す。
	// 再送が発生した場合は2回課金されるので、その分も記録する。
	// OpenAI を試したら、失敗していても課金されている可能性があるので記録する。
	// Breaker で OpenAI を飛ばしたターンだけ記録しない（#1833）。
	if sttOutcome.PrimaryTried {
		usageObs.FellBack = obs.FellBack
		recordSTTUsage(ctx, s.openaiClient, usageObs)
	}

	if err != nil {
		log.Printf("[Interview] transcribe error: %v", err)
		userText = ""
	}
	if strings.TrimSpace(userText) == "" {
		userText = "（聞き取れませんでした）"
	}

	// WEB検索・手入力で company_id=0 でも、企業名が DB 登録と一致すれば解決する (#567)
	companyContext := <-companyContextCh
	companyID = companyContext.id

	// 読み仮名: 共有DB優先。無い場合のみモデル知識（Searchではない）
	if companyName != "" && companyReading == "" {
		companyReading = companyContext.reading
	}

	// 企業情報: 共有キャッシュ優先（general/sier 問わず）。無ければクライアント文面をフォールバック
	companyInfo = companyContext.info
	companyReadingResolved = companyContext.readingResolved

	// 企業別カスタム質問とGitHubスキルスコアを取得
	customQuestions := s.fetchCustomQuestions(companyID, position)
	skillScores := s.fetchSkillScores(userID)
	directive, err := s.advanceQuestionPlan(ctx, session, companyID, position, companyName, userText, false)
	if err != nil {
		log.Printf("[Interview] advanceQuestionPlan error: %v", err)
	}
	if directive != nil && directive.TotalQuestions > 0 {
		totalQuestions = directive.TotalQuestions
	}

	// 履歴はクライアントから受け取らず、保存済みの発話から組み直す（#1636）。
	// クライアントの history は件数・長さ・role のどれも信頼できない。
	// 取得に失敗しても面接は止めず、今回の発話だけで続ける。
	var utterances []models.InterviewUtterance
	utterancesLoaded := false
	if s.utterRepo != nil {
		if utterances, err = s.utterRepo.FindBySessionID(sessionID); err != nil {
			log.Printf("[Interview] load utterances for history error: %v", err)
			utterances = nil
		} else {
			utterancesLoaded = true
		}
	}
	// 同じターンが応答の前にもう一度走っても、発話の ID が同じになるようにする（二重保存の防止）
	turnKey := turnUtteranceKey("turn", len(utterances), utterancesLoaded)
	history := historyFromUtterances(utterances)
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
	// 経歴型の面接（#2274）: 軸スロットの質問（とその深掘り）のときだけ、本人の履歴書を材料として渡す。
	// 軸なしの面接・標準トピックの質問は従来のプロンプトのまま。
	if directive != nil && directive.TargetAxis != "" {
		if resumeCtx := s.buildResumeContext(session); resumeCtx != "" {
			systemPrompt = resumeCtx + "\n" + systemPrompt
		}
	}
	if s.crossFeature != nil {
		if profileCtx := s.crossFeature.BuildInterviewContextFromUser(userID); profileCtx != "" {
			systemPrompt = profileCtx + "\n" + systemPrompt
		}
	}
	if nameCtx := buildStudentNameContext(s.fetchStudentName(userID)); nameCtx != "" {
		systemPrompt = nameCtx + "\n" + systemPrompt
	}
	// Chat失敗時も面接官としての言い換え応答にフォールバックし、ターンを中断させない（#910）
	aiText, llmOutcome, err := s.chatInterview(ctx, systemPrompt, history)
	if err != nil {
		log.Printf("[Interview] chat error: %v", err)
		aiText = interviewFallbackReplyText
	} else {
		// 就職差別につながる質問が生成されたら1回だけ再生成し、それでも含まれるなら定型質問にする
		aiText = guardDiscriminatory(aiText, func() (string, error) {
			text, _, regenErr := s.chatInterview(ctx, systemPrompt, history)
			return text, regenErr
		})
	}

	// TTS: AI返答を音声化（企業名は読み仮名に置換して誤読を防ぐ。表示用のaiTextはそのまま保持）。
	// TTS失敗時もターンは中断させず、音声なし（テキストのみ）で返す（#910）。
	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	ttsText := applyCompanyReadingForTTS(aiText, companyName, companyReading)
	ttsStart := time.Now()
	audio, ttsOutcome, err := s.speak(ctx, ttsText, voice)
	// 失敗しても課金されている可能性があるため、成否に関わらず記録する（#1294）。
	// Breaker で OpenAI を飛ばしてローカルで読み上げたときだけ記録しない（#1833）。
	if ttsOutcome.PrimaryTried {
		recordTTSUsage(ctx, s.openaiClient, ttsText, time.Since(ttsStart))
	}
	if err != nil {
		log.Printf("[Interview] tts error: %v", err)
		audio = nil
	}

	// 書き起こしはサーバーが保存する（#1969）。ユーザー → 面接官の順に並べる。
	// 応答を返す直前に保存し、クライアントが切断・タイムアウトした（context が終わった）ターンは
	// 保存しない。届かなかった質問を書き起こしと次の履歴に残すと、会話とレポートが利用者の
	// 体験とずれる。以前のクライアント保存（応答を受け取ったターンだけ保存）と同じ扱いにする。
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	utteranceSaveFailed := false
	if err := s.saveTurnUtterances(sessionID, turnKey,
		turnUtterance{role: "user", text: userText},
		turnUtterance{role: "ai", text: aiText},
	); err != nil {
		log.Printf("[Interview] save turn utterances session=%d: %v", sessionID, err)
		utteranceSaveFailed = true
	}

	result := &TurnResult{
		UserText:               userText,
		AIText:                 aiText,
		Audio:                  audio,
		CompanyReading:         companyReading,
		CompanyReadingResolved: companyReadingResolved,
		ResolvedCompanyID:      companyID,
		CustomQuestionsEnabled: companyID > 0 && s.questionStateRepo != nil,
		AIFallback:             fallbackStages(sttOutcome, llmOutcome, ttsOutcome),
		UtteranceSaveFailed:    utteranceSaveFailed,
	}
	if directive != nil {
		result.QuestionSource = directive.Source
		result.QuestionCategory = directive.Category
		result.IsDeepening = directive.IsDeepening
		result.TotalQuestions = directive.TotalQuestions
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
	ctx, cancelBudget := s.withTurnBudget(ctx)
	defer cancelBudget()
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

	// 冒頭の質問の発話 ID を決める。同じ /start-turn が応答の前にもう一度走っても二重に保存しない
	startSaved, startLoaded := 0, false
	if s.utterRepo != nil {
		if existing, err := s.utterRepo.FindBySessionID(sessionID); err != nil {
			log.Printf("[Interview] load utterances for start-turn error: %v", err)
		} else {
			startSaved, startLoaded = len(existing), true
		}
	}
	turnKey := turnUtteranceKey("start", startSaved, startLoaded)

	// WEB検索・手入力で company_id=0 でも、企業名が DB 登録と一致すれば解決する (#567)
	companyID = s.resolveCompanyID(companyID, companyName)

	// 読み仮名: 共有DB優先。無い場合のみモデル知識（Searchではない）
	companyReadingResolved := strings.TrimSpace(companyReading) != ""
	if companyName != "" && companyReading == "" {
		companyReading = s.resolveCompanyReading(ctx, companyID, companyName)
		companyReadingResolved = true
	}

	// 企業情報: 共有キャッシュ優先（general/sier 問わず）。無ければクライアント文面をフォールバック
	companyInfo = s.resolveCompanyInfoForPosition(companyID, companyName, position, companyInfo)

	// 企業別カスタム質問とGitHubスキルスコアを取得
	customQuestions := s.fetchCustomQuestions(companyID, position)
	skillScores := s.fetchSkillScores(userID)
	directive, err := s.advanceQuestionPlan(ctx, session, companyID, position, companyName, "", true)
	if err != nil {
		log.Printf("[Interview] advanceQuestionPlan error: %v", err)
	}
	if directive != nil && directive.TotalQuestions > 0 {
		totalQuestions = directive.TotalQuestions
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
	if nameCtx := buildStudentNameContext(s.fetchStudentName(userID)); nameCtx != "" {
		systemPromptStart = nameCtx + "\n" + systemPromptStart
	}
	// Chat失敗時も面接開始を中断させず、定型の冒頭挨拶にフォールバックする（#910）
	startHistory := []map[string]string{
		{"role": "user", "content": "面接を開始してください。最初の自己紹介・志望動機の質問からお願いします。"},
	}
	aiText, llmOutcome, err := s.chatInterview(ctx, systemPromptStart, startHistory)
	if err != nil {
		log.Printf("[Interview] chat error: %v", err)
		aiText = "本日はよろしくお願いします。早速ですが、簡単に自己紹介をお願いできますか？"
	} else {
		aiText = guardDiscriminatory(aiText, func() (string, error) {
			text, _, regenErr := s.chatInterview(ctx, systemPromptStart, startHistory)
			return text, regenErr
		})
	}

	// TTS: 企業名は読み仮名に置換して誤読を防ぐ（表示用のaiTextはそのまま保持）。
	// TTS失敗時もターンは中断させず、音声なし（テキストのみ）で返す（#910）。
	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	ttsText := applyCompanyReadingForTTS(aiText, companyName, companyReading)
	ttsStart := time.Now()
	audio, ttsOutcome, err := s.speak(ctx, ttsText, voice)
	// 失敗しても課金されている可能性があるため、成否に関わらず記録する（#1294）。
	// Breaker で OpenAI を飛ばしてローカルで読み上げたときだけ記録しない（#1833）。
	if ttsOutcome.PrimaryTried {
		recordTTSUsage(ctx, s.openaiClient, ttsText, time.Since(ttsStart))
	}
	if err != nil {
		log.Printf("[Interview] tts error: %v", err)
		audio = nil
	}

	// 冒頭の質問もサーバーが書き起こしへ保存する（#1969）。届かなかった質問は保存しない。
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	utteranceSaveFailed := false
	if err := s.saveTurnUtterances(sessionID, turnKey, turnUtterance{role: "ai", text: aiText}); err != nil {
		log.Printf("[Interview] save start-turn utterance session=%d: %v", sessionID, err)
		utteranceSaveFailed = true
	}

	result := &TurnResult{
		AIText:                 aiText,
		Audio:                  audio,
		CompanyReading:         companyReading,
		CompanyReadingResolved: companyReadingResolved,
		ResolvedCompanyID:      companyID,
		CustomQuestionsEnabled: companyID > 0 && s.questionStateRepo != nil,
		AIFallback:             fallbackStages(aifailover.Outcome{}, llmOutcome, ttsOutcome),
		UtteranceSaveFailed:    utteranceSaveFailed,
	}
	if directive != nil {
		result.QuestionSource = directive.Source
		result.QuestionCategory = directive.Category
		result.IsDeepening = directive.IsDeepening
		result.TotalQuestions = directive.TotalQuestions
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
