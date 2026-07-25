package services

import (
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
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

// fetchCustomQuestions は企業別カスタム質問を取得する。未登録時は空スライスを返す。
func (s *InterviewService) fetchCustomQuestions(companyID uint, position string) []models.InterviewCompanyQuestion {
	if s.companyQuestionRepo == nil || companyID == 0 {
		return nil
	}
	qs, err := s.companyQuestionRepo.FindByCompanyAndPosition(companyID, position)
	if err != nil {
		log.Printf("[Interview] fetchCustomQuestions error: %v", err)
		return nil
	}
	return qs
}

// fetchSkillScores はユーザーのGitHubスキルスコアを取得する。未登録時は空スライスを返す。
func (s *InterviewService) fetchSkillScores(userID uint) []models.SkillScore {
	if s.skillScoreRepo == nil {
		return nil
	}
	scores, err := s.skillScoreRepo.GetScores(userID)
	if err != nil {
		log.Printf("[Interview] fetchSkillScores error: %v", err)
		return nil
	}
	return scores
}

// lookupCompanyReading はAIモデルの知識から企業名の日本語読み（ふりがな）を取得します。
// 取得に失敗した場合は空文字を返します（エラーは無視）。
func (s *InterviewService) lookupCompanyReading(ctx context.Context, companyName string) string {
	systemPrompt := "あなたは日本企業の名称に詳しいアシスタントです。確実に知っている場合のみ答え、不明なら空文字を返してください。"
	query := fmt.Sprintf("「%s」の正しい日本語読み（ふりがな）をカタカナで1行だけ答えてください。", companyName)
	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := s.openaiClient.ResponsesWithTemperature(ctxTimeout, systemPrompt, query, 0.0)
	if err != nil {
		return ""
	}
	// 最初の行だけ抽出し、余分な記号や空白を除去
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) == 0 {
		return ""
	}
	reading := strings.TrimSpace(lines[0])
	// 句読点・括弧・引用符等を除去
	reading = strings.Trim(reading, "「」『』（）()。、・ ")
	return reading
}

// interviewTopics は面接で扱うトピックの順序定義です
var interviewTopics = []string{
	"自己紹介・志望動機",
	"職務経験・実績",
	"強み・弱み",
	"キャリアビジョン",
	"逆質問",
}

// maxTurnsPerTopic は1トピックあたりの最大ターン数（深掘り含む）
const maxTurnsPerTopic = 3

