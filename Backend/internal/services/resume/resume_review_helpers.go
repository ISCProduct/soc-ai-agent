package resume

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/shared/textsim"
)

const (
	// quoteMatchThreshold は引用と本文ブロックを同一と見なす最小類似度。
	// 語尾・送り仮名の違いや一節の省略でもDice係数は0.6前後を保つ一方、
	// 無関係な日本語文どうしのbigram重複は0.3程度に収まるため、その中間に置いている。
	quoteMatchThreshold = 0.55
	// quoteMatchMargin は最良候補と次点の類似度差の下限。
	// これ未満なら「どちらのブロックとも言える」状態なので紐づけを諦める。
	// 誤ったブロックに紐づくと注釈PDFに焼かれ、学生が無関係な記述を直すことになるため、
	// 迷うくらいなら欠落させる。
	quoteMatchMargin = 0.1
)

func fallbackResumeReview(blocks []models.ResumeTextBlock) (*models.ResumeReview, []models.ResumeReviewItem) {
	score := 70
	summary := "主要セクションを確認しました。具体性の強化が改善ポイントです。"
	items := make([]models.ResumeReviewItem, 0)
	bbox, _ := json.Marshal([]float64{20, 20, 260, 80})
	items = append(items, models.ResumeReviewItem{
		PageNumber: 1,
		BBox:       string(bbox),
		Severity:   "info",
		Message:    "内容は整理されていますが、成果の具体性や背景の説明が不足しがちです。",
		Suggestion: "成果を数値で示し、役割や工夫点・課題を一文ずつ補足してください。",
	})
	return &models.ResumeReview{
		Score:   score,
		Summary: summary,
	}, items
}

func fallbackResumeReviewDetailed(blocks []models.ResumeTextBlock) (*models.ResumeReview, []models.ResumeReviewItem) {
	score := 70
	summary := "内容を確認しました。各項目の具体性を高めると説得力が増します。"
	items := buildHeuristicItems(blocks, 8)
	if len(items) == 0 {
		return fallbackResumeReview(blocks)
	}
	return &models.ResumeReview{
		Score:   score,
		Summary: summary,
	}, items
}

func buildResumeText(blocks []models.ResumeTextBlock, maxLen int) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, block := range blocks {
		line := strings.TrimSpace(block.Text)
		if line == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[P%dB%d] %s\n", block.PageNumber, block.BlockIndex, line))
		if b.Len() >= maxLen {
			break
		}
	}
	return b.String()
}

func decodeJSON(raw string, out any) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("empty response")
	}
	if err := json.Unmarshal([]byte(raw), out); err == nil {
		return nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return json.Unmarshal([]byte(raw[start:end+1]), out)
	}
	return errors.New("invalid JSON response")
}

func mapReviewItems(blocks []models.ResumeTextBlock, aiItems []aiReviewItem) []models.ResumeReviewItem {
	if len(aiItems) == 0 {
		return nil
	}
	// ブロック側の正規化とbigramは指摘ごとに作り直さず、ここで一度だけ計算して使い回す。
	blockTexts := make([]textsim.Text, len(blocks))
	for i := range blocks {
		blockTexts[i] = textsim.New(blocks[i].Text)
	}

	result := make([]models.ResumeReviewItem, 0, len(aiItems))
	for _, item := range aiItems {
		if strings.TrimSpace(item.Quote) == "" {
			continue
		}
		var block *models.ResumeTextBlock
		foundByIndex := false
		if item.PageHint > 0 && item.BlockIndex > 0 {
			block = findBlockByIndex(blocks, item.PageHint, item.BlockIndex)
			if block != nil {
				foundByIndex = true
			}
		}
		if block == nil && runeLen(item.Quote) >= 6 {
			block = findBestBlock(blocks, blockTexts, item.Quote, item.PageHint)
		}
		if block == nil {
			continue
		}
		if !foundByIndex && !quoteInBlock(item.Quote, block.Text) {
			continue
		}
		severity := strings.ToLower(item.Severity)
		if severity == "" {
			severity = "info"
		}
		result = append(result, models.ResumeReviewItem{
			PageNumber: block.PageNumber,
			BBox:       block.BBox,
			Severity:   severity,
			Message:    item.Message,
			Suggestion: item.Suggestion,
		})
	}
	return result
}

// findBestBlock は引用に最も近い本文ブロックを返す。
// blockTexts は blocks と同じ並びの正規化済みテキスト。
// しきい値未満、または次点と差が小さく曖昧な場合は nil を返す。
func findBestBlock(blocks []models.ResumeTextBlock, blockTexts []textsim.Text, quote string, pageHint int) *models.ResumeTextBlock {
	quoteText := textsim.New(quote)
	if quoteText.Empty() {
		return nil
	}
	// page_hint のページ内を優先し、見つからなければ全ページから探す。
	if best := bestMatchingBlock(blocks, blockTexts, quoteText, pageHint); best != nil {
		return best
	}
	if pageHint > 0 {
		return bestMatchingBlock(blocks, blockTexts, quoteText, 0)
	}
	return nil
}

