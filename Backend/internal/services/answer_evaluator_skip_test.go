package services

// skipPhrases 統合テスト（Issue #317）
// 実行: cd Backend && go test ./internal/services/... -run TestPrecheckHuman_SkipPhrases -v

import (
	"testing"
)

// TestPrecheckHuman_SkipPhrases_UnifiedList は統合された skipPhrases が全フレーズを正しく捕捉することを検証する（#317修正の担保）
func TestPrecheckHuman_SkipPhrases_UnifiedList(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		want   PrecheckAction
	}{
		// 第1定義にのみあったフレーズ（旧第2定義では脱落していた）
		{"わかりません", "わかりません", PrecheckSkip},
		{"分かりません（全角）", "分かりません", PrecheckSkip},
		// 両定義に存在したフレーズ
		{"わからない", "わからない", PrecheckSkip},
		{"分からない", "分からない", PrecheckSkip},
		{"特にない", "特にない", PrecheckSkip},
		{"特になし", "特になし", PrecheckSkip},
		{"なし", "なし", PrecheckSkip},
		// 句読点付き（第2ループで捕捉）
		{"なし。", "なし。", PrecheckSkip},
		{"特にない、", "特にない、", PrecheckSkip},
		// 通常の回答はスキップされない
		{"普通の回答", "技術スキルがあります", PrecheckScore},
		{"資格なし（部分一致はスキップしない）", "資格なし取得予定", PrecheckScore},
	}

	ev := &AnswerEvaluator{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ev.precheckHuman(tc.answer, false, false)
			if result.Action != tc.want {
				t.Errorf("answer=%q: got Action=%v, want %v", tc.answer, result.Action, tc.want)
			}
		})
	}
}