func buildInterviewSystemPrompt(
	companyName,
	companyReading,
	position,
	companyInfo,
	companyType string,
	customQuestions []models.InterviewCompanyQuestion,
	skillScores []models.SkillScore,
	turnCount int,
	remainingSeconds int,
	questionIndex int,
	totalQuestions int,
	questionElapsedSeconds int,
	questionDurationSeconds int,
	directive *questionDirective,
) string {
	if questionDurationSeconds <= 0 {
		questionDurationSeconds = 180
	}
	if totalQuestions <= 0 {
		totalQuestions = 1
	}
	if questionIndex <= 0 {
		questionIndex = 1
	}
	if questionIndex > totalQuestions {
		questionIndex = totalQuestions
	}
	if questionElapsedSeconds < 0 {
		questionElapsedSeconds = 0
	}
	if questionElapsedSeconds > questionDurationSeconds {
		questionElapsedSeconds = questionDurationSeconds
	}

	// ターン数からトピックインデックスと当該トピック内のターン数を計算
	topicIndex := 0
	turnsOnTopic := 0
	if turnCount > 0 {
		topicIndex = (turnCount - 1) / maxTurnsPerTopic
		if topicIndex >= len(interviewTopics) {
			topicIndex = len(interviewTopics) - 1
		}
		turnsOnTopic = (turnCount-1)%maxTurnsPerTopic + 1
	}
	currentTopic := interviewTopics[topicIndex]
	remainingTopics := len(interviewTopics) - topicIndex - 1

	base := `あなたは日本語の就活面接官です。以下を守ってください。

【基本ルール】
- 1回の返答は2〜3文以内で短くまとめる
- 必ず1つの質問で締めくくる
- 評価・講評は面接終了まで行わない
- 企業名を発話する際は、英字・略語はカタカナの正しい読み方で読んでください`

	// 現在のトピックと進行状況を明示
	base += fmt.Sprintf("\n\n【現在の面接状況】\n現在のトピック: %s（%d/%d）\n現在のトピックでの質問回数: %d/%d回",
		currentTopic, topicIndex+1, len(interviewTopics), turnsOnTopic, maxTurnsPerTopic)
	base += fmt.Sprintf(
		"\n\n【質問タイムボックス】\n1質問あたりの目安: 約%d分（%d秒）\n現在の質問番号: %d/%d\nこの質問の経過時間: %d秒\nこの質問の残り目安: %d秒",
		questionDurationSeconds/60,
		questionDurationSeconds,
		questionIndex,
		totalQuestions,
		questionElapsedSeconds,
		questionDurationSeconds-questionElapsedSeconds,
	)

	if remainingSeconds > 0 {
		remainingMinutes := remainingSeconds / 60
		base += fmt.Sprintf("\n残り時間: 約%d分", remainingMinutes)
		if remainingMinutes <= 2 && remainingTopics > 0 {
			base += fmt.Sprintf("（残り%dトピックあります。ペースを上げてください）", remainingTopics)
		}
	}

	base += `

【質問の進め方】
面接は以下の順番でトピックを進めてください:
1. 自己紹介・志望動機
2. 職務経験・実績
3. 強み・弱み
4. キャリアビジョン
5. 逆質問

【深掘りの判断基準】（重要）
- 応募者の回答が抽象的・表面的な場合のみ深掘りしてください
- 採用判断に有益な情報（具体的エピソード・数値・自分の行動・結果）が得られた場合は次のトピックへ移行してください
- 同一トピックへの深掘りは最大2回までとし、現在のトピックでの質問回数が3回に達したら必ず次のトピックへ移行してください
- 1つの質問は約3分を目安にし、目安時間に達したら自然に次の質問へ移行してください
- トピックを移行する際は「ありがとうございます。次に〜についてお聞きします。」と自然につないでください`

	if companyName != "" || position != "" {
		base += "\n\n【面接情報】"
		if companyName != "" {
			companyLabel := companyName
			if companyReading != "" {
				companyLabel += "（読み: " + companyReading + "）"
			}
			base += "\n志望企業: " + companyLabel
		}
		if position != "" {
			base += "\n応募職種: " + position
		}
		if companyInfo != "" {
			base += "\n\n【企業情報】\n" + companyInfo
		}
		base += "\n\n上記の企業・職種に合わせた質問を行ってください。企業文化・働き方・福利厚生の情報がある場合は、それらを踏まえた「この企業ならでは」の深掘り質問を取り入れてください。"
	}

	if companyType == "sier" {
		base += `

【SIer企業向け質問ガイドライン】
- 常駐先での顧客折衝・要件ヒアリング経験を深掘りする
- 上流工程（要件定義・基本設計）への関与実績を確認する
- ウォーターフォール・アジャイルどちらの経験があるか確認する
- IPA資格や技術資格の取得状況・今後の学習意欲を聞く
- 多様な現場・技術スタックへの適応力を問う
- 技術フェーズ（詳細設計・実装・テスト）と要件定義フェーズの両方を経験しているか確認する`
	}

	// 企業別カスタム質問をシステムプロンプトに注入
	if directive != nil && strings.TrimSpace(directive.Text) != "" {
		base += "\n\n【今回の質問】"
		if directive.IsDeepening {
			base += "\n前の回答を踏まえた深掘り質問です。"
		}
		base += fmt.Sprintf("\n次の質問文をそのまま面接官として投げかけてください（1問のみ）:\n%s", directive.Text)
		if directive.Category != "" {
			base += fmt.Sprintf("\n（カテゴリ: %s）", directive.Category)
		}
	} else if len(customQuestions) > 0 {
		var required, recommended []string
		for _, q := range customQuestions {
			if q.IsRequired {
				required = append(required, fmt.Sprintf("- [%s] %s", q.Category, q.QuestionText))
			} else {
				recommended = append(recommended, fmt.Sprintf("- [%s] %s", q.Category, q.QuestionText))
			}
		}
		if len(required) > 0 {
			base += "\n\n【必須質問（必ず全て質問してください）】\n" + strings.Join(required, "\n")
		}
		if len(recommended) > 0 {
			base += "\n\n【推奨質問（会話の流れに応じて取り入れてください）】\n" + strings.Join(recommended, "\n")
		}
	}

	// GitHubスキルスコアに基づく技術質問ガイドラインを注入（エンジニア職種のみ）
	if len(skillScores) > 0 && isEngineerPosition(position) {
		techHints := buildTechQuestionHints(skillScores)
		if techHints != "" {
			base += "\n\n【GitHubスキル分析に基づく技術質問ガイドライン】\n" + techHints
		}
	}

	return strings.TrimSpace(base)
}

// isEngineerPosition は職種名がエンジニア系かどうかを判定する
func isEngineerPosition(position string) bool {
	engineerKeywords := []string{
		"エンジニア", "engineer", "Engineer",
		"開発", "developer", "Developer",
		"プログラマ", "programmer", "Programmer",
		"バックエンド", "フロントエンド", "インフラ", "SRE", "DevOps",
		"アーキテクト", "Architect", "architect",
	}
	for _, kw := range engineerKeywords {
		if strings.Contains(position, kw) {
			return true
		}
	}
	return false
}

// buildTechQuestionHints はスキルスコアをもとに技術質問ガイドラインを生成する
func buildTechQuestionHints(scores []models.SkillScore) string {
	type scored struct {
		category models.SkillCategory
		score    float64
	}
	var top []scored
	for _, s := range scores {
		if s.Score >= 30 {
			top = append(top, scored{s.Category, s.Score})
		}
	}
	if len(top) == 0 {
		return ""
	}

	// スコア降順でソート
	for i := 0; i < len(top)-1; i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].score > top[i].score {
				top[i], top[j] = top[j], top[i]
			}
		}
	}

	hints := []string{"（応募者のGitHubスキルスコアを参考に技術的な深掘りを行ってください）"}
	for _, s := range top {
		switch s.category {
		case models.SkillCategoryBackend:
			hints = append(hints, fmt.Sprintf("- バックエンド（スコア%.0f）: 言語・フレームワーク・設計パターン・パフォーマンスチューニングについて質問する", s.score))
		case models.SkillCategoryFrontend:
			hints = append(hints, fmt.Sprintf("- フロントエンド（スコア%.0f）: 使用フレームワーク・状態管理・UI最適化・アクセシビリティについて質問する", s.score))
		case models.SkillCategoryInfra:
			hints = append(hints, fmt.Sprintf("- インフラ（スコア%.0f）: クラウドサービス・IaC・CI/CD・監視設計について質問する", s.score))
		case models.SkillCategoryDB:
			hints = append(hints, fmt.Sprintf("- データベース（スコア%.0f）: スキーマ設計・クエリ最適化・トランザクション管理について質問する", s.score))
		}
	}
	return strings.Join(hints, "\n")
}
