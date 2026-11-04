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
	// quoteMatchMargin は最良候補と、それと重ならない次点候補の類似度差の下限。
	// これ未満なら「本文のどちらの箇所とも言える」状態なので紐づけを諦める。
	// 誤ったブロックに紐づくと注釈PDFに焼かれ、学生が無関係な記述を直すことになるため、
	// 迷うくらいなら欠落させる。
	//
	// 連結窓(#1559)を入れる前は 0.025 だった。一文が2行に分かれた引用を1行ブロックと
	// 突き合わせていたため、正解と不正解の差が 0.038 しかなく大きくできなかった。
	// 連結窓なら一文全体が1つの候補になり、2〜4行に分かれた引用は窓が満点（実測 1.00、
	// 次点との差 0.92〜1.00）を取るので 0.1 に戻せる。
	// 語尾だけ違う近似ブロックの差 0.036（実測）はこれで捨てられる。
	// 使える帯は 0.0989〜0.1278（実測）で、TestQuoteMatchMarginBoundary が上下から固定している。
	quoteMatchMargin = 0.1
	// quoteMinRunes は照合に使う引用の最小文字数（正規化後）。
	// Normalize は約物・記号を落とすので、生8文字の「「、。・（）粘」」は「粘」1文字まで縮む。
	// こうした引用が1ブロックにだけ含まれるのは偶然の一致であり、
	// 1〜2文字の漢字で注釈PDFにピンが打たれてしまうため捨てる。
	quoteMinRunes = 6
	// quoteMaxWindowBlocks は引用と突き合わせる連結窓の最大行数（#1559）。
	//
	// OCRブロックは行単位なのに、プロンプトが要求する引用は一文なので、
	// 履歴書・ESの自己PRや志望動機では1文が2〜4行に折り返すのが普通である。
	// Dice係数が quoteMatchThreshold を超えるには1行が引用の約38%以上を占める必要があり、
	// 3等分(33%)では原理的に届かない。そこで連続する行を連結した窓も候補にする。
	//
	// 4行までにしているのは、履歴書の記入欄が数行で折り返す前提と、
	// 窓を広げるほど無関係な行を巻き込んだ窓が増えて誤紐づけの危険が上がるため。
	quoteMaxWindowBlocks = 4

	// OCRRewriteMinConfidence は書き換え案を出してよい認識スコアの下限（0〜1）。
	//
	// PaddleOCR の行スコアは 0〜1 で、活字の履歴書は概ね 0.9 前後に集まる。
	// 0.6 未満は欠落や別字が混ざる帯なので、その行の文字を根拠にした
	// 置き換え文は出さない（#1942）。信頼度が NULL の行（#1942 以前、平文入力）は
	// 不明であり、低いとは扱わない。
	//
	// 0.599 は捨て、0.6 は残す。定数をこの外へ動かすと
	// TestOCRRewriteMinConfidenceBoundary が落ちる。
	OCRRewriteMinConfidence = 0.6

	// lowConfidenceMark はプロンプト上で「この行を書き換えない」と示す印。
	// 本文の一部ではない。モデルが quote に写しても、照合と保存の前に除く。
	lowConfidenceMark = "[低信頼]"

	// rewriteWithheldLowOCR は書き換えを空にした理由。画面と注釈PDFが同じ値を見る。
	rewriteWithheldLowOCR = "low_ocr_confidence"
)

// fallbackResumeReview / fallbackResumeReviewDetailed は削除した（#1529）。
//
// どちらも本番コードから一度も呼ばれておらず、固定値70のスコアを返していた。
// 「70点」が良かったから70なのか失敗したから70なのか区別できない元凶で、
// 復活させると採点基準の無いスコアがマッチングへ流れる。
// LLM が使えないときはスコアを書かない（docs/wiki/scoring.md §2-3）。
// ヒューリスティックな指摘生成（buildHeuristicItems / classifyBlock）も
// 呼び出し元が無くなったため併せて削除した。

// blockConfidenceTooLow は、認識スコアが下限未満で書き換え案を出してはいけない行かを返す。
// スコアが無い行は不明なので false（書き換えを禁止しない）。
func blockConfidenceTooLow(confidence *float64) bool {
	return confidence != nil && *confidence < OCRRewriteMinConfidence
}

