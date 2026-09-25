package openai

import (
	"context"
	"testing"
	"time"

	"Backend/internal/usagectx"
)

// TestReportUsageFillsAttribution は記録層が context から配賦軸を埋めることを検証する（#1294）。
// ここが埋まらないと api_call_logs は機能別・組織別に割れない。
func TestReportUsageFillsAttribution(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		report      usageReport
		wantFeature string
		wantUser    bool
		wantOrg     bool
		wantLatency int
		wantCache   bool
	}{
		{
			name:        "機能名と主体が載る",
			ctx:         usagectx.WithActor(usagectx.WithFeature(context.Background(), usagectx.FeatureESReview), 42, 7),
			report:      usageReport{provider: "openai", model: "gpt-4o", promptTokens: 10, completionTokens: 5, latency: 1500 * time.Millisecond},
			wantFeature: usagectx.FeatureESReview,
			wantUser:    true,
			wantOrg:     true,
			wantLatency: 1500,
		},
		{
			name:        "機能名が無ければunknown(計測漏れの指標)",
			ctx:         context.Background(),
			report:      usageReport{provider: "local", model: "llama"},
			wantFeature: usagectx.FeatureUnknown,
		},
		{
			name:        "バッチ経路は主体なしで記録する",
			ctx:         usagectx.WithFeature(context.Background(), usagectx.FeatureCompanyCrawl),
			report:      usageReport{provider: "openai", model: "gpt-4o-mini", cacheHit: true},
			wantFeature: usagectx.FeatureCompanyCrawl,
			wantCache:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Usage
			cli := &Client{OnUsage: func(u Usage) { got = u }}

			cli.reportUsage(tt.ctx, tt.report)

			if got.Feature != tt.wantFeature {
				t.Errorf("Feature=%q, 期待=%q", got.Feature, tt.wantFeature)
			}
			if (got.UserID != nil) != tt.wantUser {
				t.Errorf("UserID=%v, 主体の期待=%v", got.UserID, tt.wantUser)
			}
			if (got.OrganizationID != nil) != tt.wantOrg {
				t.Errorf("OrganizationID=%v, 配賦先の期待=%v", got.OrganizationID, tt.wantOrg)
			}
			if got.LatencyMs != tt.wantLatency {
				t.Errorf("LatencyMs=%d, 期待=%d", got.LatencyMs, tt.wantLatency)
			}
			if got.CacheHit != tt.wantCache {
				t.Errorf("CacheHit=%v, 期待=%v", got.CacheHit, tt.wantCache)
			}
			if got.Model != tt.report.model {
				t.Errorf("Model=%q, 期待=%q", got.Model, tt.report.model)
			}
		})
	}
}
