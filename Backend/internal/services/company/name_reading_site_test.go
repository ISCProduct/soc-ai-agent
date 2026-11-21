package company

import (
	"context"
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
)

type fakeAI struct {
	reply    string
	err      error
	calls    int
	lastUser string
}

func (f *fakeAI) ChatCompletionJSON(_ context.Context, _, user string, _ float32, _ int, _ ...string) (string, error) {
	f.calls++
	f.lastUser = user
	return f.reply, f.err
}

func fakeFetch(pages map[string]string) PageFetcher {
	return func(_ context.Context, url string) (string, error) {
		if h, ok := pages[url]; ok {
			return h, nil
		}
		return "", errors.New("404")
	}
}

const topPage = `<html><body><nav><a href="/news">ニュース</a><a href="/company/">会社概要</a><a href="https://other.example.com/company">外部</a></nav><p>サンプルのトップページです。</p></body></html>`
const overviewPage = `<html><body><table><tr><th>会社名</th><td>株式会社サンプル商事（かぶしきがいしゃさんぷるしょうじ）</td></tr><tr><th>所在地</th><td>東京都千代田区</td></tr></table></body></html>`

func company1() *models.Company {
	return &models.Company{ID: 1, Name: "株式会社サンプル商事", WebsiteURL: "https://sample.example.jp/"}
}

func TestExtractSiteReading_Grounded(t *testing.T) {
	ai := &fakeAI{reply: `{"reading":"サンプルショウジ","quote":"株式会社サンプル商事（かぶしきがいしゃさんぷるしょうじ）"}`}
	e := &SiteReadingExtractor{Fetch: fakeFetch(map[string]string{"https://sample.example.jp/": topPage, "https://sample.example.jp/company/": overviewPage}), AI: ai}
	got, err := e.Extract(context.Background(), company1())
	if err != nil {
		t.Fatal(err)
	}
	// 法人格（カブシキガイシャ）は読みに含めない（法人番号公表サイトのフリガナと同じ形式にそろえる）
	if got.Reading != "サンプルショウジ" || got.Reason != "" || got.SourceURL != "https://sample.example.jp/company/" {
		t.Fatalf("got %+v", got)
	}
}

func TestExtractSiteReading_StripsLegalFormFromReading(t *testing.T) {
	ai := &fakeAI{reply: `{"reading":"カブシキガイシャサンプルショウジ","quote":"株式会社サンプル商事（かぶしきがいしゃさんぷるしょうじ）"}`}
	e := &SiteReadingExtractor{Fetch: fakeFetch(map[string]string{"https://sample.example.jp/": topPage, "https://sample.example.jp/company/": overviewPage}), AI: ai}
	got, _ := e.Extract(context.Background(), company1())
	if got.Reading != "サンプルショウジ" {
		t.Fatalf("法人格を除く: %+v", got)
	}
}

func TestExtractSiteReading_Rejects(t *testing.T) {
	pages := map[string]string{"https://sample.example.jp/": topPage, "https://sample.example.jp/company/": overviewPage}
	tests := []struct {
		name  string
		reply string
		why   string
	}{
		{"AIが推測した読み（引用文に読みが無い）", `{"reading":"サンプルトレーディング","quote":"株式会社サンプル商事（かぶしきがいしゃさんぷるしょうじ）"}`, ReasonSiteNotGrounded},
		{"引用文がページに存在しない（捏造）", `{"reading":"サンプルショウジ","quote":"サンプル商事の読みはサンプルショウジです"}`, ReasonSiteNotGrounded},
		{"サイトに記載が無い（AIが空を返す）", `{"reading":"","quote":""}`, ReasonSiteNoReading},
		{"読みとして使えない形（漢字を含む）", `{"reading":"サンプル商事","quote":"株式会社サンプル商事（かぶしきがいしゃさんぷるしょうじ）"}`, ReasonSiteNotGrounded},
		{"JSONでない応答", `わかりません`, ReasonSiteNotGrounded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &SiteReadingExtractor{Fetch: fakeFetch(pages), AI: &fakeAI{reply: tt.reply}}
			got, err := e.Extract(context.Background(), company1())
			if err != nil {
				t.Fatal(err)
			}
			if got.Reading != "" || got.Reason != tt.why {
				t.Fatalf("採用しないはず: %+v", got)
			}
		})
	}
}