// spanConfidenceTooLow は、照合に使った窓のどれか1行でも下限未満なら true。
// 代表の1行だけを見ると、隣の誤読行を含む引用に書き換えが残る。
func spanConfidenceTooLow(blocks []models.ResumeTextBlock, start, size int) bool {
	if size <= 0 || start < 0 || start+size > len(blocks) {
		return false
	}
	for i := start; i < start+size; i++ {
		if blockConfidenceTooLow(blocks[i].Confidence) {
			return true
		}
	}
	return false
}

// stripLowConfidenceMark はプロンプト用の印を引用から除く。
// 画面と注釈PDFの「原文」に印が出ないように、保存する文字列へは残さない。
func stripLowConfidenceMark(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, lowConfidenceMark, ""))
}

// sameRewriteAsQuote は、空白の差を除いて書き換えが原文の写しかを返す。
func sameRewriteAsQuote(rewrite, quote string) bool {
	return stripSpaces(rewrite) != "" && stripSpaces(rewrite) == stripSpaces(quote)
}

func stripSpaces(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '　':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// normalizeOCRScore は 0〜1 の認識スコアだけを残す。範囲外は不明（nil）にする。
// 0 は有効な「読めなかった」なので nil にしない。
func normalizeOCRScore(score float64) *float64 {
	if score < 0 || score > 1 {
		return nil
	}
	v := score
	return &v
}

func buildResumeText(blocks []models.ResumeTextBlock, maxLen int) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, block := range blocks {
		line := formatReviewBlockLine(block)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if b.Len() >= maxLen {
			break
		}
	}
	return b.String()
}

// formatReviewBlockLine はプロンプトへ載せる1行を作る。
// 下限未満の行には lowConfidenceMark を付け、モデルへ書き換えを出させない。
func formatReviewBlockLine(block models.ResumeTextBlock) string {
	line := strings.TrimSpace(block.Text)
	if line == "" {
		return ""
	}
	if blockConfidenceTooLow(block.Confidence) {
		return fmt.Sprintf("[P%dB%d] %s %s", block.PageNumber, block.BlockIndex, lowConfidenceMark, line)
	}
	return fmt.Sprintf("[P%dB%d] %s", block.PageNumber, block.BlockIndex, line)
}

// reviewRewriteInstruction は初回・やり直しの両方に載せる書き換えの契約。
func reviewRewriteInstruction() string {
	return fmt.Sprintf(`rewriteはquoteをそのまま置き換えられる完成した一文にしてください。説明文や「〜するとよい」はrewriteに書かずsuggestionに書いてください。
行頭に %s が付く行はOCRの誤認識の可能性があるため、その行をquoteに選んでもrewriteは空文字にしてください。誤読した文字を直した文を出してはいけません。`, lowConfidenceMark)
}

// reviewItemJSONExample は出力例。初回とやり直しで欄が食い違うと、片方だけ rewrite が欠ける。
const reviewItemJSONExample = `{"quote":"本文中の一文","message":"指摘","suggestion":"直し方の説明","rewrite":"置き換え後の一文","severity":"info|warning|critical","page_hint":1,"block_index":1}`

// decodeJSON はレビューJSONを読む。
//
// 本文は JSON mode（#1583）で素の JSON が返るため、通常は最初の Unmarshal で通る。
// '{' 〜 '}' の切り出しは、JSON mode が使えないモデルへ差し替えたときの保険として
// 残している。前置きに '{' が含まれると誤った範囲を切り出すため、この経路に
// 依存してはいけない。
func decodeJSON(raw string, out any) error {
	_, err := decodeJSONRecovered(raw, out)
	return err
}

