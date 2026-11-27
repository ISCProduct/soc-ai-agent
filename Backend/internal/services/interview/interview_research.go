package interview

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultResearchTTL = 168 * time.Hour

// confirmedResearchQuestions は、面接で使う企業別の質問候補を返す。
//   - 保存先に期限内のサーバー側の調査結果があれば、それを使う（クライアントの候補では上書きしない）。
//   - なければ、リクエストで渡された候補をこのセッション限りで使う。クライアント由来の候補は
//     企業×職種で共有される保存先には書かない（他の利用者の面接に混ざるのを防ぐ）。
//   - 保存先を読めなくても質問キューの作成は止めない。ログに残し、渡された候補（なければ空）で続ける。
//
// ここから Web 検索はしない。
func (s *InterviewService) confirmedResearchQuestions(ctx context.Context, companyName, position string, hints []string) []string {
	sessionOnly := normalizeCompanyQuestionHints(hints)
	if s.researchRepo == nil {
		return sessionOnly
	}
	key := researchCompanyKey(companyName)
	if key == "" {
		return sessionOnly
	}
	stored, ok, err := s.researchRepo.FindFresh(ctx, key, strings.TrimSpace(position), time.Now())
	if err != nil {
		log.Printf("[Interview] 調査結果を読めないため、セッション限りの候補で続けます: %v", err)
		return sessionOnly
	}
	if ok {
		return stored
	}
	return sessionOnly
}

// SaveServerResearch は、サーバー側で調査した質問候補を共有の保存先に書く。
// リクエスト本文（クライアント）から来た候補を渡してはいけない。保存に失敗しても面接は止めず、エラーを返すだけ。
// 企業名・職種が列の幅を超えるときは、リポジトリが列の幅まで切り詰める。
func (s *InterviewService) SaveServerResearch(ctx context.Context, companyName, position string, questions []string) error {
	if s.researchRepo == nil {
		return nil
	}
	key := researchCompanyKey(companyName)
	normalized := normalizeCompanyQuestionHints(questions)
	if key == "" || len(normalized) == 0 {
		return nil
	}
	now := time.Now()
	return s.researchRepo.Save(ctx, key, strings.TrimSpace(position), normalized, now, now.Add(s.researchTTL))
}

func researchCompanyKey(companyName string) string {
	return strings.ToLower(strings.TrimSpace(companyName))
}

// researchTTLFromEnv は INTERVIEW_RESEARCH_TTL_HOURS を読む。サービス生成時に1回だけ呼ぶ。
func researchTTLFromEnv() time.Duration {
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
