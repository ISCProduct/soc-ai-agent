package controllers

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"testing"
)

// TestChangesPublication は「編集 PUT からの公開」を検知できるかを見る。
// ここが漏れると PATCH /publish の platform 限定を PUT で迂回できる。
func TestChangesPublication(t *testing.T) {
	tests := []struct {
		name           string
		existing       models.Company
		payload        models.Company
		provisionalSet bool
		want           bool
	}{
		{
			name:     "公開状態を変えない編集は通す",
			existing: models.Company{DataStatus: "draft"},
			payload:  models.Company{Name: "新社名"},
			want:     false,
		},
		{
			name:     "draft から published は公開",
			existing: models.Company{DataStatus: "draft"},
			payload:  models.Company{DataStatus: "published"},
			want:     true,
		},
		{
			name:     "同じ data_status の送信は変更ではない",
			existing: models.Company{DataStatus: "published"},
			payload:  models.Company{DataStatus: "published"},
			want:     false,
		},
		{
			name:           "is_provisional を送って値が変われば公開状態の変更",
			existing:       models.Company{IsProvisional: true},
			payload:        models.Company{IsProvisional: false},
			provisionalSet: true,
			want:           true,
		},
		{
			name:     "is_provisional 未送信なら bool のゼロ値で誤検知しない",
			existing: models.Company{IsProvisional: true},
			payload:  models.Company{Name: "技術スタックだけ更新"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := changesPublication(&tt.existing, &tt.payload, tt.provisionalSet); got != tt.want {
				t.Fatalf("changesPublication() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
