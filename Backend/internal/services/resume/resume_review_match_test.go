package resume

// 引用(quote)と本文ブロックの照合テスト（Issue #1522 / #1559）
// 実行: cd Backend && go test ./internal/services/resume/... -run TestQuoteMatch -v
//
// quoteMatchThreshold / quoteMatchMargin の値は TestQuoteMatchThresholdBoundary と
// TestQuoteMatchMarginBoundary が上下から固定している。しきい値を動かすとどちらかが落ちる。

import (
	"fmt"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared/textsim"
)

// score は本文ブロックに対する引用の一致度を返す（境界値の記録用）。
func score(blockText, quote string) float64 {
	return textsim.New(blockText).MatchScore(textsim.New(quote))
}

// matchedLineOnly は連結窓を使わず1行ずつ照合したときに紐づくかを返す（#1559 以前の実装）。
// 現行のしきい値・margin・満点の扱いは findBestBlock と揃えてある。
func matchedLineOnly(blocks []models.ResumeTextBlock, quote string) bool {
	quoteText := textsim.New(quote)
	best, second := 0.0, 0.0
	for i := range blocks {
		s := textsim.New(blocks[i].Text).MatchScore(quoteText)
		if s > best {
			best, second = s, best
			continue
		}
		second = max(second, s)
	}
	if best < quoteMatchThreshold {
		return false
	}
	if best == 1 && second < 1 {
		return true
	}
	return best-second >= quoteMatchMargin
}

// windowScores は連結窓の照合結果（最良の窓と、それに重ならない次点）を返す。
// margin の境界ケースで実測値を記録するために使う。
func windowScores(blocks []models.ResumeTextBlock, quote string) (best, second float64) {
	blockTexts := make([]textsim.Bigrams, len(blocks))
	for i := range blocks {
		blockTexts[i] = textsim.New(blocks[i].Text)
	}
	windows := buildBlockWindows(blocks, blockTexts)
	quoteText := textsim.New(quote)
	var bestWindow blockWindow
	for _, w := range windows {
		if s := w.text.MatchScore(quoteText); s > best {
			best, bestWindow = s, w
		}
	}
	for _, w := range windows {
		if w.overlaps(bestWindow) {
			continue
		}
		second = max(second, w.text.MatchScore(quoteText))
	}
	return best, second
}

// makeBlocks はテスト用の本文ブロックを1ページ分組み立てる。
// BBox は "bbox-1" から始まる連番で、どのブロックに紐づいたかの目印に使う。
func makeBlocks(texts ...string) []models.ResumeTextBlock {
	blocks := make([]models.ResumeTextBlock, 0, len(texts))
	for i, text := range texts {
		blocks = append(blocks, models.ResumeTextBlock{
			PageNumber: 1,
			BlockIndex: i + 1,
			Text:       text,
			BBox:       fmt.Sprintf("bbox-%d", i+1),
		})
	}
	return blocks
}

// mapOne は指摘1件を紐づけ、当たったブロックのBBoxを返す（紐づかなければ空文字）。
func mapOne(t *testing.T, blocks []models.ResumeTextBlock, item aiReviewItem) string {
	t.Helper()
	got := mapReviewItems(blocks, []aiReviewItem{item})
	if len(got) == 0 {
		return ""
	}
	if len(got) > 1 {
		t.Fatalf("1件以下を期待したが %d 件だった", len(got))
	}
	return got[0].BBox
}

const (
	blockPR      = "アルバイトでは接客を通じて売上向上に貢献しました。"
	blockWeb     = "Webサイトの制作を担当しました。"
	blockMotive  = "貴社の地域密着型のサービスに魅力を感じ志望しました。"
	blockLicense = "資格は日商簿記2級を取得しています、実務でも活用したいです。"
)

