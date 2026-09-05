package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/email"
	"Backend/internal/services/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

func (s *InterviewService) StartWorker() {
	// Redis キュー利用時は asynq worker が処理する。フォールバック用 channel worker は常に起動。
	s.workerOnce.Do(func() {
		go s.runWorker()
	})
}

func (s *InterviewService) runWorker() {
	for sessionID := range s.jobCh {
		if err := s.generateReport(context.Background(), sessionID); err != nil {
			log.Printf("[Interview] Report generation failed for session %d: %v\n", sessionID, err)
			continue
		}
		log.Printf("[Interview] Report generation completed for session %d\n", sessionID)
	}
}

// buildTranscript formats utterances into a plain-text transcript for the LLM prompt.
// AI turns are labeled "Interviewer" and user turns are labeled "User".
func BuildTranscript(utterances []models.InterviewUtterance) string {
	var b strings.Builder
	for _, u := range utterances {
		role := "User"
		if u.Role == "ai" {
			role = "Interviewer"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(u.Text))
		b.WriteString("\n")
	}
	return b.String()
}

// extractJSONObject strips surrounding markdown code fences and extracts the
// outermost JSON object from an LLM response.
// Some models wrap their output in ```json ... ``` even when instructed not to.
func ExtractJSONObject(raw string) string {
	s := strings.TrimSpace(raw)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return s
}

func (s *InterviewService) generateReport(ctx context.Context, sessionID uint) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	lang := session.Language
	if lang == "" {
		lang = "ja"
	}

	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return err
	}
	if len(utterances) == 0 {
		// utterances が0件の場合は空レポートを保存して正常終了
		empty := &models.InterviewReport{
			SessionID:         sessionID,
			SummaryText:       "発話データがありませんでした。",
			ScoresJSON:        `{"logic":0,"specificity":0,"ownership":0,"communication":0,"enthusiasm":0}`,
			EvidenceJSON:      `{}`,
			StrengthsJSON:     `[]`,
			ImprovementsJSON:  `[]`,
			TeacherReportJSON: `{}`,
		}
		return s.reportRepo.Upsert(empty)
	}
	transcript := BuildTranscript(utterances)
	systemPrompt := buildReportSystemPrompt(lang)
	userPrompt := fmt.Sprintf(`以下の面接ログを読み、下記の評価基準に従ってJSONのみで出力してください。
出力言語: %s

## 評価基準（各スコアは0〜5の整数）
- logic（論理性）: 回答が筋道立っているか、主張に一貫性があるか
- specificity（具体性）: 具体的なエピソードや数値が含まれているか
- ownership（主体性）: 「私が〜した」という自分起点の表現があるか
- communication（コミュニケーション力）: 簡潔・明確に伝えられているか、聞き返しが少ないか
- enthusiasm（積極性・熱意）: 志望動機や意欲が伝わっているか。取り組みのきっかけや継続の姿勢も評価材料に含めてよい

## 出力フォーマット（このキーと型を厳守してください）
{
  "summary": "面接全体の総合評価コメント（2〜3文、生徒向けのやさしい言葉で）",
  "scores": {"logic": 3, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4},
  "evidence": {
    "logic": "論理性の根拠となった発言",
    "specificity": "具体性の根拠となった発言",
    "ownership": "主体性の根拠となった発言",
    "communication": "コミュニケーション力の根拠となった発言",
    "enthusiasm": "積極性・熱意の根拠となった発言（きっかけや継続の姿勢を含めてよい）"
  },
  "strengths": ["強み1", "強み2", "強み3"],
  "improvements": ["改善点1", "改善点2", "改善点3"],
  "teacher": {
    "overall_comment": "教員向け総評（指導観点・クラス内での位置づけ等）",
    "detailed_evidence": {"logic": "詳細な根拠と指導ポイント", "specificity": "詳細な根拠と指導ポイント", "ownership": "詳細な根拠と指導ポイント"},
    "coaching_points": ["具体的な改善指導ポイント1", "ポイント2", "ポイント3"],
    "strengths_for_teacher": ["指導者が把握すべき強み1", "強み2"],
    "next_steps": ["次回面接に向けた具体的な課題1", "課題2"]
  }
}

※ scoresは実際の会話内容に基づいて正直に採点してください（全て同じ値は避ける）。
※ strengths/improvementsは各2〜4件のリスト形式で具体的に記述してください。
※ teacher以下は教員専用の詳細情報として出力してください。

Interview transcript:
%s`, lang, transcript)

	model := shared.GetEnv("INTERVIEW_REPORT_MODEL", "")
	raw, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.4, 2000, model)
	if err != nil {
		return err
	}
	type teacherReport struct {
		OverallComment      string            `json:"overall_comment"`
		DetailedEvidence    map[string]string `json:"detailed_evidence"`
		CoachingPoints      []string          `json:"coaching_points"`
		StrengthsForTeacher []string          `json:"strengths_for_teacher"`
		NextSteps           []string          `json:"next_steps"`
	}
	type reportPayload struct {
		Summary      string            `json:"summary"`
		Scores       map[string]int    `json:"scores"`
		Evidence     map[string]string `json:"evidence"`
		Strengths    []string          `json:"strengths"`
		Improvements []string          `json:"improvements"`
		Teacher      *teacherReport    `json:"teacher"`
	}
	var payload reportPayload
	cleaned := ExtractJSONObject(raw)
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return fmt.Errorf("invalid report json: %w", err)
	}
	scoresJSON, _ := json.Marshal(payload.Scores)
	evidenceJSON, _ := json.Marshal(payload.Evidence)
	strengthsJSON, _ := json.Marshal(payload.Strengths)
	improvementsJSON, _ := json.Marshal(payload.Improvements)
	teacherJSON := []byte("{}")
	if payload.Teacher != nil {
		teacherJSON, _ = json.Marshal(payload.Teacher)
	}
	if s.questionStateRepo != nil {
		if states, stateErr := s.questionStateRepo.FindBySessionID(sessionID); stateErr == nil && len(states) > 0 {
			coverage := buildQuestionCoverage(states)
			if merged, mergeErr := mergeTeacherReportWithCoverage(string(teacherJSON), coverage); mergeErr == nil {
				teacherJSON = []byte(merged)
			}
		}
	}

	report := &models.InterviewReport{
		SessionID:         sessionID,
		SummaryText:       payload.Summary,
		ScoresJSON:        string(scoresJSON),
		EvidenceJSON:      string(evidenceJSON),
		StrengthsJSON:     string(strengthsJSON),
		ImprovementsJSON:  string(improvementsJSON),
		TeacherReportJSON: string(teacherJSON),
	}
	if err := s.reportRepo.Upsert(report); err != nil {
		return err
	}

	// 面接スコアを UserWeightScore に反映（crossFeature が設定済みの場合のみ）
	if s.crossFeature != nil {
		chatSessionID := fmt.Sprintf("interview-%d", session.UserID)
		if err := s.crossFeature.UpdateScoresFromInterviewReport(session.UserID, chatSessionID, report); err != nil {
			// スコア反映失敗はレポート生成を失敗扱いにしない
			log.Printf("[CrossFeature] interview score update failed for session %d: %v\n", sessionID, err)
		}
	}
	return nil
}

