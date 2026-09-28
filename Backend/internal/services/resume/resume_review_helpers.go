package resume

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/shared/textsim"
)

// 引用とブロックの照合しきい値。値は resume_review_match_test.go の
// TestQuoteMatchThresholdBoundary / TestQuoteMatchMarginBoundary が
// 実測スコアの境界ケースで上下から固定している（動かすとどちらかが落ちる）。
const (
	// quoteMatchThreshold は引用と本文ブロックを同一と見なす最小類似度。
	// 語尾・送り仮名の違いや一節の省略は 0.55〜0.95（実測）に収まり、
	// 助詞や語順まで変わった言い換えは 0.51 以下に落ちるため、その境目に置いている。
	quoteMatchThreshold = 0.55
	// quoteMatchMargin は最良候補と次点の類似度差の下限。
	// これ未満なら「どちらのブロックとも言える」状態なので紐づけを諦める。
	// 誤ったブロックに紐づくと注釈PDFに焼かれ、学生が無関係な記述を直すことになるため、
	// 迷うくらいなら欠落させる。
	//
	// 言い回しが1語だけ違うブロックどうしの差は 0.015（実測）。一方、一文が2行に
	// ほぼ均等に分かれたときの正解と不正解の差は 0.038〜0.063（実測）しかないので、
	// 大きくすると均等分割の引用がまるごと欠落する。その間の値。
	//
	// 振る舞いで固定できている帯は 0.016〜0.038。0.022 以下にすると
	// TestQuoteMatchUniqueExactWins の前提チェックが鳴るため、実際に通るのは
	// 0.023〜0.038。帯がこれだけ狭いのは一文が2行に分かれた引用を1行ブロックと
	// 突き合わせているからで、隣接行の連結窓(Issue #1559)が入れば 0.1 に戻せる。
	quoteMatchMargin = 0.025
	// quoteMinRunes は照合に使う引用の最小文字数（正規化後）。
	// Normalize は約物・記号を落とすので、生8文字の「「、。・（）粘」」は「粘」1文字まで縮む。
	// こうした引用が1ブロックにだけ含まれるのは偶然の一致であり、
	// 1〜2文字の漢字で注釈PDFにピンが打たれてしまうため捨てる。
	quoteMinRunes = 6
)

// fallbackResumeReview / fallbackResumeReviewDetailed は削除した（#1529）。
//
// どちらも本番コードから一度も呼ばれておらず、固定値70のスコアを返していた。
// 「70点」が良かったから70なのか失敗したから70なのか区別できない元凶で、
// 復活させると採点基準の無いスコアがマッチングへ流れる。
// LLM が使えないときはスコアを書かない（docs/wiki/scoring.md §2-3）。
// ヒューリスティックな指摘生成（buildHeuristicItems / classifyBlock）も
// 呼び出し元が無くなったため併せて削除した。

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
	blockTexts := make([]textsim.Bigrams, len(blocks))
	for i := range blocks {
		blockTexts[i] = textsim.New(blocks[i].Text)
	}

	result := make([]models.ResumeReviewItem, 0, len(aiItems))
	for _, item := range aiItems {
		// 長さは正規化後で判定する。生の文字数で見ると約物だけの引用が通ってしまう。
		// page_hint 経路も同じゲートを通すため、ここで1回だけ判定する。
		quoteText := textsim.New(item.Quote)
		if quoteText.Len() < quoteMinRunes {
			continue
		}

		var block *models.ResumeTextBlock
		// page_hint / block_index はLLM生成で幻覚するので、指しているブロックが
		// 実際に引用と一致するかを裏取りし、外れていたら通常の探索に落とす。
		// block_index は ocr_extract.py が 0 始まりで振るため 0 も受ける。
		if item.PageHint > 0 {
			if i := findBlockByIndex(blocks, item.PageHint, item.BlockIndex); i >= 0 &&
				blockTexts[i].MatchScore(quoteText) >= quoteMatchThreshold {
				block = &blocks[i]
			}
		}
		// findBestBlock が quoteMatchThreshold を保証して返すため、追加の照合はしない。
		if block == nil {
			block = findBestBlock(blocks, blockTexts, quoteText)
		}
		if block == nil {
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
//
// page_hint での絞り込みはしない。page_hint はLLM生成で幻覚するため、
// 絞り込むと他ページの完全一致を見ずに同ページの弱い候補を採ってしまう。
func findBestBlock(blocks []models.ResumeTextBlock, blockTexts []textsim.Bigrams, quoteText textsim.Bigrams) *models.ResumeTextBlock {
	var best *models.ResumeTextBlock
	bestScore, secondScore := 0.0, 0.0
	fullScoreCount := 0
	for i := range blocks {
		score := blockTexts[i].MatchScore(quoteText)
		if score == 1 {
			fullScoreCount++
		}
		if score > bestScore {
			bestScore, secondScore, best = score, bestScore, &blocks[i]
			continue
		}
		if score > secondScore {
			secondScore = score
		}
	}
	if best == nil || bestScore < quoteMatchThreshold {
		return nil
	}
	// 満点が1件だけならそれが引用元。行が長いほど1語違いの行のスコアは
	// 1.0に漸近する(≒1-2/N)ので、marginで捨てると長文行が必ず落ちてしまう。
	// 満点はほぼ「本文が引用を含む」ケースだが、bigram集合が一致しても1.0になり得る
	// （"あいあ"と"いあい"）ため、包含とは言わず満点として数える。
	if bestScore == 1 && fullScoreCount == 1 {
		return best
	}
	// 次点と差が小さいときは、どちらのブロックとも言える状態。
	// 誤ったブロックに紐づけるより欠落させる（同じ言い回しが2箇所にある場合など）。
	if bestScore-secondScore < quoteMatchMargin {
		return nil
	}
	return best
}

// adoptRetryItems は紐づけリトライの結果を採用するかを決める。
// 件数が増えたときだけ採用する。初回より少ない結果で上書きすると、
// 初回2件・リトライ0件のようなケースで紐づけ失敗エラーになってしまう。
//
// 結合ではなく置き換えにしているのは、リトライが selectReviewBlocks で選び直した
// 別のブロック集合に指摘を再依頼するもので初回と内容が重複し得るのに、
// (PageNumber, BBox) での重複排除を持たないため。同じ箇所への二重注釈を避ける。
func adoptRetryItems(initial, retry []models.ResumeReviewItem) []models.ResumeReviewItem {
	if len(retry) > len(initial) {
		return retry
	}
	return initial
}

// findBlockByIndex は page_hint / block_index が指すブロックの添字を返す（無ければ -1）。
func findBlockByIndex(blocks []models.ResumeTextBlock, pageHint int, blockIndex int) int {
	for i := range blocks {
		if blocks[i].PageNumber == pageHint && blocks[i].BlockIndex == blockIndex {
			return i
		}
	}
	return -1
}

func runeLen(s string) int {
	return len([]rune(strings.TrimSpace(s)))
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
