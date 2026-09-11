package chat

import (
	"regexp"
	"strings"
)

// calculateSimilarity 2つの文字列の類似度を計算（簡易版）
func calculateSimilarity(s1, s2 string) float64 {
	// 正規化
	s1 = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s1, " ", ""), "　", ""))
	s2 = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s2, " ", ""), "　", ""))

	// 完全一致
	if s1 == s2 {
		return 1.0
	}

	// 一方が他方を含む場合
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 0.9
	}

	// 共通の単語数をカウント
	words1 := extractKeywords(s1)
	words2 := extractKeywords(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	commonCount := 0
	for w1 := range words1 {
		if words2[w1] {
			commonCount++
		}
	}

	// Jaccard係数
	totalWords := len(words1) + len(words2) - commonCount
	if totalWords == 0 {
		return 0.0
	}

	return float64(commonCount) / float64(totalWords)
}

// extractKeywords 文字列から重要なキーワードを抽出
func extractKeywords(s string) map[string]bool {
	// ストップワードを除外
	stopWords := map[string]bool{
		"あなた": true, "ます": true, "です": true, "ですか": true, "ください": true,
		"について": true, "として": true, "という": true, "どのよう": true,
		"何": true, "どう": true, "いつ": true, "どこ": true, "誰": true,
		"か": true, "の": true, "に": true, "を": true, "は": true, "が": true,
		"で": true, "と": true, "や": true, "から": true, "まで": true,
	}

	keywords := make(map[string]bool)

	// 3文字以上の単語を抽出（簡易版）
	runes := []rune(s)
	for i := 0; i < len(runes)-2; i++ {
		word := string(runes[i : i+3])
		if !stopWords[word] {
			keywords[word] = true
		}

		// 4文字以上も試す
		if i < len(runes)-3 {
			word4 := string(runes[i : i+4])
			if !stopWords[word4] {
				keywords[word4] = true
			}
		}
	}

	return keywords
}

// sanitizeForNewGrad 新卒向けに質問文を個人志向に書き換える
func sanitizeForNewGrad(q string) string {
	if strings.TrimSpace(q) == "" {
		return q
	}
	// 一般的な置換ルール（軽量）
	q = strings.ReplaceAll(q, "この会社", "あなた")
	q = strings.ReplaceAll(q, "会社で", "学ぶ場で")
	q = strings.ReplaceAll(q, "採用する", "学ぶ")
	q = strings.ReplaceAll(q, "採用しますか", "学びたいですか")
	q = strings.ReplaceAll(q, "導入", "学ぶこと")
	q = strings.ReplaceAll(q, "導入しますか", "学びますか")
	q = strings.ReplaceAll(q, "業務", "活動")
	q = strings.ReplaceAll(q, "プロジェクト", "グループワーク")
	q = strings.ReplaceAll(q, "クライアント", "相手")
	q = strings.ReplaceAll(q, "マネジメント", "まとめ役")
	q = strings.ReplaceAll(q, "KPI", "目標")
	q = strings.ReplaceAll(q, "売上", "成果")
	q = strings.ReplaceAll(q, "実績", "経験")
	q = strings.ReplaceAll(q, "現場", "活動の場")

	// パターン置換: 「新しい技術 .* 採用」-> 「新しい技術を学ぶことに興味がありますか」
	re := regexp.MustCompile(`(?i)新しい技術[\s\S]{0,30}採用`)
	if re.MatchString(q) {
		q = re.ReplaceAllString(q, "新しい技術を学ぶことに興味はありますか")
	}

	// 不自然な表現の微修正
	q = strings.ReplaceAll(q, "あなたは学ぶ", "あなたは学ぶことに興味がありますか")

	// 最後にトリム
	q = strings.TrimSpace(q)
	return q
}

func isVerboseQuestion(q string) bool {
	if strings.TrimSpace(q) == "" {
		return false
	}
	if len([]rune(q)) > 120 {
		return true
	}
	if strings.Contains(q, "（") || strings.Contains(q, "例：") || strings.Contains(q, "例:") || strings.Contains(q, "例えば") {
		return true
	}
	if strings.Count(q, "？")+strings.Count(q, "?") > 1 {
		return true
	}
	if strings.Count(q, "\n") > 1 {
		return true
	}
	return false
}

func simplifyNewGradQuestion(q string) string {
	s := strings.TrimSpace(q)
	if s == "" {
		return s
	}
	if idx := strings.Index(s, "（"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if idx := strings.Index(s, "例"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > 120 {
		s = string([]rune(s)[:120])
	}
	if !strings.HasSuffix(s, "？") && !strings.HasSuffix(s, "?") {
		s += "？"
	}
	return s
}
