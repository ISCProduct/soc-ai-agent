package interview

import (
	"Backend/internal/openai"
	"log/slog"
	"os"
	"strings"
	"time"
)

// STTObservation は音声認識1回ぶんの計測値（音声R&D Task 1）。
//
// 学生の発話本文・認識本文は入れない。ここに入れた瞬間、
// アプリログという保存期限も削除手順も無い場所へ個人情報が流れる。
// 再現に必要なのは「どのセッションの何ターン目か」までで、中身ではない。
type STTObservation struct {
	SessionID    uint
	TurnCount    int
	Model        string
	AudioBytes   int
	AudioSeconds float64
	MimeType     string
	LatencyMS    int64
	Succeeded    bool
	ResultChars  int
	// FellBack は高精度モデルへ再送したか。採用したかではなく実行したか。
	// 再送率＝追加費用なので、採用しなかった再送も課金されている。
	FellBack bool
}

// STTModelName は実際に使われる音声認識モデル名を返す。
//
// openai.Transcribe と同じ解決順にする。ここがずれると
// 「ログ上はminiなのに実際は高精度モデル」という状態になり、
// 費用と品質の突き合わせができなくなる。
// 既定値は openai パッケージから直接引く。
// 以前はここに同じ文字列を書き写し「一致させること」とコメントで縛っていたが、
// 実際には片方だけ変わって食い違っていた。規約ではなく型で守る。
func STTModelName() string {
	if m := os.Getenv("OPENAI_WHISPER_MODEL"); m != "" {
		return m
	}
	return openai.DefaultTranscribeModel
}

// EstimateAudioSeconds は音声バイト数からおおよその秒数を出す。
//
// WebMは可変ビットレートのコンテナで、正確な長さはヘッダから簡単には取れない。
// ここでの用途は「音声長に対して認識文字数が極端に少ないか」の判定と
// 費用の概算なので、桁が合っていれば足りる。
//
// bytesPerSecond はブラウザの MediaRecorder が出す実測値から決めた概算。
// docs/research/interview-audio-eval/audio/*.webm は 8〜10秒で 30〜46KB
// だったため、約4KB/秒とする。
func EstimateAudioSeconds(audioBytes int) float64 {
	const bytesPerSecond = 4000.0
	if audioBytes <= 0 {
		return 0
	}
	return float64(audioBytes) / bytesPerSecond
}

// LogSTTObservation は計測値を構造化ログへ出す。
//
// 発話本文・認識本文・APIキーは出さない。出すのは
// 「再現と費用計算に必要な最小限」に限る。
func LogSTTObservation(o STTObservation) {
	slog.Info("stt observation",
		"session_id", o.SessionID,
		"turn", o.TurnCount,
		"model", o.Model,
		"audio_bytes", o.AudioBytes,
		"audio_seconds", round2(o.AudioSeconds),
		"mime_type", o.MimeType,
		"latency_ms", o.LatencyMS,
		"succeeded", o.Succeeded,
		"result_chars", o.ResultChars,
		"fell_back", o.FellBack,
	)
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// ObserveTranscribe は認識結果から計測値を組み立てる。
//
// 文字数は rune 数で数える。日本語をバイト数で数えると
// 「認識できているのに文字数が少ない」判定を誤る。
func ObserveTranscribe(
	sessionID uint, turnCount int, audioBytes int, mimeType string,
	start time.Time, text string, err error,
) STTObservation {
	return STTObservation{
		SessionID:    sessionID,
		TurnCount:    turnCount,
		Model:        STTModelName(),
		AudioBytes:   audioBytes,
		AudioSeconds: EstimateAudioSeconds(audioBytes),
		MimeType:     mimeType,
		LatencyMS:    time.Since(start).Milliseconds(),
		Succeeded:    err == nil,
		ResultChars:  len([]rune(strings.TrimSpace(text))),
	}
}
