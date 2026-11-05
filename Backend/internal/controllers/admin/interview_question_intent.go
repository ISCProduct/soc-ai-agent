package admin

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// maxEvaluationIntentRunes は「評価したい観点」の上限（#1092）。
// AI 面接官の system プロンプトへ質問ごとに入るため、上限が無いとプロンプトが肥大化する。
const maxEvaluationIntentRunes = 300

var errEvaluationIntentTooLong = errors.New("evaluation_intent must be at most 300 characters")

// normalizeEvaluationIntent は前後の空白を落とし、空なら nil（NULL = 未設定）にする。
func normalizeEvaluationIntent(s string) (*string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(s) > maxEvaluationIntentRunes {
		return nil, errEvaluationIntentTooLong
	}
	return &s, nil
}

// clipEvaluationIntent は AI が生成した観点を上限に収める。
// 生成した質問は管理者が選んでから保存するので、拒否せず切り詰める。
func clipEvaluationIntent(s string) string {
	s = strings.TrimSpace(s)
	if rs := []rune(s); len(rs) > maxEvaluationIntentRunes {
		return string(rs[:maxEvaluationIntentRunes])
	}
	return s
}
