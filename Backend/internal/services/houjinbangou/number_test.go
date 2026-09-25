package houjinbangou

import "testing"

func TestValidateNumber(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		// 国税庁APIで実在を確認済みの番号。
		{name: "トヨタ自動車", number: "1180301018771", want: true},
		{name: "日本電気", number: "7010401022916", want: true},
		{name: "味の素", number: "8010001034740", want: true},
		{name: "三菱ＵＦＪ信託銀行", number: "6010001008770", want: true},

		// かつてシードにハードコードされていた、登記に存在しない番号。
		{name: "捏造(旧seed:トヨタ自動車)", number: "7010401026738", want: false},
		{name: "捏造(旧seed:NEC)", number: "4010401019905", want: false},
		{name: "捏造(旧seed:味の素)", number: "9060001000184", want: false},

		{name: "空文字", number: "", want: false},
		{name: "桁不足", number: "118030101877", want: false},
		{name: "桁超過", number: "11803010187710", want: false},
		{name: "数字以外を含む", number: "118030101877X", want: false},
		{name: "前後の空白は許容", number: " 1180301018771 ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateNumber(tt.number); got != tt.want {
				t.Errorf("ValidateNumber(%q) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}