func TestQuoteMatching(t *testing.T) {
	standard := makeBlocks(blockPR, blockWeb, blockMotive, blockLicense)
	// OCRは行単位で検出するため、履歴書には必ず短い表ヘッダ行が並ぶ
	tableHeaders := makeBlocks("年", "月", "日", "なし", "氏名", "平成30年4月 ○○県立△△高等学校 入学")
	// 同じ言い回しが2箇所にあるケース
	duplicated := makeBlocks(
		"接客のアルバイトで培った傾聴力を活かしたいです。",
		"接客のアルバイトで培った傾聴力を仕事で発揮します。",
	)

	tests := []struct {
		name     string
		blocks   []models.ResumeTextBlock
		item     aiReviewItem
		wantBBox string // 空文字なら紐づかないことを期待する
	}{
		{
			name:     "引用が本文と完全一致",
			blocks:   standard,
			item:     aiReviewItem{Quote: blockMotive},
			wantBBox: "bbox-3",
		},
		{
			name:     "引用が要約・省略されている",
			blocks:   standard,
			item:     aiReviewItem{Quote: "接客を通じて売上向上に貢献した"},
			wantBBox: "bbox-1",
		},
		{
			name:     "全角半角の違い",
			blocks:   standard,
			item:     aiReviewItem{Quote: "Ｗｅｂサイトの制作を担当しました。"},
			wantBBox: "bbox-2",
		},
		{
			name:     "引用に改行が入っている",
			blocks:   standard,
			item:     aiReviewItem{Quote: "貴社の地域密着型のサービスに\n魅力を感じ志望しました。"},
			wantBBox: "bbox-3",
		},
		{
			name:     "約物(読点)の違い",
			blocks:   standard,
			item:     aiReviewItem{Quote: "資格は日商簿記2級を取得しています実務でも活用したいです"},
			wantBBox: "bbox-4",
		},
		{
			name:     "全く無関係な引用は紐づけない",
			blocks:   standard,
			item:     aiReviewItem{Quote: "宇宙開発の研究室で衛星の姿勢制御を研究しました"},
			wantBBox: "",
		},
		{
			// 短いブロックは引用の部分文字列になるが、それで最高スコアにしてはならない
			name:     "短い表ヘッダではなく該当行に紐づく",
			blocks:   tableHeaders,
			item:     aiReviewItem{Quote: "平成30年4月に○○県立△△高等学校へ入学しました"},
			wantBBox: "bbox-6",
		},
		{
			name:     "短い表ヘッダしか候補がなければ紐づけない",
			blocks:   makeBlocks("なし", "以上", "年"),
			item:     aiReviewItem{Quote: "賞罰の欄にはなしと記載しています"},
			wantBBox: "",
		},
		{
			name:     "同じ言い回しが2箇所にあるなら紐づけない",
			blocks:   duplicated,
			item:     aiReviewItem{Quote: "接客のアルバイトで培った傾聴力"},
			wantBBox: "",
		},
		{
			// 小数点は残すので「1.5倍」と「15倍」は区別される
			name:     "数値が違う記述ではなく一致する記述に紐づく",
			blocks:   makeBlocks("売上を1.5倍にしました", "売上を15倍にしました"),
			item:     aiReviewItem{Quote: "売上を15倍にしました"},
			wantBBox: "bbox-2",
		},
		{
			// 桁区切りのカンマは落とすので書式差では減点しない
			name:     "桁区切りの有無は同一視する",
			blocks:   makeBlocks("売上を1,200万円伸ばしました"),
			item:     aiReviewItem{Quote: "売上を1200万円伸ばしました"},
			wantBBox: "bbox-1",
		},
		{
			name:     "波ダッシュと全角チルダの違い",
			blocks:   makeBlocks("2020〜2023年に在籍しました"),
			item:     aiReviewItem{Quote: "2020～2023年に在籍しました"},
			wantBBox: "bbox-1",
		},
		{
			// page_hint が指すブロックも引用と一致しなければ採用しない
			name:     "page_hintが指すブロックが引用と無関係なら紐づけない",
			blocks:   standard,
			item:     aiReviewItem{Quote: "宇宙開発の研究室で衛星の姿勢制御を研究しました", PageHint: 1, BlockIndex: 2},
			wantBBox: "",
		},
		{
			// page_hint は曖昧な候補の決定には使える（裏取りが通るため）
			name:     "同じ言い回しが2箇所でもpage_hintが裏取りできれば紐づく",
			blocks:   duplicated,
			item:     aiReviewItem{Quote: "接客のアルバイトで培った傾聴力", PageHint: 1, BlockIndex: 2},
			wantBBox: "bbox-2",
		},
		{
			// #1522 では margin 0.025 がこの差(0.036)を拾えず誤紐づけしていた。
			// 連結窓(#1559)で margin を 0.1 に戻せたので欠落するようになった。
			name: "語尾だけ違う近似ブロックには紐づけない",
			blocks: makeBlocks(
				"接客のアルバイトで培った傾聴力を活かしたいです。",
				"接客のアルバイトで培った傾聴力を発揮します。",
			),
			item:     aiReviewItem{Quote: "接客のアルバイトで培った傾聴力を仕事で使う"},
			wantBBox: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapOne(t, tt.blocks, tt.item); got != tt.wantBBox {
				t.Errorf("BBox = %q, want %q", got, tt.wantBBox)
			}
		})
	}
}

