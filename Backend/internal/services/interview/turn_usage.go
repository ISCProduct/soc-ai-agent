package interview

import (
	"context"
	"os"
	"time"

	"Backend/internal/openai"
	"Backend/internal/usagectx"
)

// recordSTTUsage は音声認識の利用量を記録する（#1294）。
//
// STT のレスポンスにはトークン使用量が入らないため、既に stt_observability で
// 計測している音声秒数とレイテンシをそのまま使う。秒数はフロントの録音ビットレートから
// 逆算した推定値なので、正確な請求額ではなく「機能別にどれだけ使ったか」を見るための値。
//
// 高精度モデルへの再送が発生した場合は2回課金されている。obs.FellBack が立っていれば
// 再送分も別の1件として記録する（採用可否ではなく実行したかで課金される）。
func recordSTTUsage(ctx context.Context, cli *openai.Client, obs STTObservation) {
	if cli == nil {
		return
	}
	ctx = usagectx.WithFeature(ctx, usagectx.FeatureInterviewSTT)
	cli.ReportAudioUsage(ctx, openai.AudioUsage{
		Model:        obs.Model,
		AudioSeconds: obs.AudioSeconds,
		Latency:      time.Duration(obs.LatencyMS) * time.Millisecond,
	})
	if obs.FellBack {
		cli.ReportAudioUsage(ctx, openai.AudioUsage{
			Model:        FallbackModel,
			AudioSeconds: obs.AudioSeconds,
			// 再送のレイテンシは別計測していない。0 のまま入れると平均を押し下げるため、
			// レイテンシは本体ぶんだけを記録する。
		})
	}
}

// recordTTSUsage は音声合成の利用量を記録する（#1294）。
// TTS は文字数が課金単位。rune 数で数える（日本語をバイト数で数えると実態と桁が合わない）。
func recordTTSUsage(ctx context.Context, cli *openai.Client, text string, latency time.Duration) {
	if cli == nil || text == "" {
		return
	}
	ctx = usagectx.WithFeature(ctx, usagectx.FeatureInterviewTTS)
	cli.ReportAudioUsage(ctx, openai.AudioUsage{
		Model:      ttsModelName(),
		Characters: len([]rune(text)),
		Latency:    latency,
	})
}

// ttsModelName は実際に使われる音声合成モデル名を返す。
// openai.TTS と同じ解決順にする（ずれると費用の内訳がモデル名で割れなくなる）。
func ttsModelName() string {
	if m := os.Getenv("OPENAI_TTS_MODEL"); m != "" {
		return m
	}
	return openai.DefaultTTSModel
}
