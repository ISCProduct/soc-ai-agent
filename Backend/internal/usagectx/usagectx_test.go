package usagectx

import (
	"context"
	"testing"
)

// TestFeature は機能名の出し入れと既定値を検証する（#1294）。
func TestFeature(t *testing.T) {
	tests := []struct {
		name  string
		setup func() context.Context
		want  string
	}{
		{"未設定はunknown", func() context.Context { return context.Background() }, FeatureUnknown},
		{"設定した値が取れる", func() context.Context {
			return WithFeature(context.Background(), FeatureESReview)
		}, FeatureESReview},
		{"空文字は上書きしない", func() context.Context {
			return WithFeature(WithFeature(context.Background(), FeatureChatSummary), "")
		}, FeatureChatSummary},
		{"後勝ちで上書きできる", func() context.Context {
			return WithFeature(WithFeature(context.Background(), FeatureChatSummary), FeatureMatchingReason)
		}, FeatureMatchingReason},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Feature(tt.setup()); got != tt.want {
				t.Errorf("Feature()=%q, 期待=%q", got, tt.want)
			}
		})
	}
}

// TestActor は実行主体と配賦先の出し入れを検証する。
// バッチ経路は主体を持たないため、nil のまま記録されることが正しい。
func TestActor(t *testing.T) {
	tests := []struct {
		name    string
		userID  uint
		orgID   uint
		wantUsr *uint
		wantOrg *uint
	}{
		{"両方あり", 42, 7, ptr(42), ptr(7)},
		{"ユーザーのみ(組織未解決)", 42, 0, ptr(42), nil},
		{"どちらも無い場合は載せない", 0, 0, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithActor(context.Background(), tt.userID, tt.orgID)
			gotUsr, gotOrg := Actor(ctx)
			assertPtr(t, "userID", gotUsr, tt.wantUsr)
			assertPtr(t, "organizationID", gotOrg, tt.wantOrg)
		})
	}
}

// TestActorUnset は未設定のコンテキストで nil が返ることを検証する。
func TestActorUnset(t *testing.T) {
	usr, org := Actor(context.Background())
	if usr != nil || org != nil {
		t.Errorf("未設定なのに値が返った: user=%v org=%v", usr, org)
	}
}

func ptr(v uint) *uint { return &v }

func assertPtr(t *testing.T, label string, got, want *uint) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s: got=%v, want=%v", label, got, want)
	case *got != *want:
		t.Errorf("%s: got=%d, want=%d", label, *got, *want)
	}
}
