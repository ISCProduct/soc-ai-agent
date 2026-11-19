package interview

import (
	"slices"
	"testing"

	"Backend/internal/models"
)

// SOCAIAGENT-675: 漢数字の数値化と、カタカナ・英字の語の照合。

func TestExtractNumbers_漢数字(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"助数詞つき", "売上を三倍にしました", []string{"3"}},
		{"位取り", "五十人をまとめました", []string{"50"}},
		{"位取りは助数詞が無くても数値", "全部で二百あります", []string{"200"}},
		{"万を含む位取り", "三万二千円でした", []string{"32000"}},
		{"桁を並べた年", "二〇二五年に始めました", []string{"2025"}},
		{"二つの数値", "二週間で百件", []string{"2", "100"}},
		{"一緒・一番は数値にしない（U5）", "一緒に働く人と一番の目標を決めました", nil},
		{"一つ・一度は数値にしない", "一つずつ一度に片付けました", nil},
		{"十分（じゅうぶん）は数値にしない", "十分に準備しました", nil},
		{"一時的は数値にしない", "一時的に止めました", nil},
		{"位取り1文字だけの語は数値にしない", "百貨店と千葉と万全", nil},
		{"算用数字と万", "200万円の売上", []string{"2000000"}},
		{"算用数字と漢数字の万は同じ値", "二百万円の売上", []string{"2000000"}},
		{"小数の万", "1.5万人", []string{"15000"}},
		{"全角数字と桁区切り（従来どおり）", "３,０００円", []string{"3000"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNumbers(tt.in)
			slices.Sort(got)
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("extractNumbers(%q) = %v, want %v", tt.in, got, want)
			}
		})
	}
}

func TestExtractTerms(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"カタカナと英字", "Pythonでエクセルの集計を自動化", []string{"python", "エクセル"}},
		{"全角英字・半角カタカナは同じ語", "ＧｉｔＨｕｂとｶﾌｪ", []string{"github", "カフェ"}},
		{"1文字の語・長音だけは語にしない", "AとアとーーとC#", []string{"c#"}},
		{"重複は1つ", "SNSとsnsとSNS", []string{"sns"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractTerms(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("extractTerms(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateEvidence_漢数字と語の照合(t *testing.T) {
	spoken := SpokenContent([]models.InterviewUtterance{
		{Role: "user", Text: "Javaで在庫管理のツールを作り、作業時間を三割減らしました。"},
		// 面接官の発話の語は、受験者が言ったことにならない
		{Role: "ai", Text: "Pythonは使いましたか。"},
	})
	tests := []struct {
		name      string
		evidence  string
		wantMatch bool
	}{
		{"発話どおりの引用", "Javaで在庫管理のツールを作り、作業時間を三割減らしました", true},
		{"漢数字を算用数字で引用", "Javaで在庫管理のツールを作り、作業時間を3割減らしました", true},
		{"英字の大文字小文字の違いは同じ語", "JAVAで在庫管理のツールを作りました", true},
		{"言っていない技術名", "Pythonで在庫管理のツールを作りました", false},
		{"数値の入れ替え（漢数字）", "Javaで在庫管理のツールを作り、作業時間を五割減らしました", false},
		{"教員向けの講評の語は照合しない", "「Javaで在庫管理のツールを作り」と主体性がある。ただしチームでの分担に触れていない", true},
		{"教員向けでも引用の中の語は照合する", "「Pythonで在庫管理のツールを作り」と主体性がある", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := len(ValidateEvidence(map[string]string{"logic": tt.evidence}, spoken).Unmatched) == 0
			if matched != tt.wantMatch {
				t.Fatalf("照合=%v, want %v: %q", matched, tt.wantMatch, tt.evidence)
			}
		})
	}
}
