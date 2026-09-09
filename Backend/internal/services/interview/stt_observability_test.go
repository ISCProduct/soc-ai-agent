package interview

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// 学生の発話本文・認識本文がログへ出ないこと。
// アプリログは保存期限も削除手順も無い場所なので、
// ここへ個人情報が流れると後から回収できない。
func TestLogSTTObservation_DoesNotLeakTranscript(t *testing.T) {
	const secret = "アルバイト先の業務改善で作業時間を年間120時間削減しました"

	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })

	obs := ObserveTranscribe(42, 3, 48000, "audio/webm", time.Now().Add(-time.Second), secret, nil)
	LogSTTObservation(obs)

	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("認識本文がログに出ている: %s", out)
	}
	for _, frag := range []string{"アルバイト", "業務改善", "120時間"} {
		if strings.Contains(out, frag) {
			t.Errorf("認識本文の一部 %q がログに出ている: %s", frag, out)
		}
	}
	for _, want := range []string{"session_id", "turn", "model", "audio_bytes", "latency_ms", "result_chars"} {
		if !strings.Contains(out, want) {
			t.Errorf("計測項目 %q がログに無い", want)
		}
	}
}

// 文字数は rune で数える。バイト数で数えると日本語で
// 「認識できているのに文字数が少ない」判定を誤る。
func TestObserveTranscribe_CountsRunes(t *testing.T) {
	obs := ObserveTranscribe(1, 1, 1000, "audio/webm", time.Now(), "あいうえお", nil)
	if obs.ResultChars != 5 {
		t.Errorf("ResultChars = %d, want 5（バイト数だと15になる）", obs.ResultChars)
	}
}

func TestObserveTranscribe_TrimsWhitespace(t *testing.T) {
	obs := ObserveTranscribe(1, 1, 1000, "audio/webm", time.Now(), "  \n ", nil)
	if obs.ResultChars != 0 {
		t.Errorf("ResultChars = %d, want 0（空白のみは認識できていない）", obs.ResultChars)
	}
}

// 失敗を成功として記録すると、認識不能率が実態より低く見える。
func TestObserveTranscribe_RecordsFailure(t *testing.T) {
	ok := ObserveTranscribe(1, 1, 1000, "audio/webm", time.Now(), "はい", nil)
	if !ok.Succeeded {
		t.Error("成功時に Succeeded = false")
	}
	ng := ObserveTranscribe(1, 1, 1000, "audio/webm", time.Now(), "", errors.New("timeout"))
	if ng.Succeeded {
		t.Error("失敗時に Succeeded = true")
	}
}

// ログ上のモデル名が実際に使われるモデルとずれると、
// 費用と品質の突き合わせができなくなる。
func TestSTTModelName(t *testing.T) {
	t.Setenv("OPENAI_WHISPER_MODEL", "")
	if got := STTModelName(); got != defaultWhisperModel {
		t.Errorf("既定 = %q, want %q", got, defaultWhisperModel)
	}
	t.Setenv("OPENAI_WHISPER_MODEL", "gpt-4o-transcribe")
	if got := STTModelName(); got != "gpt-4o-transcribe" {
		t.Errorf("環境変数の上書きが効いていない: %q", got)
	}
}

func TestEstimateAudioSeconds(t *testing.T) {
	tests := []struct {
		name  string
		bytes int
		want  float64
	}{
		{"0バイト", 0, 0},
		{"負数", -1, 0},
		{"4KBで約1秒", 4000, 1},
		{"40KBで約10秒", 40000, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateAudioSeconds(tt.bytes); got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}

// 応答時間が記録されること。0のままだと遅延の傾向が追えない。
func TestObserveTranscribe_RecordsLatency(t *testing.T) {
	obs := ObserveTranscribe(1, 1, 1000, "audio/webm", time.Now().Add(-1500*time.Millisecond), "はい", nil)
	if obs.LatencyMS < 1400 || obs.LatencyMS > 1700 {
		t.Errorf("LatencyMS = %d, want 約1500", obs.LatencyMS)
	}
}
