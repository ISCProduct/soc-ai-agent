package masking

import (
	"regexp"
	"strings"
	"testing"
)

// leakCase は項目ごとの「置き換わるべき例文」。
type leakCase struct {
	dict    Dictionary
	input   string
	secrets []string // 出力に残ってはいけない元の値
}

// leakCases は DefaultRules の全項目の例文。TestMask_NoLeakForEveryRule が DefaultRules を回して
// ここを引くので、項目を足して例文を足し忘れるとテストが落ちる。
var leakCases = map[string]leakCase{
	KeyName: {
		dict:    Dictionary{KeyName: NameTerms("山田 太郎")},
		input:   "はじめまして、山田太郎です。山田と呼んでください。",
		secrets: []string{"山田", "太郎"},
	},
	KeyEmail: {
		input:   "連絡先はtaro.yamada+job@example.co.jpです。",
		secrets: []string{"taro.yamada", "example.co.jp"},
	},
	KeySchool: {
		dict:    Dictionary{KeySchool: {"東京大学"}},
		input:   "東京大学で情報工学を学びました。",
		secrets: []string{"東京大学"},
	},
	KeyCompany: {
		dict:    Dictionary{KeyCompany: {"株式会社サンプル商事"}},
		input:   "株式会社サンプル商事でインターンをしました。",
		secrets: []string{"サンプル商事"},
	},
	KeyPhone: {
		input:   "電話番号は090-1234-5678です。",
		secrets: []string{"090", "1234", "5678"},
	},
	KeyAddress: {
		input:   "住所は東京都千代田区霞が関1-2-3です。",
		secrets: []string{"東京都", "千代田区", "霞が関", "1-2-3"},
	},
}

