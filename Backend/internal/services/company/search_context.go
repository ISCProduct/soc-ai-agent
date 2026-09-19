package company

// 企業1社あたりの web_search を1回にまとめるための検索結果の共有。
//
// 企業情報の取得は info / relations / tech の3系統に分かれており、それぞれが
// 独立して web_search を発行していた(#1124)。web_search は検索結果が固定
// 8,000トークン/call として課金されるため、1社を埋めるのに3回分の固定課金が
// 発生していた。
//
// singleflight は入っているが同時実行をまとめるだけで、op キーも
// info / relations / tech と別なので効かない。TTLキャッシュは2回目以降にしか
// 効かず、新規企業の初回取得では3系統とも発行される。
//
// ここでは検索そのものを1回にし、得られたテキストを3系統で使い回す。
// 抽出(Parse)は用途ごとに別で行う。Parse は通常のチャット課金なので、
// 回数が増えても Search の固定課金に比べれば桁が違う。
//
// 検索プロンプトを1つに統合しないのは、3系統が辿るページが異なるため。
// 「企業概要」と「子会社の資本関係」と「採用ページの技術情報」を1クエリに
// まとめると、どれも浅くなる。代わりに1回の検索で3系統分を尋ねる。

import (
	"context"
	"sync"
	"time"

	"Backend/internal/companyfetch"
)

// searchContextTTL は共有した検索結果を使い回す時間。
// 1リクエスト内で3系統が順に動くことを想定しており、長く持つ必要はない。
// 長く持つと情報の鮮度が落ち、TTLキャッシュ(90/60/7日)との二重管理にもなる。
const searchContextTTL = 5 * time.Minute

// SearchContext は企業ごとの検索結果を短時間だけ保持する。
type SearchContext struct {
	mu      sync.Mutex
	entries map[string]*searchEntry
}

type searchEntry struct {
	once    sync.Once
	text    string
	model   string
	err     error
	storedA time.Time
}

func NewSearchContext() *SearchContext {
	return &SearchContext{entries: map[string]*searchEntry{}}
}

// Fetch は企業キーに対する検索結果を返す。同じキーなら fetch は1度しか呼ばれない。
//
// 期限切れのエントリは捨てて引き直す。fetch が失敗した場合も結果(エラー)を
// 共有する。失敗のたびに3系統が個別に再検索すると、課金だけが増える。
func (c *SearchContext) Fetch(companyKey string, fetch func() (text string, model string, err error)) (string, string, error) {
	if c == nil {
		return fetch()
	}

	c.mu.Lock()
	e, ok := c.entries[companyKey]
	if !ok || time.Since(e.storedA) > searchContextTTL {
		e = &searchEntry{storedA: time.Now()}
		c.entries[companyKey] = e
	}
	c.mu.Unlock()

	e.once.Do(func() {
		e.text, e.model, e.err = fetch()
	})
	return e.text, e.model, e.err
}

// Forget は保持している検索結果を捨てる。強制更新のときに使う。
func (c *SearchContext) Forget(companyKey string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, companyKey)
	c.mu.Unlock()
}

// sharedSearchPrompt は1回の検索で3系統分を尋ねるプロンプトを組み立てる。
//
// 個別のプロンプトを単純に連結するのではなく、検索対象のページが異なることを
// 明示して、どれかに偏らないようにする。
func sharedSearchPrompt(companyName, websiteURL string) string {
	siteHint := ""
	if websiteURL != "" {
		siteHint = "（公式サイト: " + websiteURL + "）"
	}

	return "日本の企業「" + companyName + "」" + siteHint + "について、公開情報から確認できる事実を調べてください。" +
		"次の3つはそれぞれ別のページに載っていることが多いので、どれか1つに偏らず順に確認してください。\n\n" +
		"【1. 企業の基本情報】会社概要・IR資料から\n" +
		"企業概要、業種、本社所在地、公式サイトURL、設立年、従業員数（直近有価証券報告書の連結を優先し、連結か単体かを明記）、" +
		"主要事業、企業文化、勤務スタイル（リモート/ハイブリッド/オフィス）、福利厚生。\n\n" +
		"【2. 資本関係・取引関係】有価証券報告書・適時開示から\n" +
		"子会社・グループ会社（議決権50%以上）と出資比率、資本提携・関連会社（議決権50%未満）と提携内容、" +
		"取引先・業務提携先（資本関係が無い相手）と取引内容、入札・調達・補助金があれば発注元の省庁・自治体名、上場区分・証券コード。" +
		"社名が曖昧なものは載せないでください。\n\n" +
		"【3. 技術・設備】採用ページ・技術ブログ・製品紹介から\n" +
		"IT企業なら開発言語・フレームワーク・クラウド・CI/CD・開発手法。" +
		"製造業なら製品技術・生産設備・生産方式・拠点。該当しない項目は無理に埋めないでください。\n\n" +
		"各事実の根拠URLを含めてください。不明な項目は推測せず「不明」と書いてください。"
}

// searchThenParse は共有の検索結果を使って解析だけを行う。
// 共有が無い場合は従来どおり検索と解析をまとめて行う。
//
// 3系統(info / relations / tech)がこれを通ることで、web_search は
// 企業ごとに1回で済む。解析は用途ごとのスキーマで別々に行う。
func searchThenParse(
	ctx context.Context,
	shared *SearchContext,
	llm *companyfetch.LLM,
	companyKey, sharedPrompt, fallbackSearchPrompt, systemPrompt, parseUser string,
	parseMaxTokens int,
) (raw string, modelsUsed string, err error) {
	if shared == nil {
		return llm.SearchLiteThenParse(ctx, fallbackSearchPrompt, systemPrompt, parseUser, parseMaxTokens)
	}

	searchText, searchModel, err := shared.Fetch(companyKey, func() (string, string, error) {
		return llm.SearchLiteJSON(ctx, sharedPrompt, 1500)
	})
	if err != nil {
		return "", "", err
	}

	parsed, parseModel, err := llm.ParseJSON(ctx, systemPrompt,
		parseUser+"\n\n---\n検索結果:\n"+companyfetch.TrimText(searchText, 2000), parseMaxTokens)
	if err != nil {
		return "", searchModel, err
	}
	return parsed, searchModel + "+" + parseModel, nil
}