// TestQuoteMatchingRejectsDegenerateQuotes は、正規化で1〜3文字まで縮む引用を捨てることを固定する。
// Normalize は約物・記号を落とすので、生の文字数でゲートすると素通りしてしまう。
// 1文字の漢字は1ブロックにだけ含まれることがあり、満点1件として margin も抜けて
// 無関係な行に注釈PDFのピンが打たれる。
func TestQuoteMatchingRejectsDegenerateQuotes(t *testing.T) {
	blocks := makeBlocks(
		"私の強みは課題を分解して粘り強く取り組めることです。",
		blockWeb,
		blockMotive,
		"普通自動車免許を取得しています。",
	)

	tests := []struct {
		name  string
		quote string
	}{
		{name: "約物と記号だけで1文字に縮む", quote: "「、。・（）粘」"},
		{name: "記号で2文字に縮む", quote: "◯◯◯◯◯免許"},
		{name: "中黒で3文字に縮む", quote: "・・・・・自動車"},
		{name: "空白だけ", quote: "　 \n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if n := textsim.New(tt.quote).Len(); n >= quoteMinRunes {
				t.Fatalf("正規化後 %d 文字では前提が崩れる（quoteMinRunes=%d 未満のはず）", n, quoteMinRunes)
			}
			if got := mapOne(t, blocks, aiReviewItem{Quote: tt.quote}); got != "" {
				t.Errorf("BBox = %q, want 空文字（生%d文字でも正規化後は短すぎる）", got, len([]rune(tt.quote)))
			}
			// page_hint 経路も同じゲートを通ること
			hinted := aiReviewItem{Quote: tt.quote, PageHint: 1, BlockIndex: 1}
			if got := mapOne(t, blocks, hinted); got != "" {
				t.Errorf("page_hint あり: BBox = %q, want 空文字", got)
			}
		})
	}
}

// TestQuoteMatchingVerifiesPageHint は page_hint / block_index が指すブロックを
// 引用で裏取りすることを固定する。page_hint は幻覚するので無条件には信頼できない。
func TestQuoteMatchingVerifiesPageHint(t *testing.T) {
	// ocr_extract.py は block_index を 0 始まりで振る
	blocks := []models.ResumeTextBlock{
		{PageNumber: 1, BlockIndex: 0, Text: blockPR, BBox: "p1b0"},
		{PageNumber: 2, BlockIndex: 0, Text: "普通自動車免許を取得しています。", BBox: "p2b0"},
	}

	tests := []struct {
		name     string
		item     aiReviewItem
		wantBBox string
	}{
		{
			name:     "別ページを指していても引用に一致するブロックへ落ちる",
			item:     aiReviewItem{Quote: blockPR, PageHint: 2, BlockIndex: 0},
			wantBBox: "p1b0",
		},
		{
			name:     "本文に存在しない引用は紐づけない",
			item:     aiReviewItem{Quote: "宇宙開発の研究室で衛星の姿勢制御を研究しました", PageHint: 2, BlockIndex: 0},
			wantBBox: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapOne(t, blocks, tt.item); got != tt.wantBBox {
				t.Errorf("BBox = %q, want %q", got, tt.wantBBox)
			}
		})
	}
}