func TestExtractSiteReading_RubyMarkupIsKept(t *testing.T) {
	page := `<html><body><h1>会社概要</h1><p><ruby>株式会社<rt>かぶしきがいしゃ</rt></ruby><ruby>山田製作所<rt>やまだせいさくしょ</rt></ruby></p><p>私たちは製造業の会社です。山田製作所は1950年に創業しました。</p></body></html>`
	ai := &fakeAI{reply: `{"reading":"ヤマダセイサクショ","quote":"株式会社（かぶしきがいしゃ）山田製作所（やまだせいさくしょ）"}`}
	c := &models.Company{ID: 2, Name: "株式会社山田製作所", WebsiteURL: "https://yamada.example.jp/"}
	e := &SiteReadingExtractor{Fetch: fakeFetch(map[string]string{"https://yamada.example.jp/": page}), AI: ai}
	got, err := e.Extract(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	// ルビは「漢字（よみ）」の形でテキスト化される（AIに渡す文章に、読みが残る）
	if !strings.Contains(ai.lastUser, "山田製作所（やまだせいさくしょ）") {
		t.Fatalf("ルビがAIに渡す文章に残っていない: %s", ai.lastUser)
	}
	if got.Reading != "ヤマダセイサクショ" {
		t.Fatalf("got %+v", got)
	}
}

// 社名の一部にしか読みが書かれていないとき、AI が残りを補った読みは採用しない（サイトに書かれていない部分は、推測）。
func TestExtractSiteReading_PartialRubyIsNotCompletedByAI(t *testing.T) {
	page := `<html><body><h1>会社概要</h1><p>株式会社<ruby>山田<rt>やまだ</rt></ruby>製作所は1950年に創業しました。</p></body></html>`
	ai := &fakeAI{reply: `{"reading":"ヤマダセイサクショ","quote":"株式会社山田（やまだ）製作所は1950年に創業しました。"}`}
	c := &models.Company{ID: 2, Name: "株式会社山田製作所", WebsiteURL: "https://yamada.example.jp/"}
	e := &SiteReadingExtractor{Fetch: fakeFetch(map[string]string{"https://yamada.example.jp/": page}), AI: ai}
	got, err := e.Extract(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reading != "" || got.Reason != ReasonSiteNotGrounded {
		t.Fatalf("書かれていない部分を補った読みは採用しない: %+v", got)
	}
}

func TestExtractSiteReading_OnlySameSiteLinks(t *testing.T) {
	var fetched []string
	fetch := func(_ context.Context, u string) (string, error) {
		fetched = append(fetched, u)
		if u == "https://sample.example.jp/" {
			return topPage, nil
		}
		return "", errors.New("404")
	}
	e := &SiteReadingExtractor{Fetch: fetch, AI: &fakeAI{}}
	_, _ = e.Extract(context.Background(), company1())
	for _, u := range fetched {
		if strings.Contains(u, "other.example.com") {
			t.Fatalf("別のサイトへ移ってはいけない: %v", fetched)
		}
	}
}

func TestExtractSiteReading_NoWebsiteOrNoPages(t *testing.T) {
	e := &SiteReadingExtractor{Fetch: fakeFetch(nil), AI: &fakeAI{}}
	if got, _ := e.Extract(context.Background(), &models.Company{Name: "株式会社サンプル商事"}); got.Reason != ReasonSiteNoWebsite {
		t.Fatalf("公式サイトの登録が無い: %+v", got)
	}
	if got, _ := e.Extract(context.Background(), company1()); got.Reason != ReasonSiteUnreachable {
		t.Fatalf("取得できるページが無い: %+v", got)
	}
}

func TestExtractSiteReading_AIErrorIsNotAMiss(t *testing.T) {
	boom := errors.New("ai down")
	e := &SiteReadingExtractor{Fetch: fakeFetch(map[string]string{"https://sample.example.jp/": topPage, "https://sample.example.jp/company/": overviewPage}), AI: &fakeAI{err: boom}}
	if _, err := e.Extract(context.Background(), company1()); !errors.Is(err, boom) {
		t.Fatalf("AIの失敗は「記載が無い」と区別して返す: %v", err)
	}
}

func TestExtractSiteReading_PageWithoutNameMentionDoesNotCallAI(t *testing.T) {
	ai := &fakeAI{reply: `{"reading":"ダレカ","quote":"x"}`}
	pages := map[string]string{"https://sample.example.jp/": `<html><body>無関係なページです。</body></html>`}
	e := &SiteReadingExtractor{Fetch: fakeFetch(pages), AI: ai}
	got, _ := e.Extract(context.Background(), company1())
	if ai.calls != 0 || got.Reading != "" {
		t.Fatalf("社名の記載が無いページでは、AIを呼ばない（費用と、誤りを避ける）: calls=%d %+v", ai.calls, got)
	}
}
