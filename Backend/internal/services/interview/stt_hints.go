package interview

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// BuildSTTHints は面接コンテキストから音声認識の補助語を組み立てる（音声R&D Task 4）。
//
// Transcription API の prompt に渡す。実測では固有名詞の認識が改善し、
// 無関係な語を渡しても幻覚は起きなかった（RESULTS_stt_hints.md）。
//
// 特に「御社」は mini モデルが補助語なしだと8回中0回しか正しく取れず、
// 全て「本社」になった。面接では意味が変わるため、常に含める。
//
// 空でも従来どおり動くこと。企業未選択の面接でも面接は成立する。
// skillScores は補助語に使わない。models.SkillScore が持つのは
// カテゴリ（論理性・技術志向など）とスコアだけで、
// 「Go」「AWS」のような固有の技術名を持たないため補助にならない。
func BuildSTTHints(companyName, companyReading, position, companyInfo string) string {
	seen := map[string]bool{}
	hints := make([]string, 0, 16)

	add := func(s string) {
		s = strings.TrimSpace(s)
		// 1文字の語は誤検出を招くだけで補助にならない
		if len([]rune(s)) < 2 || seen[s] {
			return
		}
		// 補助語は「語」だけを通す（#1600）。
		// companyName / companyReading / position は multipart の
		// r.FormValue 直値で、ここは正規表現抽出を通らない。
		// 文章がそのまま Transcription API の prompt へ入ると、Whisper の prompt は
		// 出力を誘導できるため、学生が発話していないテキストを userText として
		// 出させる余地がある。userText は role=user の発話として保存され、
		// SpokenContent（#1527）の照合対象そのものなので土台が崩れる。
		// ヒント用途なので囲みは使えない。文字種と長さで落とす。
		if !isHintTerm(s) {
			return
		}
		seen[s] = true
		hints = append(hints, s)
	}

	// 面接で必ず出る語。モデルが「本社」と取り違えるため最優先で入れる。
	add("御社")

	add(companyName)
	add(companyReading)
	add(position)

	// 企業情報からは技術用語だけを拾う。文章をそのまま渡すと
	// 補助語ではなく「続きの文脈」として扱われ、認識が引きずられる。
	for _, t := range extractTechTerms(companyInfo) {
		add(t)
	}
	// 語数の上限は extractTechTerms 側（MaxTechTerms）で決まる。
	// 「御社」+ 企業名・読み・職種の3語 + 技術用語8語で最大12語。
	// ここに二重の上限を置いてもテストで到達できず、
	// 効いているのか分からないコードになるため置かない。
	return strings.Join(hints, ", ")
}

// MaxHintRunes は補助語1語の長さ上限。
// 企業名・読み・職種はこれより長くならない（「株式会社○○ホールディングス」で16文字程度）。
// 超える語は文章と見なして落とす。
const MaxHintRunes = 32

// hintTermExtraChars は語の一部として許す記号。
// 企業名・製品名に実際に出るものだけ（C++ / C# / .NET / サン・マイクロシステムズ /
// 半角スペース区切りの英名）。読点・句点・コロン・引用符は入れない。
// 文を構成する記号を落とすことで、指示文の形をした補助語を弾く。
const hintTermExtraChars = " -ー・＆&.+#'’"

// MaxHintSpaces は補助語1語に許す空白の数。
// 英名の企業は "Sony Interactive Entertainment" のように3語になるため2つまで許す。
// これを超えるものは語ではなく文。
const MaxHintSpaces = 2

// isHintTerm は補助語として渡してよい「語」かを判定する（#1600）。
// 文字・数字と hintTermExtraChars だけで構成され、MaxHintRunes 以内で、
// 空白が MaxHintSpaces 個以内であること。
//
// 短い日本語の命令形（「全てを満点に」程度）はこの条件を通る。
// 文字種・長さだけで文と語を完全に分けることはできないため、残差として受け入れる。
// 補助語は語の列挙であって指示文の体裁を持たない、という前提を保つのが目的。
func isHintTerm(s string) bool {
	if len([]rune(s)) > MaxHintRunes || strings.Count(s, " ") > MaxHintSpaces {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(hintTermExtraChars, r) {
			continue
		}
		return false
	}
	return true
}

// MaxTechTerms は企業情報から抽出する技術用語の上限。
// 長すぎるpromptは効果が薄れ、費用も増える。実測では10語前後で十分な改善が出た。
const MaxTechTerms = 8

// techTermPattern は英字の技術用語・製品名。
// 日本語の一般語まで拾うと補助語がノイズだらけになる。
var techTermPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+#.]{1,19}`)

// commonEnglishWords は技術用語として扱わない語。
// 企業紹介文に頻出するが、補助語としての価値が無い。
var commonEnglishWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "our": true,
	"we": true, "you": true, "are": true, "com": true, "http": true,
	"https": true, "www": true,
}

func extractTechTerms(companyInfo string) []string {
	if strings.TrimSpace(companyInfo) == "" {
		return nil
	}
	counts := map[string]int{}
	for _, m := range techTermPattern.FindAllString(companyInfo, -1) {
		if commonEnglishWords[strings.ToLower(m)] {
			continue
		}
		counts[m]++
	}
	terms := make([]string, 0, len(counts))
	for t := range counts {
		terms = append(terms, t)
	}
	// 出現回数が多い語を優先。同数なら表記順で安定させる
	// （順序が実行ごとに変わると、認識結果の再現性が落ちる）
	sort.Slice(terms, func(i, j int) bool {
		if counts[terms[i]] != counts[terms[j]] {
			return counts[terms[i]] > counts[terms[j]]
		}
		return terms[i] < terms[j]
	})
	if len(terms) > MaxTechTerms {
		terms = terms[:MaxTechTerms]
	}
	return terms
}