// TestQuoteMatchingAcceptsZeroBlockIndex は 0 始まりの block_index を受けることを固定する。
// ocr_extract.py は enumerate で block_index を振るので各ページ先頭行は B0 になる。
// 曖昧で探索だけでは決まらない候補を使い、page_hint 経路が実際に効いていることを確かめる。
func TestQuoteMatchingAcceptsZeroBlockIndex(t *testing.T) {
	const quote = "接客のアルバイトで培った傾聴力"
	blocks := []models.ResumeTextBlock{
		{PageNumber: 1, BlockIndex: 0, Text: "接客のアルバイトで培った傾聴力を活かしたいです。", BBox: "p1b0"},
		{PageNumber: 1, BlockIndex: 1, Text: "接客のアルバイトで培った傾聴力を仕事で発揮します。", BBox: "p1b1"},
	}

	// 前提: どちらも満点なので、探索だけでは曖昧として捨てられる
	if got := mapOne(t, blocks, aiReviewItem{Quote: quote}); got != "" {
		t.Fatalf("page_hint なしで BBox = %q, want 空文字（このテストの前提が崩れた）", got)
	}

	if got := mapOne(t, blocks, aiReviewItem{Quote: quote, PageHint: 1, BlockIndex: 0}); got != "p1b0" {
		t.Errorf("BBox = %q, want p1b0（0始まりのblock_index）", got)
	}
}

// TestQuoteMatchingIgnoresPageHint は page_hint が幻覚しても
// 他ページの完全一致を取りこぼさないことを固定する。
// page_hint でページを絞ると、同ページの弱い候補(0.851)に誤って紐づく。
func TestQuoteMatchingIgnoresPageHint(t *testing.T) {
	const quote = "アルバイトでは接客を通じて売上向上に貢献しました。"
	blocks := []models.ResumeTextBlock{
		{PageNumber: 1, BlockIndex: 1, Text: "アルバイトでは接客を通して売上の向上に貢献しました。", BBox: "p1"},
		{PageNumber: 2, BlockIndex: 1, Text: quote, BBox: "p2"},
	}
	if got := score(blocks[0].Text, quote); got < 0.846 || got > 0.856 {
		t.Fatalf("前提が崩れた: p1のスコア = %.4f, want 0.851 前後", got)
	}

	// block_index を伴わない page_hint は当てにならないので無視する
	for _, pageHint := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("page_hint=%d", pageHint), func(t *testing.T) {
			got := mapOne(t, blocks, aiReviewItem{Quote: quote, PageHint: pageHint})
			if got != "p2" {
				t.Errorf("BBox = %q, want p2（完全一致するブロック）", got)
			}
		})
	}
}

// TestQuoteMatchUniqueExactWins は「完全一致が1件だけなら margin を適用せず採用する」を固定する。
// 1行が長いほど1語違いの行のスコアは 1.0 に漸近する(≒1-2/N)ため、
// margin だけに任せると長い行の完全一致が次点に潰されて欠落する。
func TestQuoteMatchUniqueExactWins(t *testing.T) {
	const (
		quote = "アルバイト先の書店では店長の代理として発注業務とシフト作成を任され、繁忙期には新人スタッフ5名の教育も並行して担当し、前年比110%の売上を達成することができ、店長からも高い評価をいただきました。"
		near  = "アルバイト先の書店では店長の代理として発注業務とシフト作成を任され、繁忙期には新人スタッフ6名の教育も並行して担当し、前年比110%の売上を達成することができ、店長からも高い評価をいただきました。"
	)
	blocks := makeBlocks(near, quote)

	exact, second := score(quote, quote), score(near, quote)
	if exact != 1 {
		t.Fatalf("完全一致の前提が崩れた: %.4f", exact)
	}
	if diff := exact - second; diff >= quoteMatchMargin {
		t.Fatalf("次点との差 %.4f が margin %.3f 以上では、このテストがガードを検証できない", diff, quoteMatchMargin)
	}

	if got := mapOne(t, blocks, aiReviewItem{Quote: quote}); got != "bbox-2" {
		t.Errorf("BBox = %q, want bbox-2（唯一の完全一致）", got)
	}
}

