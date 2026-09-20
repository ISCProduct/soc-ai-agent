package controllers

// chat_controller.go のレコメンド暫定判定のテスト。
// 以前は admin_company_publication_test.go に同居していたが、テスト対象は
// admin ではなく chat 側のため、admin の切り出しにあわせて分離した。

import (
	"Backend/domain/entity"
	"testing"
)

// TestMinMatchedAxisCount は根拠軸数の最小値の取り方を見る。
// 呼び出し側で表示対象の match だけに絞ってから渡す前提。
func TestMinMatchedAxisCount(t *testing.T) {
	tests := []struct {
		name    string
		matches []*entity.UserCompanyMatch
		want    int
	}{
		{name: "空なら0", want: 0},
		{
			name: "最小値を返す",
			matches: []*entity.UserCompanyMatch{
				{MatchedAxisCount: 6},
				{MatchedAxisCount: 2},
				{MatchedAxisCount: 4},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minMatchedAxisCount(tt.matches); got != tt.want {
				t.Fatalf("minMatchedAxisCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestIsRecommendationProvisional_MinAxes は根拠軸数が薄いときに暫定表示になるかを見る。
func TestIsRecommendationProvisional_MinAxes(t *testing.T) {
	const enoughCategories = 10

	if isRecommendationProvisional(enoughCategories, 90, nil, 6) {
		t.Fatal("根拠軸が十分なら暫定にしない")
	}
	if !isRecommendationProvisional(enoughCategories, 90, nil, 2) {
		t.Fatal("根拠軸が薄いなら暫定にする")
	}
	if isRecommendationProvisional(enoughCategories, 90, nil, 0) {
		t.Fatal("マッチが無い(0)ときは軸数を理由に暫定化しない")
	}
}
