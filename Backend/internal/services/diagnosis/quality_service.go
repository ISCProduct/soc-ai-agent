package diagnosis

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/chat"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"
)

// QualityService は診断完了後に妥当性フラグを保存する（スコアは更新しない）。
type QualityService struct {
	reportRepo *repositories.DiagnosisQualityRepository
	scoreRepo  repository.UserWeightScoreRepository
	chatRepo   repository.ChatMessageRepository
	matchRepo  repository.UserCompanyMatchRepository
	aiClient   *openai.Client
}

func NewQualityService(
	reportRepo *repositories.DiagnosisQualityRepository,
	scoreRepo repository.UserWeightScoreRepository,
	chatRepo repository.ChatMessageRepository,
	matchRepo repository.UserCompanyMatchRepository,
	aiClient *openai.Client,
) *QualityService {
	return &QualityService{
		reportRepo: reportRepo,
		scoreRepo:  scoreRepo,
		chatRepo:   chatRepo,
		matchRepo:  matchRepo,
		aiClient:   aiClient,
	}
}

type qualityLLMResult struct {
	Confidence int      `json:"confidence"`
	Flags      []string `json:"flags"`
	Summary    string   `json:"summary"`
}

// RunDiagnosisQuality は会話・スコア・マッチからフラグを算出し保存する。
func (s *QualityService) RunDiagnosisQuality(ctx context.Context, userID uint, sessionID string) error {
	scores, err := s.scoreRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return fmt.Errorf("load scores: %w", err)
	}
	messages, err := s.chatRepo.FindRecentBySessionIDForUser(sessionID, userID, 40)
	if err != nil {
		log.Printf("[diagnosis.quality] history load failed user=%d session=%s err=%v", userID, sessionID, err)
		messages = nil
	}
	matches, err := s.matchRepo.FindTopMatchesByUserAndSession(userID, sessionID, 5)
	if err != nil {
		log.Printf("[diagnosis.quality] matches load failed user=%d session=%s err=%v", userID, sessionID, err)
		matches = nil
	}

	chatFlags, chatStats := chatEvidenceFlags(messages)
	flags := mergeFlags(heuristicFlags(scores, matches), chatFlags)
	confidence := heuristicConfidence(scores, flags)
	summary := heuristicSummary(flags, confidence)

	if s.aiClient != nil && len(messages) > 0 {
		if llm, llmErr := s.evaluateWithLLM(ctx, scores, messages, matches); llmErr != nil {
			log.Printf("[diagnosis.quality] LLM evaluate failed user=%d session=%s err=%v", userID, sessionID, llmErr)
		} else if llm != nil {
			flags = mergeFlags(flags, llm.Flags)
			// LLM が楽観的でも、構造的な根拠不足を高 confidence にできないようにする
			if llm.Confidence > 0 {
				confidence = minInt(confidence, clampInt(llm.Confidence, 0, 100))
			}
			if strings.TrimSpace(llm.Summary) != "" {
				summary = strings.TrimSpace(llm.Summary)
			}
		}
	}

	// raw は最終状態を監査できるよう、マージ後に組み立てる
	raw := map[string]any{
		"score_count": len(scores),
		"match_top_n": len(matches),
		"message_n":   len(messages),
		"chat_stats":  chatStats,
		"flags":       flags,
		"confidence":  confidence,
	}
	if len(flags) == 0 {
		raw["flags"] = []string{}
	}

	flagsJSON, _ := json.Marshal(flags)
	if flags == nil {
		flagsJSON = []byte("[]")
	}
	rawJSON, _ := json.Marshal(raw)
	report := &models.DiagnosisQualityReport{
		UserID:     userID,
		SessionID:  sessionID,
		Confidence: confidence,
		FlagsJSON:  string(flagsJSON),
		Summary:    summary,
		RawJSON:    string(rawJSON),
	}
	if err := s.reportRepo.Upsert(report); err != nil {
		return fmt.Errorf("upsert report: %w", err)
	}
	log.Printf("[diagnosis.quality] saved user=%d session=%s confidence=%d flags=%v", userID, sessionID, confidence, flags)
	return nil
}

func measuredAxisCount(scores []entity.UserWeightScore) int {
	// マッチングと同じく「行がある＝計測済み」（値0も含む）
	return len(scores)
}

func heuristicFlags(scores []entity.UserWeightScore, matches []*entity.UserCompanyMatch) []string {
	var flags []string
	measured := measuredAxisCount(scores)
	if measured == 0 {
		flags = append(flags, "no_measured_axes")
	} else if measured < 4 {
		flags = append(flags, "few_measured_axes")
	}

	switch {
	case len(matches) == 0:
		flags = append(flags, "no_matches")
	case len(matches) == 1:
		flags = append(flags, "single_match_only")
		if matches[0].MatchedAxisCount < 4 {
			flags = append(flags, "thin_match_evidence")
		}
	default:
		maxS := matches[0].MatchScore
		minS := matches[len(matches)-1].MatchScore
		for _, m := range matches {
			if m.MatchScore > maxS {
				maxS = m.MatchScore
			}
			if m.MatchScore < minS {
				minS = m.MatchScore
			}
		}
		if maxS >= 90 && (maxS-minS) < 8 {
			flags = append(flags, "saturated_matches")
		}
		// 上位マッチの「最小」根拠軸数で見る（docs/wiki/scoring.md）。先頭だけだと後続の薄い根拠を見落とす。
		minAxes := matches[0].MatchedAxisCount
		for _, m := range matches[1:] {
			if m.MatchedAxisCount < minAxes {
				minAxes = m.MatchedAxisCount
			}
		}
		if minAxes < 4 {
			flags = append(flags, "thin_match_evidence")
		}
	}
	return flags
}