// TestQuoteMatchThresholdBoundary は quoteMatchThreshold を上下から固定する。
// 候補が1件しかない組み合わせを使うので、採否はしきい値だけで決まる。
func TestQuoteMatchThresholdBoundary(t *testing.T) {
	const quote = "接客を通じて売上に貢献した"

	tests := []struct {
		name      string
		block     string
		wantScore float64 // 実測値
		wantMatch bool
	}{
		{
			name:      "しきい値の少し上なら紐づく",
			block:     "アルバイトでは接客を通して売上の向上に貢献しました。",
			wantScore: 0.556,
			wantMatch: true,
		},
		{
			name:      "しきい値の少し下なら紐づけない",
			block:     "接客を通じて売上の向上に少し貢献できたと思います。",
			wantScore: 0.514,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := score(tt.block, quote); got < tt.wantScore-0.005 || got > tt.wantScore+0.005 {
				t.Fatalf("Score = %.3f, want %.3f 前後（類似度の計算式が変わった）", got, tt.wantScore)
			}
			// 0.514 < quoteMatchThreshold <= 0.556 でなければどちらかが落ちる
			got := mapOne(t, makeBlocks(tt.block), aiReviewItem{Quote: quote})
			if gotMatch := got != ""; gotMatch != tt.wantMatch {
				t.Errorf("紐づき = %v, want %v (quoteMatchThreshold=%.3f)", gotMatch, tt.wantMatch, quoteMatchThreshold)
			}
		})
	}
}

// TestQuoteMatchMarginBoundary は quoteMatchMargin を上下から固定する。
// 最良の窓と、それに重ならない次点の窓との差だけで採否が決まるケースを並べている。
//
// 固定できている帯は 0.0989〜0.1278（この帯を外すとどれかが落ちる）。
// #1522 時点では 0.023〜0.038 しか使えなかったが、連結窓(#1559)で
// 一文全体が1つの候補になり、2行・3行に分かれた引用が次点を大きく上回るようになった。
func TestQuoteMatchMarginBoundary(t *testing.T) {
	tests := []struct {
		name     string
		blocks   []models.ResumeTextBlock
		quote    string
		wantDiff float64 // 最良の窓と、重ならない次点の窓との実測差
		wantBBox string  // 空文字なら紐づかないことを期待する
	}{
		{
			// margin の下限。これを 0.036 以下にすると誤った行に注釈が焼かれる
			name: "語尾だけ違う近似ブロックなら紐づけない",
			blocks: makeBlocks(
				"接客のアルバイトで培った傾聴力を活かしたいです。", // 0.837
				"接客のアルバイトで培った傾聴力を発揮します。",   // 0.873
			),
			quote:    "接客のアルバイトで培った傾聴力を仕事で使う",
			wantDiff: 0.036,
			wantBBox: "",
		},
		{
			name: "1語違いのブロックどうしなら紐づけない",
			blocks: makeBlocks(
				"Webサイトの制作を担当し表示速度を改善しました。",  // 0.634
				"Webサイトの制作を担当しアクセス数を伸ばしました。", // 0.619
			),
			quote:    "Webサイトの制作を担当し成果を出した",
			wantDiff: 0.015,
			wantBBox: "",
		},
		{
			// margin の実質的な下限。2行の引用元(0.750)に対し、別箇所の似た一文が
			// 0.651 まで迫るので、margin を 0.0988 以下にするとここへ紐づいてしまう
			name: "別の箇所に似た一文があるなら紐づけない",
			blocks: makeBlocks(
				"アルバイトでは接客を通じて店舗の",
				"売上向上に大きく貢献しました。",
				"自己PR",
				"インターンでは接客を通じて店舗の集客に貢献しました。",
			),
			quote:    "接客を通じて店舗の売上向上に貢献しました",
			wantDiff: 0.0988,
			wantBBox: "",
		},
		{
			// margin の上限。0.1278 より大きくすると、2行に分かれた引用が
			// 似た一文に押されてまるごと欠落する
			name: "別箇所の一文が似ていても差があれば紐づく",
			blocks: makeBlocks(
				"アルバイトでは接客を通じて店舗の",
				"売上向上に大きく貢献しました。",
				"自己PR",
				"アルバイトでは接客を通じて店舗の課題解決に貢献しました。",
			),
			quote:    "接客を通じて店舗の売上向上に貢献しました",
			wantDiff: 0.1278,
			wantBBox: "bbox-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best, second := windowScores(tt.blocks, tt.quote)
			if best < quoteMatchThreshold || second < quoteMatchThreshold {
				t.Fatalf("両候補がしきい値を超えている前提が崩れた: %.4f / %.4f", best, second)
			}
			if best == 1 {
				t.Fatalf("完全一致を含まない前提が崩れた（満点は margin を通らない）: %.4f", best)
			}
			if diff := best - second; diff < tt.wantDiff-0.005 || diff > tt.wantDiff+0.005 {
				t.Fatalf("差 = %.4f, want %.3f 前後（類似度の計算式が変わった）", diff, tt.wantDiff)
			}
			// 0.0988 < quoteMatchMargin <= 0.1278 でなければどれかが落ちる
			if got := mapOne(t, tt.blocks, aiReviewItem{Quote: tt.quote}); got != tt.wantBBox {
				t.Errorf("BBox = %q, want %q (quoteMatchMargin=%.3f)", got, tt.wantBBox, quoteMatchMargin)
			}
		})
	}
}