// decodeJSONRecovered は decodeJSON と同じ処理をしつつ、'{' 〜 '}' の切り出し
// （復旧処理）を通ったかどうかも返す。
//
// 戻り値を分けているのは、「JSON mode を入れたら復旧処理を通らなくなる」ことを
// 外から観測できるようにするため（TestDecodeJSON_復旧経路）。テスト側で
// 分岐条件を書き写すと、実装をどう変えても落ちない恒真テストになる。
func decodeJSONRecovered(raw string, out any) (recovered bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, errors.New("empty response")
	}
	if err := json.Unmarshal([]byte(raw), out); err == nil {
		return false, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return true, json.Unmarshal([]byte(raw[start:end+1]), out)
	}
	return false, errors.New("invalid JSON response")
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
	windows := buildBlockWindows(blocks, blockTexts)

	result := make([]models.ResumeReviewItem, 0, len(aiItems))
	for _, item := range aiItems {
		// 長さは正規化後で判定する。生の文字数で見ると約物だけの引用が通ってしまう。
		// page_hint 経路も同じゲートを通すため、ここで1回だけ判定する。
		rawQuote := stripLowConfidenceMark(item.Quote)
		quoteText := textsim.New(rawQuote)
		if quoteText.Len() < quoteMinRunes {
			continue
		}

		var block *models.ResumeTextBlock
		spanStart, spanSize := 0, 0
		// page_hint / block_index はLLM生成で幻覚するので、指しているブロックが
		// 実際に引用と一致するかを裏取りし、外れていたら通常の探索に落とす。
		// block_index は ocr_extract.py が 0 始まりで振るため 0 も受ける。
		//
		// 裏取りは1行のままにしている（連結窓では見ない）。複数行に分かれた引用では
		// この裏取りは必ず失敗して findBestBlock に落ちるが、そちらが窓で解決する。
		// 「hint が指す行を含む窓」で裏取りすると、隣が本文でありさえすれば
		// 表ヘッダ行を指す hint まで通ってしまい、無関係な行に注釈が焼かれる。
		if item.PageHint > 0 {
			if i := findBlockByIndex(blocks, item.PageHint, item.BlockIndex); i >= 0 &&
				blockTexts[i].MatchScore(quoteText) >= quoteMatchThreshold {
				block = &blocks[i]
				spanStart, spanSize = i, 1
			}
		}
		// findBestBlock が quoteMatchThreshold を保証して返すため、追加の照合はしない。
		if block == nil {
			block, spanStart, spanSize = findBestBlock(blocks, blockTexts, windows, quoteText)
		}
		if block == nil {
			continue
		}
		severity := strings.ToLower(item.Severity)
		if severity == "" {
			severity = "info"
		}
		quote := rawQuote
		if quote == "" {
			quote = strings.TrimSpace(block.Text)
		}
		rewrite := strings.TrimSpace(item.Rewrite)
		withheld := ""
		// モデルが誤認識行へ書き換えを出しても、ここで捨てる。
		// プロンプトの印は守られないことがあるため、保存前に必ず見る。
		// 窓の代表行だけを見ると、隣の誤読行を含む引用に書き換えが残る。
		if spanConfidenceTooLow(blocks, spanStart, spanSize) {
			rewrite = ""
			withheld = rewriteWithheldLowOCR
		} else if sameRewriteAsQuote(rewrite, quote) {
			rewrite = ""
		}
		result = append(result, models.ResumeReviewItem{
			PageNumber:      block.PageNumber,
			BBox:            block.BBox,
			Severity:        severity,
			Message:         item.Message,
			Suggestion:      item.Suggestion,
			Quote:           quote,
			Rewrite:         rewrite,
			RewriteWithheld: withheld,
		})
	}
	return result
}

// blockWindow は連続する本文ブロックを連結した照合窓（#1559）。
// blocks[start:start+size] を指す。
type blockWindow struct {
	start int
	size  int
	text  textsim.Bigrams
}

// buildBlockWindows は照合に使う窓を行数の小さい順に組み立てる。
// 1行の窓は blockTexts をそのまま使い回す（同じテキストを二度正規化しない）。
//
// 指摘ごとに作り直さないよう mapReviewItems で一度だけ呼ぶこと。
// 窓の数は最大 len(blocks)×quoteMaxWindowBlocks に収まる。
func buildBlockWindows(blocks []models.ResumeTextBlock, blockTexts []textsim.Bigrams) []blockWindow {
	windows := make([]blockWindow, 0, len(blocks)*quoteMaxWindowBlocks)
	for i := range blocks {
		windows = append(windows, blockWindow{start: i, size: 1, text: blockTexts[i]})
	}
	var b strings.Builder
	for size := 2; size <= quoteMaxWindowBlocks; size++ {
		for start := 0; start+size <= len(blocks); start++ {
			// ページをまたぐ一文は無いので連結しない。
			// 無駄な窓が増えるだけでなく、別ページの行を巻き込んだ誤紐づけを招く。
			if blocks[start].PageNumber != blocks[start+size-1].PageNumber {
				continue
			}
			b.Reset()
			for _, block := range blocks[start : start+size] {
				b.WriteString(block.Text)
			}
			windows = append(windows, blockWindow{start: start, size: size, text: textsim.New(b.String())})
		}
	}
	return windows
}

