package interview

import (
	"errors"
	"testing"
)

// 通常の発話が再送されないこと。再送率がそのまま追加費用になるため、
// ここが緩むと費用が倍になる。
func TestNeedsSTTFallback_NormalSpeechNotRetried(t *testing.T) {
	tests := []struct {
		name string
		text string
		sec  float64
	}{
		{"通常の回答", "三人チームでウェブアプリを作り、バックエンドを担当しました。", 12},
		{"短い相槌", "はい。", 1.5},
		{"短い肯定", "そうですね。", 2},
		{"数字を含む回答", "作業時間を年間120時間削減しました。", 8},
		{"英語を含む回答", "GoとAWSを使って開発しました。", 7},
		{"言い淀み", "えーと、私が主体となって進めました。", 8},
		{"長さ不明", "はい。", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsSTTFallback(tt.text, false, tt.sec); got != FallbackNone {
				t.Errorf("再送すると判定された（reason=%s）。通常発話は再送しない", got)
			}
		})
	}
}

// 問題が疑われる結果だけ再送すること。
func TestNeedsSTTFallback_RetriesProblems(t *testing.T) {
	tests := []struct {
		name string
		text string
		err  bool
		sec  float64
		want FallbackReason
	}{
		{"STTエラー", "", true, 10, FallbackError},
		{"空文字", "", false, 10, FallbackEmpty},
		{"空白のみ", "   ", false, 10, FallbackEmpty},
		{"聞き取れなかった", "（聞き取れませんでした）", false, 10, FallbackUnintelligible},
		{"10秒で3文字", "はい。", false, 10, FallbackTooShort},
		{"御社を本社と誤認", "本社のクラウドサービスに魅力を感じています。", false, 10, FallbackRoleConfusion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsSTTFallback(tt.text, tt.err, tt.sec); got != tt.want {
				t.Errorf("= %q, want %q", got, tt.want)
			}
		})
	}
}

// 短い音声にまで長さの下限を当てると、正常な相槌を再送し続ける。
func TestNeedsSTTFallback_ShortAudioExemptFromLengthCheck(t *testing.T) {
	// 2秒で1文字は比率としては下限割れだが、短い音声なので判定しない
	if got := NeedsSTTFallback("え", false, 2); got != FallbackNone {
		t.Errorf("短い音声で再送判定された: %s", got)
	}
	// 同じ文字数でも音声が長ければ再送する
	if got := NeedsSTTFallback("はい", false, 10); got != FallbackTooShort {
		t.Errorf("長い音声で短すぎる結果が再送されない: %s", got)
	}
}

// エラーは他のどの条件よりも優先する。
func TestNeedsSTTFallback_ErrorTakesPrecedence(t *testing.T) {
	if got := NeedsSTTFallback("正常に見えるテキストです", true, 10); got != FallbackError {
		t.Errorf("= %q, want %q", got, FallbackError)
	}
}

// 再送しても失敗・空なら元の結果を残す。
// 「再認識したら悪化した」を防ぐ。
func TestShouldUseFallbackResult(t *testing.T) {
	tests := []struct {
		name    string
		retried string
		err     error
		want    bool
	}{
		{"再送成功", "御社の", nil, true},
		{"再送がエラー", "御社の", errors.New("timeout"), false},
		{"再送が空", "", nil, false},
		{"再送が空白のみ", "  ", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldUseFallbackResult(tt.retried, tt.err); got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}

// 既定モデルが既に高精度なら再送しても結果は変わらず、費用だけ倍になる。
func TestFallbackIsRedundant(t *testing.T) {
	t.Setenv("OPENAI_WHISPER_MODEL", FallbackModel)
	if !FallbackIsRedundant() {
		t.Error("既定が高精度モデルなのに再送しようとしている")
	}
	t.Setenv("OPENAI_WHISPER_MODEL", "gpt-4o-mini-transcribe")
	if FallbackIsRedundant() {
		t.Error("既定がminiなのに再送しないと判定されている")
	}
}