// TestQuoteMatchMultiLineSplit は一文が複数のOCR行に分かれた引用が
// 正しい行に紐づくことを固定する（Issue #1559）。
//
// OCRブロックは行単位（ocr_extract.py）なのに、プロンプトが要求する引用は一文なので、
// 自己PRや志望動機では1文が2〜4行に折り返すのが主系。1行ずつの照合では
// Dice 係数が原理的にしきい値へ届かず（3等分なら最大 0.525 前後）、
// しきい値を下げても1位と2位の差が 0.025 しかなく margin で落ちていた。
//
// 期待する紐づけ先は「引用と最も長く重なる行」。注釈PDFのbboxは行単位なので、
// 窓が複数行にまたがっても代表の1行に寄せる（pickRepresentative のコメント参照）。
func TestQuoteMatchMultiLineSplit(t *testing.T) {
	// 紙面に必ず混ざる短い行と、紛らわしい別の一文
	noise := []string{"氏名", "年", "月", "日"}

	tests := []struct {
		name string
		// lines は連結すると引用になる行。
		lines []string
		// wantBBox は代表行（noise 4行のあとに並ぶので bbox-5 から）。
		wantBBox string
		// needsWindow は「1行ずつの照合では紐づかない＝#1559 の再現」であることを表す。
		// 行の長さが偏っていると1行でも margin を抜けられるので、全ケースには立たない。
		needsWindow bool
	}{
		{
			name:        "2行に均等に分かれた自己PR",
			lines:       []string{"私は責任感を持って最後まで", "やり遂げることができます。"},
			wantBBox:    "bbox-5",
			needsWindow: true,
		},
		{
			name:     "2行に偏って分かれたガクチカ",
			lines:    []string{"学生時代はサークルの新人勧誘を担当し、", "50名の入会につなげました。"},
			wantBBox: "bbox-5",
		},
		{
			name:        "3行に分かれた自己PR",
			lines:       []string{"飲食店のアルバイトでは常に", "お客様の様子を観察し、注文前に水を", "追加するなどの先回りを心がけました。"},
			wantBBox:    "bbox-7",
			needsWindow: true,
		},
		{
			name:     "3行に分かれた課題解決のエピソード",
			lines:    []string{"研究室ではデータ収集の手順が属人化していたため、", "手順書を作成して後輩でも同じ精度で", "測定できる状態に整えました。"},
			wantBBox: "bbox-5",
		},
		{
			name: "4行に分かれた志望動機",
			lines: []string{
				"御社を志望した理由は、地域の中小企業を", "支えるという理念に共感したからです。",
				"学生時代に商店街の活性化イベントを", "運営した経験を活かせると考えています。",
			},
			wantBBox:    "bbox-5",
			needsWindow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			texts := append(append([]string{}, noise...), tt.lines...)
			blocks := makeBlocks(texts...)
			quote := strings.Join(tt.lines, "")

			// 前提: needsWindow のケースは1行ずつの照合（#1559 以前）では紐づかないこと。
			// これが成り立たないと、連結窓が効いている証拠にならない。
			if lineOnly := matchedLineOnly(blocks, quote); lineOnly == tt.needsWindow {
				t.Fatalf("1行ずつの照合で紐づく=%v, needsWindow=%v（#1559 の再現条件が変わった）",
					lineOnly, tt.needsWindow)
			}
			if got := mapOne(t, blocks, aiReviewItem{Quote: quote}); got != tt.wantBBox {
				t.Errorf("BBox = %q, want %q", got, tt.wantBBox)
			}
		})
	}
}

