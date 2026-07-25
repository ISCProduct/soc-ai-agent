package services

import (
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func (s *InterviewService) GetReport(userID uint, sessionID uint) (*models.InterviewReport, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	report, err := s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return report, nil
}

// GetPhraseSuggestions はセッションのユーザー発話を分析し、言い換え提案を返す。
func (s *InterviewService) GetPhraseSuggestions(ctx context.Context, userID uint, sessionID uint) ([]PhraseSuggestion, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	var userTexts []string
	for _, u := range utterances {
		if u.Role == "user" {
			t := strings.TrimSpace(u.Text)
			if t != "" {
				userTexts = append(userTexts, t)
			}
		}
	}
	if len(userTexts) == 0 {
		return []PhraseSuggestion{}, nil
	}
	transcript := strings.Join(userTexts, "\n")
	systemPrompt := "あなたは就活面接コーチです。応募者の発言から改善すべき曖昧・抽象的・弱い表現を抽出し、より具体的・主体的・印象的な言い換えを提案してください。"
	userPrompt := fmt.Sprintf(`以下は就活面接における応募者の発言です。
改善が有効な表現を最大5件抽出し、それぞれに2〜3件の言い換え候補を提示してください。

出力はJSONのみ（マークダウン不可）で以下の形式を厳守してください:
{"suggestions": [{"original": "元の表現", "suggestions": ["言い換え1", "言い換え2"]}, ...]}

応募者発言:
%s`, transcript)

	model := getEnv("INTERVIEW_REPORT_MODEL", "")
	raw, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.5, 1000, model)
	if err != nil {
		return nil, err
	}
	raw = ExtractJSONObject(raw)
	var payload struct {
		Suggestions []PhraseSuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse suggestions: %w", err)
	}
	return payload.Suggestions, nil
}

// GetTrend は指定ユーザーの完了済み面接セッションのスコア時系列を返す。
// sessions は古い順（昇順）で返却されるため、フロントエンドでそのままグラフに使える。
func (s *InterviewService) GetTrend(userID uint, limit int) ([]InterviewTrendPoint, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	// 新しい順で取得し、後で逆順にする（古い順でグラフ描画するため）
	sessions, err := s.sessionRepo.ListFinishedByUser(userID, limit)
	if err != nil {
		return nil, err
	}
	// 古い順に並べ替え
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}

	points := make([]InterviewTrendPoint, 0, len(sessions))
	for _, session := range sessions {
		report, err := s.reportRepo.FindBySessionID(session.ID)
		if err != nil {
			// レポート未生成のセッションはスキップ
			continue
		}
		var scores map[string]float64
		if err := json.Unmarshal([]byte(report.ScoresJSON), &scores); err != nil {
			continue
		}
		pt := InterviewTrendPoint{
			SessionID: session.ID,
			CreatedAt: session.CreatedAt,
		}
		if v, ok := scores["logic"]; ok {
			vv := v
			pt.Logic = &vv
		}
		if v, ok := scores["specificity"]; ok {
			vv := v
			pt.Specificity = &vv
		}
		if v, ok := scores["ownership"]; ok {
			vv := v
			pt.Ownership = &vv
		}
		if v, ok := scores["communication"]; ok {
			vv := v
			pt.Communication = &vv
		}
		if v, ok := scores["enthusiasm"]; ok {
			vv := v
			pt.Enthusiasm = &vv
		}
		points = append(points, pt)
	}
	return points, nil
}

