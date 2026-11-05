package textcheck

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func slHits(text string, fs []Finding) []string {
	u := utf16.Encode([]rune(text))
	var out []string
	for _, f := range fs {
		out = append(out, string(utf16.Decode(u[f.Start:f.End])))
	}
	return out
}

func TestSpokenLanguage(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string
	}{
		{"すごく", "私はすごく頑張った。", []string{"すごく"}},
		{"やっぱりは1件", "やっぱり続けた。", []string{"やっぱり"}},
		{"やっぱ", "やっぱ続けた。", []string{"やっぱ"}},
		{"ちゃんと・いっぱい", "ちゃんと準備し、いっぱい学んだ。", []string{"ちゃんと", "いっぱい"}},
		{"アルバイトは対象外", "飲食店のアルバイトをした。", nil},
		{"バイトは略語", "飲食店のバイトをした。", []string{"バイト"}},
		{"メガバイトは対象外", "500メガバイトのデータ", nil},
		{"就活", "就活を通じて学んだ。", []string{"就活"}},
		{"文頭のなので", "失敗した。なので、改善した。", []string{"なので、"}},
		{"文中のなのでは対象外", "大学なので、通いやすい。", nil},
		{"箇条書きの文頭", "・でも、続けた。", []string{"でも、"}},
		{"改行のあとの文頭", "失敗した\nだから、改善した。", []string{"だから、"}},
		{"読点が無ければ対象外", "でも私は続けた。", nil},
		{"引用の中は対象外", "「すごく良い」と言われた。", nil},
		{"閉じ忘れた引用は改行で終わる", "「閉じ忘れ\nすごく頑張った。", []string{"すごく"}},
		{"させていただは2回目から", "担当させていただきます。協力させていただきました。", []string{"させていただ"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := SpokenLanguage{}.Check(tc.text)
			for _, f := range fs {
				if f.Rule != "spoken_language" {
					t.Errorf("Rule = %q", f.Rule)
				}
			}
			if got := slHits(tc.text, fs); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpokenLanguage_Severity(t *testing.T) {
	fs := SpokenLanguage{}.Check("やっぱり。させていただく。させていただく。")
	if len(fs) != 2 || fs[0].Severity != SeverityWarning || fs[1].Severity != SeverityInfo {
		t.Fatalf("got %+v", fs)
	}
}

func TestMixedStyle_QuoteDepthResetsAtNewline(t *testing.T) {
	if fs := (MixedStyle{}).Check("「閉じ忘れの引用\n私は学生です。私は走る。"); len(fs) == 0 {
		t.Fatal("閉じ忘れた引用が次の行まで続き、文体の混在を見逃した")
	}
}

func TestBlankFields(t *testing.T) {
	mk := func(texts ...string) []Block {
		bs := make([]Block, len(texts))
		for i, s := range texts {
			bs[i] = Block{Text: s, PageNumber: 1, BlockIndex: i}
		}
		return bs
	}
	cases := []struct {
		name   string
		blocks []Block
		want   []string
	}{
		{"最後の見出しは空欄", mk("氏名", "山田太郎", "志望動機"), []string{"志望動機"}},
		{"次が見出しなら空欄", mk("自己PR", "生年月日", "2000年1月1日"), []string{"自己PR"}},
		{"空のブロックは飛ばす", mk("住所", "  ", "東京都"), nil},
		{"必須でない見出しは見ない", mk("資格", "学歴"), nil},
		{"空白はそろえる", mk("氏　名", "住所", "東京都"), []string{"氏名"}},
		{"全角はそろえる", mk("自己ＰＲ"), []string{"自己PR"}},
		{"見出しが無ければ指摘しない", mk("山田太郎", "東京都"), nil},
		{"見出しを含む文は見出しではない", mk("志望動機は特にない"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, f := range BlankFields(tc.blocks) {
				if f.Rule != "blank_field" || f.Severity != SeverityInfo || f.PageHint != 1 {
					t.Errorf("got %+v", f)
				}
				got = append(got, normalizeHeading(tc.blocks[f.BlockIndex].Text))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCheckResume_MapsFindingsToBlocks(t *testing.T) {
	blocks := []Block{
		{Text: "私は学生です。", PageNumber: 1, BlockIndex: 0},
		{Text: "私は走る。", PageNumber: 1, BlockIndex: 1},
		{Text: "飲食店のバイトをしました。", PageNumber: 2, BlockIndex: 3},
		{Text: "2020年と２０２１年", PageNumber: 2, BlockIndex: 4},
		{Text: "志望動機", PageNumber: 2, BlockIndex: 5},
	}
	got := map[string]Finding{}
	for _, f := range CheckResume(blocks) {
		switch f.Rule {
		case "mixed_width", "mixed_numeral", "mixed_era", "char_limit":
			t.Errorf("履歴書では使わないルールが出た: %+v", f)
		}
		got[f.Rule] = f
	}

	ms, ok := got["mixed_style"]
	if !ok || ms.PageHint != 1 || ms.BlockIndex != 1 {
		t.Fatalf("mixed_style = %+v", ms)
	}
	if hit := slHits(blocks[1].Text, []Finding{ms})[0]; !strings.Contains(hit, "走る") {
		t.Errorf("mixed_style の位置がブロック内に戻っていない: %q", hit)
	}

	sl := got["spoken_language"]
	if sl.PageHint != 2 || sl.BlockIndex != 3 || sl.Start != 4 || sl.End != 7 {
		t.Errorf("spoken_language = %+v", sl)
	}
	if bf := got["blank_field"]; bf.PageHint != 2 || bf.BlockIndex != 5 {
		t.Errorf("blank_field = %+v", bf)
	}
}

func TestDefaultRules_IncludesSpokenLanguage(t *testing.T) {
	for _, r := range DefaultRules(0, "") {
		if r.Name() == "spoken_language" {
			return
		}
	}
	t.Fatal("ES のルールに spoken_language が無い")
}
