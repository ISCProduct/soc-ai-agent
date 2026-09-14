package diagnosis

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
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

	flags := heuristicFlags(scores, matches)
	confidence := heuristicConfidence(scores, matches, flags)
	summary := heuristicSummary(flags, confidence)

	raw := map[string]any{
		"source":      "heuristic",
		"score_count": len(scores),
		"match_top_n": len(matches),
		"message_n":   len(messages),
		"flags":       flags,
		"confidence":  confidence,
	}

	if s.aiClient != nil && len(messages) > 0 {
		if llm, llmErr := s.evaluateWithLLM(ctx, scores, messages, matches); llmErr != nil {
			log.Printf("[diagnosis.quality] LLM evaluate failed user=%d session=%s err=%v", userID, sessionID, llmErr)
			raw["llm_error"] = llmErr.Error()
		} else if llm != nil {
			flags = mergeFlags(flags, llm.Flags)
			if llm.Confidence > 0 {
				confidence = clampInt(llm.Confidence, 0, 100)
			}
			if strings.TrimSpace(llm.Summary) != "" {
				summary = strings.TrimSpace(llm.Summary)
			}
			raw["source"] = "heuristic+llm"
			raw["llm"] = llm
		}
	}

	flagsJSON, _ := json.Marshal(flags)
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

func heuristicFlags(scores []entity.UserWeightScore, matches []*entity.UserCompanyMatch) []string {
	var flags []string
	measured := 0
	for _, s := range scores {
		if s.Score != 0 {
			measured++
		}
	}
	if measured == 0 {
		flags = append(flags, "no_measured_axes")
	} else if measured < 4 {
		flags = append(flags, "few_measured_axes")
	}

	if len(matches) >= 2 {
		maxS := matches[0].MatchScore
		minS := matches[0].MatchScore
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
		if matches[0].MatchedAxisCount > 0 && matches[0].MatchedAxisCount < 4 {
			flags = append(flags, "thin_match_evidence")
		}
	} else if len(matches) == 0 {
		flags = append(flags, "no_matches")
	}
	return flags
}

func heuristicConfidence(scores []entity.UserWeightScore, matches []*entity.UserCompanyMatch, flags []string) int {
	measured := 0
	for _, s := range scores {
		if s.Score != 0 {
			measured++
		}
	}
	base := measured * 10 // 0-100 for 10 axes
	if base > 100 {
		base = 100
	}
	penalty := 12 * len(flags)
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
JSONのみで返答:
{"confidence":0-100の整数,"flags":["thin_evidence"|"score_transcript_mismatch"|"few_measured_axes"|"saturated_matches"等],"summary":"日本語で2文以内"}`
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
