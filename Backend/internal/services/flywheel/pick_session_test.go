package flywheel

import (
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func TestPickDiagnosisSessionID(t *testing.T) {
	dbErr := errors.New("connection refused")

	tests := []struct {
		name        string
		latest      string
		err         error
		want        string
		wantErrIs   error
		wantErrNone bool
	}{
		{name: "チャット診断があればそれを使う", latest: "chat-abc", want: "chat-abc", wantErrNone: true},
		{name: "診断が無ければスナップショットへ", latest: "", err: gorm.ErrRecordNotFound, want: "interview-8", wantErrNone: true},
		{name: "空文字もスナップショットへ", latest: "   ", want: "interview-8", wantErrNone: true},
		{name: "面接スナップショットは診断とみなさない", latest: "interview-8", want: "interview-8", wantErrNone: true},
		{name: "一時エラーはフォールバックせず返す", err: dbErr, wantErrIs: dbErr},
		{
			name:        "ラップされた ErrRecordNotFound もフォールバック",
			err:         fmt.Errorf("検索に失敗: %w", gorm.ErrRecordNotFound),
			want:        "interview-8",
			wantErrNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PickDiagnosisSessionID(8, tt.latest, tt.err)
			if tt.wantErrNone && err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("err = %v, want %v", err, tt.wantErrIs)
				}
				if got != "" {
					t.Fatalf("エラー時は空文字を返すべき: got %q", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