func ruleByKey(t *testing.T, key string) Rule {
	t.Helper()
	for _, r := range DefaultRules() {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("項目 %q が DefaultRules にありません", key)
	return Rule{}
}

func mustNew(t *testing.T, rules []Rule, dict Dictionary) *Masker {
	t.Helper()
	m, err := New(rules, dict)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func mask(t *testing.T, dict Dictionary, text string) string {
	t.Helper()
	return mustNew(t, DefaultRules(), dict).Mask(text)
}

// 全項目を同じ検査にかける：元の値が残らず、その項目の記号が入る。
func TestMask_NoLeakForEveryRule(t *testing.T) {
	for _, rule := range DefaultRules() {
		t.Run(rule.Key, func(t *testing.T) {
			tc, ok := leakCases[rule.Key]
			if !ok {
				t.Fatalf("項目 %q の例文が leakCases にありません。項目を足したら例文も足してください", rule.Key)
			}
			got := mask(t, tc.dict, tc.input)
			for _, s := range tc.secrets {
				if strings.Contains(got, s) {
					t.Errorf("出力に %q が残っています: %q", s, got)
				}
			}
			if !strings.Contains(got, rule.Token) {
				t.Errorf("出力に %s がありません: %q", rule.Token, got)
			}
		})
	}
}

// 受け入れ条件：全項目を含む発話から、どれも残らない。行の数（改行）も変わらない。
func TestMask_AllRulesInOneText(t *testing.T) {
	dict := Dictionary{}
	var lines, secrets []string
	for _, rule := range DefaultRules() {
		tc := leakCases[rule.Key]
		for k, v := range tc.dict {
			dict[k] = append(dict[k], v...)
		}
		lines = append(lines, tc.input)
		secrets = append(secrets, tc.secrets...)
	}
	input := strings.Join(lines, "\n")
	got := mask(t, dict, input)
	for _, s := range secrets {
		if strings.Contains(got, s) {
			t.Errorf("出力に %q が残っています: %q", s, got)
		}
	}
	for _, rule := range DefaultRules() {
		if !strings.Contains(got, rule.Token) {
			t.Errorf("出力に %s がありません: %q", rule.Token, got)
		}
	}
	if gotLines := strings.Count(got, "\n"); gotLines != len(lines)-1 {
		t.Errorf("改行の数 = %d, want %d: %q", gotLines, len(lines)-1, got)
	}
}

func TestMask_LongestNameFirst(t *testing.T) {
	tests := []struct {
		name  string
		dict  Dictionary
		input string
		want  string
	}{
		{
			name:  "同じ項目で短い名称が先に並んでいても長い名称を採る",
			dict:  Dictionary{KeySchool: {"東京大学", "東京大学大学院"}},
			input: "東京大学大学院に進学します。",
			want:  "<SCHOOL>に進学します。",
		},
		{
			name:  "短い名称だけの箇所は短い名称で置き換える",
			dict:  Dictionary{KeySchool: {"東京大学", "東京大学大学院"}},
			input: "東京大学の研究室",
			want:  "<SCHOOL>の研究室",
		},
		{
			name:  "項目をまたいでも長い名称を採る",
			dict:  Dictionary{KeySchool: {"東京大学"}, KeyCompany: {"東京大学出版会"}},
			input: "東京大学出版会に応募しました。",
			want:  "<COMPANY>に応募しました。",
		},
		{
			name:  "長い名称の後ろに含まれる短い名称は残らない",
			dict:  Dictionary{KeyCompany: {"サンプル"}, KeySchool: {"情報サンプル大学"}},
			input: "情報サンプル大学の出身です。",
			want:  "<SCHOOL>の出身です。",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mask(t, tt.dict, tt.input); got != tt.want {
				t.Errorf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMask_DictionaryVariants(t *testing.T) {
	nameDict := Dictionary{KeyName: NameTerms("山田 太郎")}
	tests := []struct {
		name  string
		dict  Dictionary
		input string
		want  string
	}{
		{"姓名の連結", nameDict, "山田太郎です", "<NAME>です"},
		{"半角スペースあり", nameDict, "山田 太郎です", "<NAME>です"},
		{"全角スペースあり", nameDict, "山田　太郎です", "<NAME>です"},
		{"姓だけ", nameDict, "山田です", "<NAME>です"},
		{"名だけ", nameDict, "太郎と呼ばれています", "<NAME>と呼ばれています"},
		{"名姓の順", nameDict, "太郎山田", "<NAME>"},
		{"辞書が空白無しで本文が空白あり", Dictionary{KeyName: {"山田太郎"}}, "山田 太郎です", "<NAME>です"},
		{"中黒の有無", Dictionary{KeyName: {"ジョン・スミス"}}, "ジョンスミスです", "<NAME>です"},
		{"半角カナと濁点", Dictionary{KeyName: {"ヤマダ タロウ"}}, "ﾔﾏﾀﾞ ﾀﾛｳです", "<NAME>です"},
		{"全角英字と大文字小文字", Dictionary{KeyName: NameTerms("Taro Yamada")}, "ＴＡＲＯ　ＹＡＭＡＤＡです", "<NAME>です"},
		{"辞書が全角で本文が半角", Dictionary{KeyCompany: {"ＡＢＣ商事"}}, "abc商事に応募", "<COMPANY>に応募"},
		{"辞書の前後の空白", Dictionary{KeySchool: {"  東京大学  "}}, "東京大学です", "<SCHOOL>です"},
		// 正準等価な書き方（合成済みの é と、e + 結合用のアキュート U+0301）を同じ名称として扱う。
		{"辞書が合成済みで本文が結合文字", Dictionary{KeyName: {"Jos\u00e9"}}, "Jose\u0301です", "<NAME>です"},
		{"辞書が結合文字で本文が合成済み", Dictionary{KeyName: {"Jose\u0301"}}, "Jos\u00e9です", "<NAME>です"},
		{"大文字の結合文字も小文字の合成済みと一致する", Dictionary{KeyName: {"\u00c9ric"}}, "E\u0301RICです", "<NAME>です"},
		{"本文が結合文字の濁点", Dictionary{KeyName: {"ヤマダ"}}, "ヤマタ\u3099です", "<NAME>です"},
		// 結合文字は基底文字と1つの単位なので、アクセントの無い名称が一致して結合文字だけが残ることはない。
		{"アクセントの無い名称はアクセント付きの綴りに一致しない", Dictionary{KeyName: {"Jose"}}, "Jose\u0301です", "Jose\u0301です"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mask(t, tt.dict, tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMask_DictionaryBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		dict  Dictionary
		input string
		want  string
	}{
		{"英字の名称は英単語の途中に一致しない", Dictionary{KeyCompany: {"ABC"}}, "ABCDEF社とXABC社とABC社", "ABCDEF社とXABC社と<COMPANY>社"},
		{"英字の名称は全角英字の途中にも一致しない", Dictionary{KeyCompany: {"ABC"}}, "ＡＢＣＤ社とＸＡＢＣ社", "ＡＢＣＤ社とＸＡＢＣ社"},
		{"英字の名称は日本語に挟まれていれば一致する", Dictionary{KeyCompany: {"ABC"}}, "株式会社ABCに", "株式会社<COMPANY>に"},
		{"英字の名称は空白や記号に挟まれていれば一致する", Dictionary{KeyName: NameTerms("Taro Yamada")}, "I'm Yamada, taro.", "I'm <NAME>, <NAME>."},
		{"ラテン文字の名称はアクセント付きの単語の途中に一致しない", Dictionary{KeyName: {"\u00c9ric"}}, "Fr\u00e9d\u00e9ric", "Fr\u00e9d\u00e9ric"},
		{"ラテン文字の名称は結合文字で書いた単語の途中にも一致しない", Dictionary{KeyName: {"\u00c9ric"}}, "Fre\u0301de\u0301ric", "Fre\u0301de\u0301ric"},
		{"英字の名称は結合文字で終わる単語の直後に一致しない", Dictionary{KeyName: {"Eric"}}, "Jose\u0301Eric", "Jose\u0301Eric"},
		{"英字の名称はアクセント付きの文字の直後に一致しない", Dictionary{KeyCompany: {"ABC"}}, "\u00e9ABC社", "\u00e9ABC社"},
		{"英字の名称はアクセント付きの文字の直前に一致しない", Dictionary{KeyCompany: {"ABC"}}, "ABC\u00e9社", "ABC\u00e9社"},
		{"ラテン文字の名称は空白の前で一致する", Dictionary{KeyName: {"\u00c9ric"}}, "\u00c9ric さん", "<NAME> さん"},
		{"ラテン文字の名称は読点の前で一致する", Dictionary{KeyName: {"\u00c9ric"}}, "\u00c9ric、よろしく", "<NAME>、よろしく"},
		{"ラテン文字の名称は日本語に挟まれていれば一致する", Dictionary{KeyName: {"\u00c9ric"}}, "私は\u00c9ricです", "私は<NAME>です"},
		{"ラテン文字の名称は単独の結合文字の直後なら一致する", Dictionary{KeyName: {"\u00c9ric"}}, "\u0301\u00c9ric", "\u0301<NAME>"},
		{"1文字の名称は照合しない", Dictionary{KeyName: {"林"}}, "林さんと森林を歩いた", "林さんと森林を歩いた"},
		{"2文字の名称は照合する", Dictionary{KeyName: {"小林"}}, "小林です", "<NAME>です"},
		{"行をまたぐ名称は一致させない", Dictionary{KeyName: {"山田太郎"}}, "山田\n太郎", "山田\n太郎"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mask(t, tt.dict, tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// 置き換えた記号や、ほかの項目の一致に、辞書の名称が重ねて一致しないこと。
func TestMask_OverlappingRules(t *testing.T) {
	tests := []struct {
		name  string
		dict  Dictionary
		input string
		want  string
	}{
		{"メールの中の氏名はメールとして置き換える", Dictionary{KeyName: {"yamada"}}, "yamada@example.com", "<EMAIL>"},
		{"メールの途中から始まる氏名もメールに含める", Dictionary{KeyName: {"yamada"}}, "taro.yamada@example.com", "<EMAIL>"},
		{"記号と同じ綴りの名称があっても記号を壊さない", Dictionary{KeyCompany: {"EMAIL"}}, "a@example.com と EMAIL社", "<EMAIL> と <COMPANY>社"},
		{"辞書と正規表現が同じ範囲なら定義で先の項目を採る", Dictionary{KeyName: {"taro@example.com"}}, "taro@example.comです", "<NAME>です"},
		{"同じ位置・同じ長さなら定義で先の項目を採る", Dictionary{KeyCompany: {"サンプル学園"}, KeySchool: {"サンプル学園"}}, "サンプル学園です", "<SCHOOL>です"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mask(t, tt.dict, tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMask_Email(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"メールはtaro@example.comです", "メールは<EMAIL>です"},
		{"taro.yamada_01@mail.example.ac.jp", "<EMAIL>"},
		{"ｔａｒｏ＠ｅｘａｍｐｌｅ．ｊｐです", "<EMAIL>です"},
		// ローカル部の RFC 5322 の atext の記号も含めて、全体を1つのメールとして置き換える。
		{"john!doe@example.com", "<EMAIL>"},
		{"o'brien@example.com", "<EMAIL>"},
		{"a+b@example.com", "<EMAIL>"},
		{"a!#$%&'*+/=?^_`{|}~-.z@example.com", "<EMAIL>"},
		{"連絡は john!doe@example.com まで", "連絡は <EMAIL> まで"},
		{"メールはo'brien@example.comです", "メールは<EMAIL>です"},
		{"（a+b@example.com）", "（<EMAIL>）"},
		// メールでない「@」は残す。
		{"@で区切ります", "@で区切ります"},
		{"a@b", "a@b"},
		{"taro@example", "taro@example"},
		{"john!doe@ まで", "john!doe@ まで"},
	}
	m := mustNew(t, []Rule{ruleByKey(t, KeyEmail)}, nil)
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := m.Mask(tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMask_Phone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"携帯・ハイフンあり", "090-1234-5678です", "<PHONE>です"},
		{"携帯・ハイフンなし", "09012345678です", "<PHONE>です"},
		{"固定・ハイフンあり", "03-1234-5678", "<PHONE>"},
		{"固定・ハイフンなし", "0312345678", "<PHONE>"},
		{"固定・括弧", "03(1234)5678", "<PHONE>"},
		{"フリーダイヤル", "0120-123-456", "<PHONE>"},
		{"全角数字と全角ハイフン", "０９０－１２３４－５６７８です", "<PHONE>です"},
		{"国番号付き", "+81-90-1234-5678", "<PHONE>"},
		{"国番号付き・直後に空白", "+81 90-1234-5678", "<PHONE>"},
		{"国番号付き・固定", "+81-3-1234-5678", "<PHONE>"},
		{"国番号付き・0を残す", "+81-090-1234-5678", "<PHONE>"},
		{"国番号付き・括弧の0", "+81(0)90-1234-5678", "<PHONE>"},
		{"前後が日本語", "電話は09012345678まで", "電話は<PHONE>まで"},
		{"番号のあとの空白と数字は取り込まない", "03-1234-5678 2名", "<PHONE> 2名"},
		// 誤検出しないもの。
		{"郵便番号", "〒100-0001", "〒100-0001"},
		{"郵便番号・記号なし", "1000001", "1000001"},
		{"金額", "1,000,000円", "1,000,000円"},
		{"0で始まらない11桁", "12345678901", "12345678901"},
		{"日付", "2026-04-01", "2026-04-01"},
		{"9桁は電話番号でない", "012345678", "012345678"},
		{"長い数字列の頭", "012345678901234", "012345678901234"},
		{"直前が数字", "5090-1234-5678", "5090-1234-5678"},
	}
	m := mustNew(t, []Rule{ruleByKey(t, KeyPhone)}, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.Mask(tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMask_Address(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"番地まで", "東京都千代田区霞が関1-2-3に住んでいます", "<ADDRESS>に住んでいます"},
		{"丁目と番地", "大阪府大阪市北区梅田3丁目1番地です", "<ADDRESS>です"},
		{"政令市の区", "神奈川県横浜市中区に住んでいます", "<ADDRESS>に住んでいます"},
		{"郡と町", "北海道虻田郡倶知安町出身", "<ADDRESS>出身"},
		{"条の住所", "北海道札幌市中央区北1条西2丁目です", "<ADDRESS>です"},
		{"ひらがなの市", "埼玉県さいたま市大宮区です", "<ADDRESS>です"},
		{"全角数字の番地", "京都府京都市左京区吉田本町３６です", "<ADDRESS>です"},
		{"東京都と府中市を取り違えない", "東京都府中市", "<ADDRESS>"},
		{"市区町村のあとの語は数字が無ければ残す", "東京都千代田区出身です", "<ADDRESS>出身です"},
		// 誤検出しないもの。
		{"都道府県だけ", "東京都の大学に通っています", "東京都の大学に通っています"},
		{"都道府県＋出身", "福岡県出身です", "福岡県出身です"},
		{"市区町村だけ", "千代田区霞が関1-2-3", "千代田区霞が関1-2-3"},
	}
	m := mustNew(t, []Rule{ruleByKey(t, KeyAddress)}, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.Mask(tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// 置き換えた箇所以外（改行・前後の文）はそのまま残る。
func TestMask_PreservesStructure(t *testing.T) {
	dict := Dictionary{KeyName: NameTerms("山田 太郎"), KeySchool: {"東京大学"}}
	input := "1行目：山田太郎です。\n\n3行目：東京大学の学生です。\r\n4行目：電話は090-1234-5678。\n"
	want := "1行目：<NAME>です。\n\n3行目：<SCHOOL>の学生です。\r\n4行目：電話は<PHONE>。\n"
	if got := mask(t, dict, input); got != want {
		t.Errorf("Mask() = %q, want %q", got, want)
	}
}

func TestMask_NoMatchReturnsInput(t *testing.T) {
	input := "特に個人情報は含みません。\n2行目です。"
	if got := mask(t, Dictionary{KeySchool: {"東京大学"}}, input); got != input {
		t.Errorf("Mask() = %q, want %q", got, input)
	}
}

func TestMask_EmptyInputsDoNotPanic(t *testing.T) {
	tests := []struct {
		name  string
		rules []Rule
		dict  Dictionary
		input string
		want  string
	}{
		{"空文字・辞書が nil", DefaultRules(), nil, "", ""},
		{"空文字・辞書あり", DefaultRules(), Dictionary{KeyName: {"山田"}}, "", ""},
		{"辞書が nil", DefaultRules(), nil, "山田です", "山田です"},
		{"辞書が空", DefaultRules(), Dictionary{}, "山田です", "山田です"},
		{"辞書の項目が空", DefaultRules(), Dictionary{KeyName: {}}, "山田です", "山田です"},
		{"辞書の名称が空文字と空白だけ", DefaultRules(), Dictionary{KeyName: {"", " ", "　", "・"}}, "山田です", "山田です"},
		{"項目が無い", nil, nil, "090-1234-5678", "090-1234-5678"},
		{"不正な UTF-8", DefaultRules(), Dictionary{KeyName: {"山田"}}, "\xff山田\xfe", "\xff<NAME>\xfe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustNew(t, tt.rules, tt.dict).Mask(tt.input); got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// 項目は定義を1つ足すだけで増やせる。
func TestNew_CustomRule(t *testing.T) {
	rules := append(DefaultRules(),
		Rule{Key: "student_id", Token: "<STUDENT_ID>", Method: MethodPattern, Pattern: regexp.MustCompile(`S[0-9]{7}`)},
		Rule{Key: "club", Token: "<CLUB>", Method: MethodDictionary},
	)
	m := mustNew(t, rules, Dictionary{"club": {"軽音楽部"}, KeyName: {"山田"}})
	got := m.Mask("山田です。学籍番号はS1234567、軽音楽部でした。")
	want := "<NAME>です。学籍番号は<STUDENT_ID>、<CLUB>でした。"
	if got != want {
		t.Errorf("Mask() = %q, want %q", got, want)
	}
	// 空文字に一致しうる正規表現でも、空の一致は置き換えない（記号を文字の間に挟まない）。
	digits := mustNew(t, []Rule{{Key: "digits", Token: "<N>", Method: MethodPattern, Pattern: regexp.MustCompile(`[0-9]*`)}}, nil)
	if got, want := digits.Mask("a1b"), "a<N>b"; got != want {
		t.Errorf("Mask() = %q, want %q", got, want)
	}
	// DefaultRules は呼び出すたびに新しいスライスなので、append の影響を受けない。
	if n := len(DefaultRules()); n != 6 {
		t.Errorf("len(DefaultRules()) = %d, want 6", n)
	}
}

func TestNew_Errors(t *testing.T) {
	dictRule := Rule{Key: "a", Token: "<A>", Method: MethodDictionary}
	tests := []struct {
		name  string
		rules []Rule
		dict  Dictionary
	}{
		{"Key が空", []Rule{{Token: "<A>", Method: MethodDictionary}}, nil},
		{"Key の重複", []Rule{dictRule, dictRule}, nil},
		{"記号が空", []Rule{{Key: "a", Method: MethodDictionary}}, nil},
		{"照合の方法が未設定", []Rule{{Key: "a", Token: "<A>"}}, nil},
		{"正規表現の無い MethodPattern", []Rule{{Key: "a", Token: "<A>", Method: MethodPattern}}, nil},
		{"辞書のキーが定義に無い", []Rule{dictRule}, Dictionary{"b": {"名称"}}},
		{"辞書で照合しない項目の辞書", DefaultRules(), Dictionary{KeyEmail: {"taro@example.com"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.rules, tt.dict); err == nil {
				t.Error("New() error = nil, want error")
			}
		})
	}
}

func TestDefaultRules_KeysAndTokensAreUnique(t *testing.T) {
	keys, tokens := map[string]bool{}, map[string]bool{}
	for _, r := range DefaultRules() {
		if keys[r.Key] || tokens[r.Token] {
			t.Errorf("項目 %q（%s）が重複しています", r.Key, r.Token)
		}
		keys[r.Key], tokens[r.Token] = true, true
	}
}

func TestNameTerms(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"半角スペース区切り", "山田 太郎", []string{"山田太郎", "太郎山田", "山田", "太郎"}},
		{"全角スペース区切り", "山田　太郎", []string{"山田太郎", "太郎山田", "山田", "太郎"}},
		{"中黒区切り", "ジョン・スミス", []string{"ジョンスミス", "スミスジョン", "ジョン", "スミス"}},
		{"前後と連続の空白", "  山田   太郎 ", []string{"山田太郎", "太郎山田", "山田", "太郎"}},
		{"区切り無し", "山田太郎", []string{"山田太郎"}},
		{"空文字", "", nil},
		{"空白だけ", " 　", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NameTerms(tt.in)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
				t.Errorf("NameTerms(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
