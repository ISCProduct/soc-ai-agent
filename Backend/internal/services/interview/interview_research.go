package interview

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultResearchTTL = 168 * time.Hour

// confirmedResearchQuestions は、面接で使ってよい調査済みの質問を返す。
// 保存先が期限内の結果を持っていればそれを使い、渡された候補では上書きしない。
// 期限切れか未保存のときだけ、今回渡された候補を保存する。ここから Web 検索はしない。
func (s *InterviewService) confirmedResearchQuestions(ctx context.Context, companyName, position string, hints []string) ([]string, error) {
	if s.researchRepo == nil {
		return hints, nil
	}
	key := researchCompanyKey(companyName)
	if key == "" {
		return hints, nil
	}
	pos := strings.TrimSpace(position)
	now := time.Now()
	stored, ok, err := s.researchRepo.FindFresh(ctx, key, pos, now)
	if err != nil {
		return nil, fmt.Errorf("調査結果を読めません: %w", err)
	}
	if ok {
		return stored, nil
	}
	normalized := normalizeCompanyQuestionHints(hints)
	if len(normalized) == 0 {
		return nil, nil
	}
	expires := now.Add(researchTTL())
	if err := s.researchRepo.Save(ctx, key, pos, normalized, now, expires); err != nil {
		return nil, fmt.Errorf("調査結果を保存できません: %w", err)
	}
	return normalized, nil
}

func researchCompanyKey(companyName string) string {
	return strings.ToLower(strings.TrimSpace(companyName))
}

func researchTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("INTERVIEW_RESEARCH_TTL_HOURS"))
	if raw == "" {
		return defaultResearchTTL
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		log.Printf("[Interview] INTERVIEW_RESEARCH_TTL_HOURS が正の整数ではありません: %q", raw)
		return defaultResearchTTL
	}
	return time.Duration(hours) * time.Hour
}
