package interview

import (
	"context"
	"testing"
	"time"

	"Backend/internal/openai"
	"Backend/internal/usagectx"
)

// TestRecordSTTUsage は音声認識の利用量が機能名つきで記録されることを検証する（#1294）。
// Turn 経路は全ユーザーが通るのに一切記録されていなかった（#1193）。その退行を防ぐ。
func TestRecordSTTUsage(t *testing.T) {
	tests := []struct {
		name         string
		obs          STTObservation
		wantRecords  int
		wantFeature  string
		wantSeconds  float64
		wantLatency  int
		wantFallback bool
	}{
		{
			name:        "通常のSTT",
			obs:         STTObservation{Model: "gpt-4o-mini-transcribe", AudioSeconds: 12.5, LatencyMS: 830},
			wantRecords: 1,
			wantFeature: usagectx.FeatureInterviewSTT,
			wantSeconds: 12.5,
			wantLatency: 830,
		},
		{
			name:         "再送ありは2件記録する(2回課金されるため)",
			obs:          STTObservation{Model: "gpt-4o-mini-transcribe", AudioSeconds: 8.0, LatencyMS: 500, FellBack: true},
			wantRecords:  2,
			wantFeature:  usagectx.FeatureInterviewSTT,
			wantSeconds:  8.0,
			wantLatency:  500,
			wantFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []openai.Usage
			cli := &openai.Client{OnUsage: func(u openai.Usage) { got = append(got, u) }}

			recordSTTUsage(context.Background(), cli, tt.obs)

			if len(got) != tt.wantRecords {
				t.Fatalf("記録件数=%d, 期待=%d", len(got), tt.wantRecords)
			}
			if got[0].Feature != tt.wantFeature {
				t.Errorf("Feature=%q, 期待=%q", got[0].Feature, tt.wantFeature)
			}
			if got[0].AudioSeconds != tt.wantSeconds {
				t.Errorf("AudioSeconds=%v, 期待=%v", got[0].AudioSeconds, tt.wantSeconds)
			}
			if got[0].LatencyMs != tt.wantLatency {
				t.Errorf("LatencyMs=%d, 期待=%d", got[0].LatencyMs, tt.wantLatency)
			}
			if tt.wantFallback && got[1].Model != FallbackModel {
				t.Errorf("再送分のモデル=%q, 期待=%q", got[1].Model, FallbackModel)
			}
			if tt.wantFallback && got[1].AudioSeconds != tt.wantSeconds {
				t.Errorf("再送分も同じ音声長で課金される: AudioSeconds=%v", got[1].AudioSeconds)
			}
		})
	}
}

// TestRecordTTSUsage は音声合成の利用量が文字数で記録されることを検証する（#1294）。
func TestRecordTTSUsage(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantCount int
		wantChars int
	}{
		{"日本語はrune数で数える", "こんにちは、面接を始めます。", 1, 14},
		{"英数字混在", "OK, let's start.", 1, 16},
		{"空文字は記録しない", "", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []openai.Usage
			cli := &openai.Client{OnUsage: func(u openai.Usage) { got = append(got, u) }}

			recordTTSUsage(context.Background(), cli, tt.text, 1200*time.Millisecond)

			if len(got) != tt.wantCount {
				t.Fatalf("記録件数=%d, 期待=%d", len(got), tt.wantCount)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].Feature != usagectx.FeatureInterviewTTS {
				t.Errorf("Feature=%q, 期待=%q", got[0].Feature, usagectx.FeatureInterviewTTS)
			}
			if got[0].Characters != tt.wantChars {
				t.Errorf("Characters=%d, 期待=%d", got[0].Characters, tt.wantChars)
			}
			if got[0].LatencyMs != 1200 {
				t.Errorf("LatencyMs=%d, 期待=1200", got[0].LatencyMs)
			}
			if got[0].Model != openai.DefaultTTSModel {
				t.Errorf("Model=%q, 期待=%q", got[0].Model, openai.DefaultTTSModel)
			}
		})
	}
}

// TestRecordUsageNilClient は AI クライアント未設定でも落ちないことを検証する。
// 計測は本処理を止めない（DesignDoc §1）。
func TestRecordUsageNilClient(t *testing.T) {
	recordSTTUsage(context.Background(), nil, STTObservation{AudioSeconds: 1})
	recordTTSUsage(context.Background(), nil, "text", time.Second)
}