func chatEvidenceFlags(messages []models.ChatMessage) (flags []string, stats map[string]int) {
	stats = map[string]int{
		"user_messages": 0,
		"choice_only":   0,
		"with_reason":   0,
		"contradiction": 0,
		"free_text":     0,
		"thin_free":     0,
	}
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		stats["user_messages"]++
		switch chat.ClassifyOutgoingAnswerEvidence(msg.Content) {
		case chat.EvidenceChoiceOnly:
			stats["choice_only"]++
		case chat.EvidenceChoiceWithReason:
			stats["with_reason"]++
		case chat.EvidenceChoiceContradiction:
			stats["contradiction"]++
		case chat.EvidenceFreeText:
			stats["free_text"]++
		case chat.EvidenceThinFreeText:
			stats["thin_free"]++
		}
	}

	if stats["user_messages"] == 0 {
		return []string{"no_chat_evidence"}, stats
	}

	substantial := stats["with_reason"] + stats["free_text"]
	weak := stats["choice_only"] + stats["thin_free"] + stats["contradiction"]

	if substantial == 0 {
		flags = append(flags, "thin_chat_evidence")
	}
	if weak > 0 && weak*2 >= stats["user_messages"] {
		flags = append(flags, "mostly_choice_only")
	}
	if stats["contradiction"] > 0 {
		flags = append(flags, "choice_reason_contradiction")
	}
	return flags, stats
}

func heuristicConfidence(scores []entity.UserWeightScore, flags []string) int {
	measured := measuredAxisCount(scores)
	base := measured * 10
	if base > 100 {
		base = 100
	}
	penalty := 0
	for _, f := range flags {
		switch f {
		case "no_chat_evidence", "thin_chat_evidence", "mostly_choice_only", "choice_reason_contradiction":
			penalty += 18
		case "thin_match_evidence", "few_measured_axes", "no_measured_axes":
			penalty += 14
		default:
			penalty += 10
		}
	}
	return clampInt(base-penalty, 5, 95)
}

func heuristicSummary(flags []string, confidence int) string {
	if len(flags) == 0 {
		return fmt.Sprintf("診断根拠は概ね揃っています（信頼度 %d）。", confidence)
	}
	return fmt.Sprintf("診断に注意フラグがあります: %s（信頼度 %d）。スコアの自動補正は行っていません。",
		strings.Join(flags, ", "), confidence)
}

func mergeFlags(base, extra []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range append(base, extra...) {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *QualityService) evaluateWithLLM(
	ctx context.Context,
	scores []entity.UserWeightScore,
	messages []models.ChatMessage,
	matches []*entity.UserCompanyMatch,
) (*qualityLLMResult, error) {
	var scoreLines []string
	for _, sc := range scores {
		scoreLines = append(scoreLines, fmt.Sprintf("- %s: %d", sc.WeightCategory, sc.Score))
	}
	var matchLines []string
	for _, m := range matches {
		matchLines = append(matchLines, fmt.Sprintf("- company_id=%d score=%.1f axes=%d", m.CompanyID, m.MatchScore, m.MatchedAxisCount))
	}
	var chatLines []string
	for _, msg := range messages {
		content := msg.Content
		if utf8.RuneCountInString(content) > 200 {
			content = string([]rune(content)[:200]) + "…"
		}
		chatLines = append(chatLines, fmt.Sprintf("%s: %s", msg.Role, content))
	}

	system := `あなたは適性診断の品質監査者です。ユーザースコアを変更せず、会話・スコア・マッチの整合だけを評価してください。
選択肢だけの回答が多い・理由が薄い・スコアと会話の方向が食い違う場合は confidence を下げ、対応する flags を付けてください。
JSONのみで返答:
{"confidence":0-100の整数,"flags":["thin_chat_evidence"|"mostly_choice_only"|"score_transcript_mismatch"|"few_measured_axes"|"saturated_matches"等],"summary":"日本語で2文以内"}`
	user := fmt.Sprintf("スコア:\n%s\n\n上位マッチ:\n%s\n\n会話抜粋:\n%s",
		strings.Join(scoreLines, "\n"),
		strings.Join(matchLines, "\n"),
		strings.Join(chatLines, "\n"),
	)

	raw, err := s.aiClient.ChatCompletionJSON(ctx, system, user, 0.2, 400)
	if err != nil {
		return nil, err
	}
	var parsed qualityLLMResult
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse llm json: %w", err)
	}
	return &parsed, nil
}
