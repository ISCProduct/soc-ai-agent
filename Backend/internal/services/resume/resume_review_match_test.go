package resume

// 引用(quote)と本文ブロックの照合テスト（Issue #1522）
// 実行: cd Backend && go test ./internal/services/resume/... -run TestQuoteMatch -v
//
// quoteMatchThreshold / quoteMatchMargin の値は TestQuoteMatchThresholdBoundary と
// TestQuoteMatchMarginBoundary が上下から固定している。しきい値を動かすとどちらかが落ちる。

import (
	"fmt"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared/textsim"
)

// score は本文ブロックに対する引用の一致度を返す（境界値の記録用）。
func score(blockText, quote string) float64 {
	return textsim.New(blockText).MatchScore(textsim.New(quote))
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
			name:     "page_hintとblock_indexが一致すれば引用が弱くても紐づく",
			blocks:   standard,
			item:     aiReviewItem{Quote: "宇宙開発の研究", PageHint: 1, BlockIndex: 2},
			wantBBox: "bbox-2",
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
// どちらの候補もしきい値を超え、完全一致は含まないので、採否は1位と2位の差だけで決まる。
//
// 一文が2行に分かれた引用は、行の長さが近いほど差が小さくなる（均等分割で 0.038 まで）。
// margin を大きくするとこの種の引用がまるごと欠落するので、上限として固定している。
// 3行以上に分かれるケースは原理的に届かない（Issue #1559）。
func TestQuoteMatchMarginBoundary(t *testing.T) {
	tests := []struct {
		name     string
		blocks   []models.ResumeTextBlock
		quote    string
		wantDiff float64 // 1位と2位の実測差
		wantBBox string  // 空文字なら紐づかないことを期待する
	}{
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
			name: "一文が偏って2行に分かれたケース",
			blocks: makeBlocks(
				"50名の入会につなげました。",      // 0.571
				"学生時代はサークルの新人勧誘を担当し、", // 0.723
			),
			quote:    "学生時代はサークルの新人勧誘を担当し、50名の入会につなげました。",
			wantDiff: 0.152,
			wantBBox: "bbox-2",
		},
		{
			name: "一文がほぼ均等に2行に分かれたケース",
			blocks: makeBlocks(
				"アルバイトでは接客を通じて店舗の", // 0.682
				"売上向上に大きく貢献しました。",  // 0.619
			),
			quote:    "アルバイトでは接客を通じて店舗の売上向上に大きく貢献しました。",
			wantDiff: 0.063,
			wantBBox: "bbox-1",
		},
		{
			name: "一文が均等に2行に分かれた最も差の小さいケース",
			blocks: makeBlocks(
				"私は責任感を持って最後まで", // 0.667
				"やり遂げることができます。", // 0.629
			),
			quote:    "私は責任感を持って最後までやり遂げることができます。",
			wantDiff: 0.038,
			wantBBox: "bbox-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := score(tt.blocks[0].Text, tt.quote)
			second := score(tt.blocks[1].Text, tt.quote)
			if first < quoteMatchThreshold || second < quoteMatchThreshold {
				t.Fatalf("両候補がしきい値を超えている前提が崩れた: %.4f / %.4f", first, second)
			}
			if first == 1 || second == 1 {
				t.Fatalf("完全一致を含まない前提が崩れた: %.4f / %.4f", first, second)
			}
			diff := first - second
			if diff < 0 {
				diff = -diff
			}
			if diff < tt.wantDiff-0.005 || diff > tt.wantDiff+0.005 {
				t.Fatalf("差 = %.4f, want %.3f 前後（類似度の計算式が変わった）", diff, tt.wantDiff)
			}
			// 0.015 < quoteMatchMargin <= 0.038 でなければどれかが落ちる
			if got := mapOne(t, tt.blocks, aiReviewItem{Quote: tt.quote}); got != tt.wantBBox {
				t.Errorf("BBox = %q, want %q (quoteMatchMargin=%.3f)", got, tt.wantBBox, quoteMatchMargin)
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
