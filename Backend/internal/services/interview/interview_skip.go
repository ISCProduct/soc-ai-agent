package interview

import (
	"log"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// LightweightAnswerScore は、深掘りを飛ばすかどうかだけを決める点数（0〜100）。
// 面接の採点（ルーブリック、user_weight_scores、シャドー採点）には使わない。
// モデルは呼ばない。追加の API 費用は 0 で、遅延は文字列の走査だけ。
func LightweightAnswerScore(answer string) int {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" || trimmed == "（聞き取れませんでした）" {
		return 0
	}
	score := utf8.RuneCountInString(trimmed) * 60 / 80
	if score > 60 {
		score = 60
	}
	if strongSignalPattern.MatchString(trimmed) {
		score += 25
	}
	if numberedCountPattern.MatchString(trimmed) {
		score += 15
	}
	if len(weakSignalPattern.FindAllString(trimmed, -1)) >= 2 {
		score += 15
	}
	if score > 100 {
		return 100
	}
	return score
}

// shouldDeepen は追質問が必要かを返す。
// 閾値が無いときは NeedsDeepening と同じ。閾値以上の軽量点数なら深掘りしない。
func shouldDeepen(answer string, depth int, threshold *int) bool {
	if !NeedsDeepening(answer, depth) {
		return false
	}
	if threshold != nil && LightweightAnswerScore(answer) >= *threshold {
		return false
	}
	return true
}

// skipScoreThresholdFromEnv は INTERVIEW_SKIP_SCORE_THRESHOLD を読む。
// 未設定か 0〜100 以外は無効（深掘り判定は長さと具体性のまま）。
func skipScoreThresholdFromEnv() *int {
	raw := strings.TrimSpace(os.Getenv("INTERVIEW_SKIP_SCORE_THRESHOLD"))
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 100 {
		log.Printf("[Interview] INTERVIEW_SKIP_SCORE_THRESHOLD が 0〜100 の整数ではありません: %q", raw)
		return nil
	}
	return &n
}
