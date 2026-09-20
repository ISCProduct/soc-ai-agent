package admin

import (
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