// overlaps は2つの窓が同じ行を含むかを返す。
func (w blockWindow) overlaps(other blockWindow) bool {
	return w.start < other.start+other.size && other.start < w.start+w.size
}

// findBestBlock は引用に最も近い本文ブロックと、照合に使った窓の範囲を返す。
// blockTexts は blocks と同じ並びの正規化済みテキスト、windows は buildBlockWindows の結果。
// しきい値未満、または次点と差が小さく曖昧な場合は nil と空の範囲を返す。
// 範囲は書き換えを出してよいかを窓全体で見るために使う。代表の1行だけでは誤読行を見逃す。
//
// 候補は1行だけでなく連続行を連結した窓も見る（#1559）。OCRブロックは行単位、
// 引用は一文なので、一文が複数行に折り返していると1行との Dice 係数が原理的に
// しきい値へ届かない。
//
// page_hint での絞り込みはしない。page_hint はLLM生成で幻覚するため、
// 絞り込むと他ページの完全一致を見ずに同ページの弱い候補を採ってしまう。
func findBestBlock(blocks []models.ResumeTextBlock, blockTexts []textsim.Bigrams, windows []blockWindow, quoteText textsim.Bigrams) (*models.ResumeTextBlock, int, int) {
	scores := make([]float64, len(windows))
	best := -1
	bestScore := 0.0
	for i, w := range windows {
		scores[i] = w.text.MatchScore(quoteText)
		// 同点なら先に見た窓（＝行数の少ない窓）を残す。3行に分かれた引用は
		// それを含む4行の窓でも満点になるので、狭い方が引用の範囲を正しく表す。
		if scores[i] > bestScore {
			bestScore, best = scores[i], i
		}
	}
	if best < 0 || bestScore < quoteMatchThreshold {
		return nil, 0, 0
	}
	// 次点は「最良の窓と1行も重ならない窓」から採る。重なる窓は本文の同じ箇所を
	// 指しており、競合ではなく同じ紐づけ先の言い換えにすぎない。
	// （3行の引用なら、その一部を含む2行窓も必ず高得点になる）
	second := 0.0
	for i, w := range windows {
		if w.overlaps(windows[best]) {
			continue
		}
		second = max(second, scores[i])
	}
	// 満点の箇所が1つだけならそれが引用元。行が長いほど1語違いの行のスコアは
	// 1.0に漸近する(≒1-2/N)ので、marginで捨てると長文行が必ず落ちてしまう。
	// 満点はほぼ「本文が引用を含む」ケースだが、bigram集合が一致しても1.0になり得る
	// （"あいあ"と"いあい"）ため、包含とは言わず満点として扱う。
	if bestScore == 1 && second < 1 {
		w := windows[best]
		return pickRepresentative(blocks, blockTexts, w, quoteText), w.start, w.size
	}
	// 次点と差が小さいときは、本文のどちらの箇所とも言える状態。
	// 誤ったブロックに紐づけるより欠落させる（同じ言い回しが2箇所にある場合など）。
	if bestScore-second < quoteMatchMargin {
		return nil, 0, 0
	}
	w := windows[best]
	return pickRepresentative(blocks, blockTexts, w, quoteText), w.start, w.size
}

// pickRepresentative は窓の中で引用と最も長く重なる行を返す（同点なら上の行）。
//
// 注釈PDFのbboxは行単位なので、複数行の窓に当たっても1行を代表に選ぶ。
// 窓のbbox和集合にしない理由は、段組みの履歴書では左右の欄の行が連続して
// 並ぶことがあり、和集合が無関係な記述まで塗る巨大な矩形になるため。
// 1行に絞れば注釈は必ず引用の一部に重なる（annotate_pdf.py 側の変更も要らない）。
func pickRepresentative(blocks []models.ResumeTextBlock, blockTexts []textsim.Bigrams, w blockWindow, quoteText textsim.Bigrams) *models.ResumeTextBlock {
	best := w.start
	bestScore := -1.0
	for i := w.start; i < w.start+w.size; i++ {
		if score := blockTexts[i].MatchScore(quoteText); score > bestScore {
			bestScore, best = score, i
		}
	}
	return &blocks[best]
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
		line := formatReviewBlockLine(block)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