func bestMatchingBlock(blocks []models.ResumeTextBlock, blockTexts []textsim.Text, quoteText textsim.Text, pageHint int) *models.ResumeTextBlock {
	var best *models.ResumeTextBlock
	bestScore, secondScore := 0.0, 0.0
	bestLen := 0
	for i := range blocks {
		if pageHint > 0 && blocks[i].PageNumber != pageHint {
			continue
		}
		score := blockTexts[i].Score(quoteText)
		length := runeLen(blockTexts[i].Norm())
		if score > bestScore {
			secondScore = bestScore
			bestScore, bestLen, best = score, length, &blocks[i]
			continue
		}
		if score > secondScore {
			secondScore = score
		}
		// 完全包含(1.0)が複数あるのは一文が複数行のブロックに分かれているケース。
		// 引用と最も長く重なるブロックを採る。
		if score == bestScore && bestScore == 1 && length > bestLen {
			bestLen, best = length, &blocks[i]
		}
	}
	if best == nil || bestScore < quoteMatchThreshold {
		return nil
	}
	// 完全包含はそのまま抜粋されたケースなので曖昧扱いしない。
	// それ以外で次点と差が小さいときは、誤ったブロックに紐づけるより欠落させる。
	if bestScore < 1 && bestScore-secondScore < quoteMatchMargin {
		return nil
	}
	return best
}

func findBlockByIndex(blocks []models.ResumeTextBlock, pageHint int, blockIndex int) *models.ResumeTextBlock {
	for i := range blocks {
		block := &blocks[i]
		if block.PageNumber == pageHint && block.BlockIndex == blockIndex {
			return block
		}
	}
	return nil
}

// quoteInBlock は引用が本文ブロックに一致すると見なせるかを判定する。
// 正規化後の部分一致、または類似度がしきい値以上なら一致とする。
func quoteInBlock(quote string, blockText string) bool {
	return textsim.Similarity(blockText, quote) >= quoteMatchThreshold
}

func runeLen(s string) int {
	return len([]rune(strings.TrimSpace(s)))
}

func buildHeuristicItems(blocks []models.ResumeTextBlock, max int) []models.ResumeReviewItem {
	if len(blocks) == 0 || max <= 0 {
		return nil
	}
	result := make([]models.ResumeReviewItem, 0, max)
	seen := make(map[string]bool)
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if runeLen(text) < 12 {
			continue
		}
		label, message, suggestion := classifyBlock(text)
		if label == "" {
			continue
		}
		key := fmt.Sprintf("%d-%d-%s", block.PageNumber, block.BlockIndex, label)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, models.ResumeReviewItem{
			PageNumber: block.PageNumber,
			BBox:       block.BBox,
			Severity:   "info",
			Message:    message,
			Suggestion: suggestion,
		})
		if len(result) >= max {
			return result
		}
	}
	if len(result) == 0 {
		for _, block := range blocks {
			text := strings.TrimSpace(block.Text)
			if runeLen(text) < 16 {
				continue
			}
			result = append(result, models.ResumeReviewItem{
				PageNumber: block.PageNumber,
				BBox:       block.BBox,
				Severity:   "info",
				Message:    "この記述は成果や役割の具体性が読み取りづらいです。",
				Suggestion: "成果の数値や担当範囲、工夫点を一文ずつ補足してください。",
			})
			if len(result) >= max {
				return result
			}
		}
	}
	return result
}

func classifyBlock(text string) (string, string, string) {
	switch {
	case strings.Contains(text, "志望") || strings.Contains(text, "動機"):
		return "motivation",
			"志望動機の根拠が抽象的に見えます。",
			"企業の事業や職種と自分の経験の接点を1文で明示し、具体的な業務貢献を追記してください。"
	case strings.Contains(text, "自己PR") || strings.Contains(text, "自己ＰＲ"):
		return "pr",
			"自己PRが強みの列挙にとどまっています。",
			"成果の数値、工夫した点、再現性が分かる行動を1文ずつ追加してください。"
	case strings.Contains(text, "学歴"):
		return "", "", ""
	case strings.Contains(text, "職歴"):
		return "", "", ""
	case strings.Contains(text, "資格") || strings.Contains(text, "免許"):
		return "license",
			"資格が応募職種にどう活かせるかが伝わりづらいです。",
			"資格で得たスキルと職務での活用例を一文追加してください。"
	case strings.Contains(text, "得意") || strings.Contains(text, "特技") || strings.Contains(text, "スキル"):
		return "skill",
			"スキルの記載が抽象的で実務イメージが湧きにくいです。",
			"使用期間、具体的な成果物、担当範囲を補足してください。"
	case strings.Contains(text, "学生時代"):
		return "student",
			"活動の規模や成果が読み取りづらいです。",
			"人数・期間・結果などの具体的な数値を補足してください。"
	}
	return "generic",
		"この記述は成果や役割の具体性が読み取りづらいです。",
		"成果の数値、役割、工夫点を一文ずつ補足してください。"
}

func selectReviewBlocks(blocks []models.ResumeTextBlock, max int) []models.ResumeTextBlock {
	if len(blocks) == 0 || max <= 0 {
		return nil
	}
	selected := make([]models.ResumeTextBlock, 0, max)
	seenPages := make(map[int]bool)
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if runeLen(text) < 12 {
			continue
		}
		if strings.HasSuffix(text, "：") || strings.HasSuffix(text, ":") {
			continue
		}
		selected = append(selected, block)
		seenPages[block.PageNumber] = true
		if len(selected) >= max {
			return selected
		}
	}
	if len(selected) < max {
		for _, block := range blocks {
			if seenPages[block.PageNumber] {
				continue
			}
			text := strings.TrimSpace(block.Text)
			if runeLen(text) < 8 {
				continue
			}
			selected = append(selected, block)
			if len(selected) >= max {
				break
			}
		}
	}
	return selected
}

func buildBlockList(blocks []models.ResumeTextBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, block := range blocks {
		line := strings.TrimSpace(block.Text)
		if line == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[P%dB%d] %s\n", block.PageNumber, block.BlockIndex, line))
	}
	return b.String()
}
