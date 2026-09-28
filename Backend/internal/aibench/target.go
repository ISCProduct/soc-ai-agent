package aibench

import (
	"context"
	"fmt"
	"strings"
)

// Target は評価対象1つ。
//
// 呼び出し（ネットワーク）と評価（純粋関数）を Run の内側で分けてある。
// 評価側は *_eval.go の関数として切り出し、LLM を呼ばずにテストできる。
type Target interface {
	// Name は target 名（manifest の target と一致する）。
	Name() string
	// Model は測ったモデル名。結果に必ず残す。
	Model() string
	// Endpoint は呼び出し先の識別子（結果の再現条件として残す）。
	Endpoint() string
	// EstimateTokens は事前のコスト概算に使うトークン数の見積もり。
	EstimateTokens(c Case) (prompt, completion int)
	// Run は1ケース1回を実行して結果を返す。
	Run(ctx context.Context, c Case) Observation
}

// NewTarget は target 名から評価対象を作る。
// modelOverride が空なら本番と同じ既定モデルを使う。
func NewTarget(name, modelOverride string) (Target, error) {
	switch name {
	case TargetES:
		return newESTarget(modelOverride)
	case TargetResume:
		return newResumeTarget(modelOverride), nil
	case TargetInterviewReport:
		return newInterviewTarget(modelOverride), nil
	default:
		return nil, fmt.Errorf("未知の target: %q（対応: %s）", name, strings.Join(ValidTargets(), ", "))
	}
}

// extractJSONObject は前後の散文やコードフェンスを落として最外の JSON を取り出す。
//
// 本番も同じ復旧を行っている（resume の decodeJSON / interview の ExtractJSONObject）。
// ハーネスがこれを通さずに判定すると、本番では読めている出力まで
// 破損として数え、破損率が実態より高く出る。
// 「復旧が必要だったか」は指示違反（json_not_bare）として別に数える。
func extractJSONObject(raw string) string {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// needsJSONRecovery は出力が素の JSON ではなかったかを返す。
func needsJSONRecovery(raw string) bool {
	return strings.TrimSpace(raw) != extractJSONObject(raw)
}

// normalizeForQuote は引用の照合用に空白と改行を落とす。
// LLM は引用時に改行やスペースを入れ直すため、そのまま比較すると
// 本文に存在する引用まで捏造として数えてしまう。
func normalizeForQuote(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '　':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// runeLen は文字数（バイト数ではなく）を返す。文字数の指示遵守判定に使う。
func runeLen(s string) int { return len([]rune(s)) }
