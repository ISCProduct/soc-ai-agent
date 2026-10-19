package aibench

import (
	"testing"

	"Backend/internal/services/resume"
)

// resumePromptOverheadTokens が実際のプロンプト定型文と一致していること（#1644）。
// 本文を空にして測ると WrapUntrustedText が空を返し、OCR 本文側の宣言文が乗らない。
// 本番は本文が必ずあるので、1文字の本文で測ってその1文字ぶんを引く。
func TestResumePromptOverheadTokens_MatchesPrompt(t *testing.T) {
	const body = "あ"
	prompt := resume.BuildReviewPromptFromText(body, "", "", "", "")
	got := ApproxTokensJA(prompt) - ApproxTokensJA(body)
	// 概算なので完全一致は求めないが、#1644 の差（136トークン=5%）は検出できる幅にする。
	const tolerance = 30
	if diff := got - resumePromptOverheadTokens; diff > tolerance || diff < -tolerance {
		t.Fatalf("定型文の実測 %d トークンに対し resumePromptOverheadTokens=%d。測り直して更新すること", got, resumePromptOverheadTokens)
	}
}
