package company

// 検索結果の共有のテスト。
// 実行: cd Backend && go test ./internal/services/company/ -run "TestSearchContext|TestSharedSearchPrompt" -v
//
// 見たいのは「同じ企業なら web_search が1回で済むこと」。
// web_search は検索結果が固定8,000トークン/call で課金されるため、
// 回数がそのままコストになる(#1124)。

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSearchContext_同じ企業なら検索は1回(t *testing.T) {
	sc := NewSearchContext()
	var calls int

	fetch := func() (string, string, error) {
		calls++
		return "検索結果テキスト", "gpt-4o-mini", nil
	}

	// info / relations の2系統が順に引く想定。
	// tech は統合すると技術スタックが取れなくなるため対象外(実測で確認)。
	for range 2 {
		text, model, err := sc.Fetch("株式会社テスト", fetch)
		if err != nil {
			t.Fatalf("エラーが返った: %v", err)
		}
		if text != "検索結果テキスト" || model != "gpt-4o-mini" {
			t.Errorf("共有された値が違う: %q %q", text, model)
		}
	}

	if calls != 1 {
		t.Errorf("web_search の回数が %d 回。1回に寄せられていない", calls)
	}
}

func TestSearchContext_企業が違えば別々に検索する(t *testing.T) {
	sc := NewSearchContext()
	var calls int
	fetch := func() (string, string, error) {
		calls++
		return "text", "model", nil
	}

	sc.Fetch("A社", fetch)
	sc.Fetch("B社", fetch)

	if calls != 2 {
		t.Errorf("企業ごとに検索すべき: %d 回", calls)
	}
}

func TestSearchContext_失敗も共有する(t *testing.T) {
	sc := NewSearchContext()
	var calls int
	wantErr := errors.New("検索に失敗")

	fetch := func() (string, string, error) {
		calls++
		return "", "", wantErr
	}

	// 失敗のたびに3系統が個別に再検索すると課金だけが増える。
	for range 3 {
		_, _, err := sc.Fetch("株式会社テスト", fetch)
		if !errors.Is(err, wantErr) {
			t.Errorf("同じエラーを返すべき: %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("失敗時も検索は1回であるべき: %d 回", calls)
	}
}

func TestSearchContext_並行でも1回(t *testing.T) {
	sc := NewSearchContext()
	var mu sync.Mutex
	calls := 0

	fetch := func() (string, string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // 検索にかかる時間を模す
		return "text", "model", nil
	}

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc.Fetch("株式会社テスト", fetch)
		}()
	}
	wg.Wait()

	if calls != 1 {
		t.Errorf("並行呼び出しでも1回であるべき: %d 回", calls)
	}
}

func TestSearchContext_Forgetで引き直す(t *testing.T) {
	sc := NewSearchContext()
	var calls int
	fetch := func() (string, string, error) {
		calls++
		return "text", "model", nil
	}

	sc.Fetch("株式会社テスト", fetch)
	sc.Forget("株式会社テスト")
	sc.Fetch("株式会社テスト", fetch)

	if calls != 2 {
		t.Errorf("Forget 後は引き直すべき: %d 回", calls)
	}
}

func TestSearchContext_nilなら素通しする(t *testing.T) {
	var sc *SearchContext // 未注入
	var calls int

	// 共有が無い環境では従来どおり毎回検索する（動作を壊さない）。
	for range 2 {
		_, _, err := sc.Fetch("株式会社テスト", func() (string, string, error) {
			calls++
			return "text", "model", nil
		})
		if err != nil {
			t.Fatalf("エラー: %v", err)
		}
	}
	if calls != 2 {
		t.Errorf("nil なら毎回呼ぶべき: %d 回", calls)
	}
}

func TestSharedSearchPrompt(t *testing.T) {
	got := sharedSearchPrompt("株式会社テスト", "https://example.co.jp/")

	// 3系統ぶんを1回の検索で尋ねる。どれかが抜けると、その系統だけ
	// 情報が取れず結局あとで個別に検索することになる。
	for _, want := range []string{
		"株式会社テスト",
		"https://example.co.jp/",
		"企業概要", "従業員数", "勤務スタイル", // info
		"子会社", "出資比率", "上場区分", // relations
		"根拠URL",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("検索プロンプトに %q が含まれていない", want)
		}
	}

	// tech は統合しない。統合すると検索が会社概要・IR資料に寄り、
	// 採用ページや技術ブログまで辿らず技術スタックが取れなくなる(実測)。
	for _, ng := range []string{"開発言語", "生産設備", "技術ブログ"} {
		if strings.Contains(got, ng) {
			t.Errorf("技術は統合対象外にしたはず: %q が含まれている", ng)
		}
	}

	if !strings.Contains(got, "推測せず") {
		t.Error("推測を禁じる指示が必要")
	}
}

func TestSharedSearchPrompt_URLが無くても壊れない(t *testing.T) {
	got := sharedSearchPrompt("株式会社テスト", "")
	if strings.Contains(got, "公式サイト: )") || strings.Contains(got, "（公式サイト: ）") {
		t.Errorf("空URLの扱いが不正: %s", got[:120])
	}
	if !strings.Contains(got, "株式会社テスト") {
		t.Error("企業名が含まれていない")
	}
}
