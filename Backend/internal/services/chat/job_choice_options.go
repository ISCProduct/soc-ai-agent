package chat

import (
	"regexp"
	"strconv"
	"strings"
)

// 「1. ソフトウェアエンジニア（Webサービス開発）」のような番号付き選択肢を拾う。
//
// 改行区切りとは限らない。LLM が生成する質問文は
// 「近いのはどれですか？ 1. A 2. B 3. C」のように1行で並べることもあるため、
// 行単位ではなく本文全体から番号マーカーを探す。
var optionMarkerPattern = regexp.MustCompile(`(\d{1,2})\s*[.．:：)）]\s*`)

// 選択肢名の後ろに付く補足。「Webエンジニア（Webサービス・フロント/バックエンド開発）」の
// 括弧部分は職種名ではないので、判定に渡す前に落とす。
var optionSuffixPattern = regexp.MustCompile(`[（(].*$`)

// ExtractPresentedOptions は質問文に提示された選択肢を番号順に返す。
//
// 返すのは「利用者が画面で見た選択肢そのもの」。番号の解釈は必ずこれに対して行う。
// 職種の選択肢は2種類あり、どちらも同じ「N. 名前」形式で提示される。
//
//   - GenerateJobSelectionQuestion が作る大分類の一覧
//   - AI が「もっと具体的に」と聞き返すときの小分類の一覧
//
// 番号が何番目の選択肢を指すかは、そのとき提示された一覧にしか書かれていない。
// 一覧を固定のマスタ順で解釈すると、小分類に答えた番号を大分類として読む。
func ExtractPresentedOptions(questionText string) []string {
	text := normalizeOptionText(questionText)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	marks := optionMarkerPattern.FindAllStringSubmatchIndex(text, -1)

	// 1 から始まる連番だけを選択肢とみなす。本文中にたまたま現れた
	// 「3. の観点について」のような数字を拾わないため。
	kept := make([][]int, 0, len(marks))
	expected := 1
	for _, m := range marks {
		n, err := strconv.Atoi(text[m[2]:m[3]])
		if err != nil || n != expected {
			continue
		}
		kept = append(kept, m)
		expected++
	}
	if len(kept) < 2 {
		return nil
	}

	options := make([]string, 0, len(kept))
	for i, m := range kept {
		// 名前は「マーカーの直後」から「次のマーカー」か「行末」の手前まで。
		// 1行に並んでいても、改行区切りでも、同じ規則で切り出せる。
		end := len(text)
		if i+1 < len(kept) {
			end = kept[i+1][0]
		}
		if nl := strings.IndexByte(text[m[1]:end], '\n'); nl >= 0 {
			end = m[1] + nl
		}
		name := strings.TrimSpace(optionSuffixPattern.ReplaceAllString(text[m[1]:end], ""))
		if name == "" {
			return nil
		}
		options = append(options, name)
	}
	return options
}

// OptionForChoice は提示された選択肢のうち choice 番目を返す。
// 範囲外・選択肢なしのときは空文字。
func OptionForChoice(options []string, choice int) string {
	if choice < 1 || choice > len(options) {
		return ""
	}
	return options[choice-1]
}

var fullWidthDigits = strings.NewReplacer(
	"０", "0", "１", "1", "２", "2", "３", "3", "４", "4",
	"５", "5", "６", "6", "７", "7", "８", "8", "９", "9",
)

func normalizeOptionText(s string) string {
	return fullWidthDigits.Replace(s)
}
