package chat

import (
	"regexp"
	"strconv"
	"strings"
)

// 「1. ソフトウェアエンジニア（Webサービス開発）」のような番号付き選択肢を拾う。
//
// 全角の数字・ピリオド・括弧も来るため、正規化してから処理する。
var presentedOptionPattern = regexp.MustCompile(`^\s*(\d{1,2})\s*[.．:：)）]\s*(.+?)\s*$`)

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
	if strings.TrimSpace(questionText) == "" {
		return nil
	}

	options := make([]string, 0, 8)
	expected := 1
	for _, line := range strings.Split(normalizeOptionText(questionText), "\n") {
		m := presentedOptionPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n != expected {
			// 番号が飛んだら選択肢リストではない（本文中の「2. 」等）。
			// 1から連番で並んでいるものだけを選択肢とみなす。
			continue
		}
		name := strings.TrimSpace(optionSuffixPattern.ReplaceAllString(m[2], ""))
		if name == "" {
			continue
		}
		options = append(options, name)
		expected++
	}
	if len(options) < 2 {
		// 1件だけ拾えた場合は選択肢ではなく箇条書きの可能性が高い。
		return nil
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