func (s *InterviewService) CreateRealtimeToken(ctx context.Context, userID uint, sessionID uint) (string, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return "", err
	}
	if !s.isAllowed(userID, session.UserID) {
		return "", errors.New("forbidden")
	}
	if session.Status == "finished" {
		return "", errors.New("session already finished")
	}
	if s.realtimeUsageService != nil {
		allowed, active, maxAllowed, err := s.realtimeUsageService.CanOpenNewConnection()
		if err != nil {
			return "", err
		}
		if !allowed {
			return "", fmt.Errorf("realtime capacity exceeded: active=%d limit=%d", active, maxAllowed)
		}
	}
	lang := session.Language
	if lang == "" {
		lang = "ja"
	}
	gender := session.InterviewerGender
	if gender == "" {
		gender = "female"
	}
	model := getEnv("OPENAI_REALTIME_MODEL", "gpt-4o-realtime-preview")
	voice := realtimeVoiceForLangAndGender(lang, gender)
	transcribeModel := getEnv("OPENAI_REALTIME_TRANSCRIBE_MODEL", "")
	if transcribeModel == "" {
		transcribeModel = "gpt-4o-transcribe"
	}
	maxTokens := getIntEnv("OPENAI_REALTIME_MAX_OUTPUT_TOKENS", 120)
	req := openai.RealtimeSessionRequest{
		Model:        model,
		Modalities:   []string{"audio"},
		Voice:        voice,
		Instructions: buildRealtimeInstructions(lang),
		InputAudioTranscription: map[string]any{
			"model":    transcribeModel,
			"language": lang,
		},
		TurnDetection: map[string]any{
			"type":                "server_vad",
			"threshold":           0.35,
			"silence_duration_ms": 500,
			"prefix_padding_ms":   150,
			"create_response":     true,
		},
		MaxResponseOutputTokens: maxTokens,
	}
	resp, err := s.openaiClient.CreateRealtimeClientSecret(ctx, req)
	if err != nil {
		return "", err
	}
	if s.realtimeUsageService != nil {
		if err := s.realtimeUsageService.EnsureSessionStarted(userID, sessionID); err != nil {
			return "", err
		}
	}
	return resp.ClientSecret.Value, nil
}

// ttsVoiceForGenderAndLang 性別と言語に応じたTTSボイスを返す。
// 参照: https://note.com/affiwriting/n/nc2a665c234a7
// 男性: 英語系は echo / それ以外は onyx
// 女性: shimmer (クリアで表現力豊かな女性の声)
func ttsVoiceForGenderAndLang(gender, lang string) string {
	switch gender {
	case "male":
		switch strings.ToLower(lang) {
		case "en":
			return "echo"
		default:
			return "onyx"
		}
	default: // female
		return "shimmer"
	}
}

// TTSVoiceForGenderAndLang is an exported wrapper for ttsVoiceForGenderAndLang for external tests.
func TTSVoiceForGenderAndLang(gender, lang string) string {
	return ttsVoiceForGenderAndLang(gender, lang)
}

// RealtimeVoiceForLangAndGender is an exported wrapper for realtimeVoiceForLangAndGender for external tests.
func RealtimeVoiceForLangAndGender(lang, gender string) string {
	return realtimeVoiceForLangAndGender(lang, gender)
}

// realtimeVoiceForLangAndGender 言語・性別コードに応じた推奨ボイスを返す。
// 環境変数 OPENAI_REALTIME_VOICE が設定されている場合はそちらを優先する。
func realtimeVoiceForLangAndGender(lang, gender string) string {
	if v := getEnv("OPENAI_REALTIME_VOICE", ""); v != "" {
		return v
	}
	return ttsVoiceForGenderAndLang(gender, lang)
}

// buildRealtimeInstructions 面接官AIへのシステムプロンプトを返す。
// デフォルトは日本語で進行し、面接者から別言語を求められた場合は即座に切り替える。
func buildRealtimeInstructions(_ string) string {
	return strings.TrimSpace(`あなたはプロの就活面接官です。以下のルールに従ってください。

【言語対応】
- デフォルトは日本語で面接を行う
- 面接者から別の言語での面接を求められた場合（例：「英語でお願いします」「Please switch to English」「请用中文」など）は、即座にその言語に切り替えて面接を継続する
- 一度切り替えた言語は、面接者から変更を求められるまで維持する

【質問の意図が伝わらない場合】
- 面接者から「意味がわかりません」「質問の意図を教えてください」「もう少し詳しく教えてください」などの発言があった場合は、同じ質問を別の言い方で言い換えるか、具体的な例を添えて再度問いかける
- 言い換えても理解が難しそうな場合は「では少し視点を変えて〜」と切り出し、関連する別の質問に移る
- 面接者が質問に詰まっている場合は「焦らずに考えてみてください」と一言添えてから待つ

【面接の進め方】
- 1回の発話は短く、質問中心にする
- 詳細な講評・フィードバックは面接終了後に行う
- 面接者が話しやすいよう、具体的に深掘りする`)
}
