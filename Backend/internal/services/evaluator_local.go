package services

import (
	"Backend/domain/repository"
	"fmt"
	"strings"
)

// EvaluationResult はローカル評価の差分を表す（カテゴリ -> delta）
type EvaluationResult map[string]int

// EvaluateSpeechHeuristic は簡易ルールベースで発話からカテゴリ差分を算出する。
// 重点: LLM 呼び出しを行わずに速やかにスコア差分を得るための暫定実装。
func EvaluateSpeechHeuristic(speech string) EvaluationResult {
	low := strings.ToLower(speech)
	res := EvaluationResult{}

	// キーワードマップ（小さなステップで拡張可能）
	keywords := map[string][]string{
		"logic":       {"だから", "そのため", "つまり", "because", "therefore"},
		"specificity": {"例えば", "具体的", "経験", "実績", "回"},
		"ownership":   {"私が", "自分で", "担当した", "やった", "i did"},
		"communication": {"わかりやす", "明確", "整理", "簡潔"},
		"enthusiasm":  {"やる気", "熱意", "志望", "興味"},
	}

	for cat, kws := range keywords {
		delta := 0
		for _, kw := range kws {
			if strings.Contains(low, strings.ToLower(kw)) {
				delta += 10
			}
		}
		if delta != 0 {
			res[cat] = delta
		}
	}
	return res
}

// EvaluateAndPersist 発話を評価して user_weight_scores リポジトリへ反映する。
// 無ければ SetScore、既存なら AddScore を使って差分を適用する。
func (s *InterviewService) EvaluateAndPersist(userID uint, sessionID string, speech string) error {
	if s.userWeightScoreRepo == nil {
		// リポジトリ未注入の場合は noop
		return nil
	}
	deltas := EvaluateSpeechHeuristic(speech)
	for cat, delta := range deltas {
		if delta == 0 {
			continue
		}
		// check existing
		_, err := s.userWeightScoreRepo.FindByUserSessionAndCategory(userID, sessionID, cat)
		if err != nil {
			// not found -> create with baseline 50 +/- delta
			abs := clamp(50+delta, 0, 100)
			if err2 := s.userWeightScoreRepo.SetScore(userID, sessionID, cat, abs); err2 != nil {
				return fmt.Errorf("set score failed: %w", err2)
			}
		} else {
			if err2 := s.userWeightScoreRepo.AddScore(userID, sessionID, cat, delta); err2 != nil {
				return fmt.Errorf("add score failed: %w", err2)
			}
		}
	}
	return nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