// TestQuoteMaxWindowBlocks_Pinned は連結窓の上限を固定する（#1559）。
//
// 振る舞いでは上限を挟めない（広げても既存ケースは通る）ので値そのものを固定する。
// 4行にしているのは、履歴書・ESの記入欄で一文が折り返すのが2〜4行までという前提と、
// 窓を広げるほど無関係な行を巻き込んだ窓が増えて誤紐づけの危険が上がるため。
// 1行に戻すと TestQuoteMatchMultiLineSplit が落ちる（連結窓そのものの効果はそちらで固定）。
// 広げるなら、まず TestQuoteMatchMultiLineDoesNotOverreach のような
// 誤紐づけの実測をやり直すこと。
func TestQuoteMaxWindowBlocks_Pinned(t *testing.T) {
	if quoteMaxWindowBlocks != 4 {
		t.Errorf("quoteMaxWindowBlocks = %d。誤紐づけの実測をやり直さずに変更していないか確認すること", quoteMaxWindowBlocks)
	}
}

// TestQuoteMatchMultiLineDoesNotOverreach は連結窓が誤紐づけを増やさないことを固定する。
// 窓を広げるほど「無関係な行を巻き込んだ窓」が増えるため、本文に無い引用が
// どこかの窓に吸着しないかを確かめる。
func TestQuoteMatchMultiLineDoesNotOverreach(t *testing.T) {
	blocks := makeBlocks(
		"氏名", "年", "月", "日",
		"飲食店のアルバイトでは常に",
		"お客様の様子を観察し、注文前に水を",
		"追加するなどの先回りを心がけました。",
		"御社を志望した理由は、地域の中小企業を",
		"支えるという理念に共感したからです。",
		"普通自動車第一種運転免許", "なし", "以上",
	)

	tests := []struct {
		name  string
		quote string
	}{
		{name: "本文に無い研究の話", quote: "宇宙開発の研究室で衛星の姿勢制御を研究しました"},
		{name: "本文に無い資格の話", quote: "TOEICで900点を取得し英語での商談経験もあります"},
		{name: "本文に無い部活の話", quote: "高校時代は野球部でキャプテンを務め県大会でベスト8に入りました"},
		{name: "本文に無いアルバイトの話", quote: "アルバイトでは効率を優先して作業していました"},
		{name: "短い行だけを拾える引用", quote: "賞罰の欄にはなしと記載しています"},
		{name: "語彙は本文由来だが内容が別", quote: "接客では常にお客様の様子を観察していました"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapOne(t, blocks, aiReviewItem{Quote: tt.quote}); got != "" {
				t.Errorf("BBox = %q, want 空文字（本文に無い引用を紐づけている）", got)
			}
		})
	}
}

// TestAdoptRetryItems は紐づけリトライの結果を採用する条件を固定する。
func TestAdoptRetryItems(t *testing.T) {
	items := func(n int) []models.ResumeReviewItem {
		return make([]models.ResumeReviewItem, n)
	}

	tests := []struct {
		name    string
		initial []models.ResumeReviewItem
		retry   []models.ResumeReviewItem
		want    int
	}{
		{name: "リトライが0件なら初回を残す", initial: items(2), retry: nil, want: 2},
		{name: "どちらも0件なら0件（呼び出し側がエラーにする）", initial: nil, retry: nil, want: 0},
		{name: "リトライが増えたら採用する", initial: items(2), retry: items(3), want: 3},
		{name: "同数ならリトライを採用しない", initial: items(3), retry: items(3), want: 3},
		{name: "初回0件・リトライ1件なら採用する", initial: nil, retry: items(1), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adoptRetryItems(tt.initial, tt.retry); len(got) != tt.want {
				t.Errorf("件数 = %d, want %d", len(got), tt.want)
			}
		})
	}
}