// SendReportEmail 面接レポートをメールで送信
func (s *InterviewService) SendReportEmail(userID, sessionID uint) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !s.isAllowed(userID, session.UserID) {
		return shared.ErrForbidden
	}

	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	if user.IsGuest {
		return errors.New("guest users cannot receive email reports")
	}

	report, err := s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		return errors.New("report not found")
	}

	var scores map[string]int
	json.Unmarshal([]byte(report.ScoresJSON), &scores)
	var evidence map[string]string
	json.Unmarshal([]byte(report.EvidenceJSON), &evidence)

	summary := strings.Split(strings.TrimSpace(report.SummaryText), "\n")
	var filtered []string
	for _, line := range summary {
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, strings.TrimSpace(line))
		}
	}

	data := email.InterviewReportEmailData{
		SessionID:  fmt.Sprintf("%d", sessionID),
		Summary:    filtered,
		LogicScore: scores["logic"],
		SpecScore:  scores["specificity"],
		OwnScore:   scores["ownership"],
		LogicEvid:  evidence["logic"],
		SpecEvid:   evidence["specificity"],
		OwnEvid:    evidence["ownership"],
	}
	return s.emailService.SendInterviewReport(user, data)
}

// buildReportSystemPrompt 言語コードに応じたレポート生成用システムプロンプトを返す。
func buildReportSystemPrompt(lang string) string {
	known := map[string]string{
		"ja": "あなたは就活面接のアシスタントです。面接ログを読み、要約・評価をJSONで返してください。",
		"en": "You are a job interview assessment assistant. Read the interview transcript and return evaluation as JSON.",
		"zh": "你是一位求职面试评估助手。请阅读面试记录并以JSON格式返回评估结果。",
		"ko": "당신은 취업 면접 평가 어시스턴트입니다. 면접 기록을 읽고 JSON 형식으로 평가를 반환하세요。",
		"fr": "Vous êtes un assistant d'évaluation d'entretien d'embauche. Lisez la transcription et retournez l'évaluation en JSON.",
		"es": "Eres un asistente de evaluación de entrevistas de trabajo. Lee la transcripción y devuelve la evaluación en JSON.",
		"de": "Sie sind ein Assistent zur Bewertung von Vorstellungsgesprächen. Lesen Sie das Transkript und geben Sie die Bewertung als JSON zurück.",
		"pt": "Você é um assistente de avaliação de entrevistas de emprego. Leia a transcrição e retorne a avaliação em JSON.",
		"it": "Sei un assistente per la valutazione dei colloqui di lavoro. Leggi la trascrizione e restituisci la valutazione in JSON.",
		"ar": "أنت مساعد تقييم مقابلات العمل. اقرأ النص وأعد التقييم بصيغة JSON.",
		"ru": "Вы ассистент по оценке собеседований. Прочитайте транскрипт и верните оценку в формате JSON.",
		"hi": "आप नौकरी साक्षात्कार मूल्यांकन सहायक हैं। साक्षात्कार का विवरण पढ़ें और मूल्यांकन JSON में लौटाएं।",
		"th": "คุณเป็นผู้ช่วยประเมินการสัมภาษณ์งาน อ่านบทสนทนาแล้วส่งคืนการประเมินในรูปแบบ JSON",
		"vi": "Bạn là trợ lý đánh giá phỏng vấn tuyển dụng. Đọc bản ghi và trả về đánh giá dưới dạng JSON.",
		"id": "Anda adalah asisten evaluasi wawancara kerja. Baca transkrip dan kembalikan evaluasi dalam format JSON.",
		"tr": "Siz bir iş görüşmesi değerlendirme asistanısınız. Metni okuyun ve değerlendirmeyi JSON formatında döndürün.",
	}
	if prompt, ok := known[lang]; ok {
		return prompt
	}
	return fmt.Sprintf("You are a job interview assessment assistant. Read the interview transcript and return evaluation as JSON. Use language code \"%s\" for the summary and evidence fields.", lang)
}
