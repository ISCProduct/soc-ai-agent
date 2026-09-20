package admin

// 管理画面・バッチ経路に「検索結果の共有」が届いているかのテスト。
// 実行: cd Backend && go test ./internal/controllers/ -run TestSetInfoFetcher -v
//
// NewAdminCompanyController は openaiClient を渡されると infoFetcher を自前生成する。
// その生成物には main.go 側の SetSharedSearch が掛かっていないため、放っておくと
// fetch-missing-batch(既定30社・最大50社)が企業ごとに web_search を余分に撃つ。
// web_search は検索結果が固定8,000トークン/call で課金されるため、回数がそのまま
// コストになる(#1124)。

import (
	"testing"

	"Backend/internal/openai"
	"Backend/internal/services/company"
)

func newControllerWithSelfBuiltFetchers(t *testing.T) *AdminCompanyController {
	t.Helper()
	// クライアントを渡すことで、コンストラクタに infoFetcher を自前生成させる。
	// これが本番の main.go と同じ状態。
	c := NewAdminCompanyController(nil, nil, nil, openai.NewWithBaseURL("http://127.0.0.1:0", "gpt-4o-mini"))
	if c.infoFetcher == nil {
		t.Fatal("前提が崩れている: コンストラクタが infoFetcher を生成しなくなった")
	}
	return c
}

func TestSetInfoFetcher_共有済みインスタンスに差し替える(t *testing.T) {
	c := newControllerWithSelfBuiltFetchers(t)
	selfBuilt := c.infoFetcher

	shared := company.NewCompanyInfoFetcher(nil, openai.NewWithBaseURL("http://127.0.0.1:0", "gpt-4o-mini"))
	shared.SetSharedSearch(company.NewSearchContext())

	c.SetInfoFetcher(shared)

	if c.infoFetcher == selfBuilt {
		t.Error("自前生成のままになっている。SetSharedSearch が効かず web_search が減らない")
	}
	if c.infoFetcher != shared {
		t.Error("渡したインスタンスが使われていない")
	}
}

func TestSetInfoFetcher_抱えているサービスも作り直す(t *testing.T) {
	c := newControllerWithSelfBuiltFetchers(t)
	beforeBatch := c.missingBatch
	beforeWarm := c.catalogWarm

	shared := company.NewCompanyInfoFetcher(nil, openai.NewWithBaseURL("http://127.0.0.1:0", "gpt-4o-mini"))
	c.SetInfoFetcher(shared)

	// missingBatch と catalogWarm は infoFetcher を値として抱えている。
	// 作り直さないと、件数の出る経路だけ古い fetcher を使い続ける。
	if c.missingBatch == beforeBatch {
		t.Error("missingBatch が古い infoFetcher を抱えたまま。バッチに共有検索が届かない")
	}
	if c.catalogWarm == beforeWarm {
		t.Error("catalogWarm が古い infoFetcher を抱えたまま")
	}
}

func TestSetInfoFetcher_nilは無視する(t *testing.T) {
	c := newControllerWithSelfBuiltFetchers(t)
	before := c.infoFetcher

	c.SetInfoFetcher(nil)

	// nil で上書きすると管理画面の取得が丸ごと落ちる。何もしないのが正しい。
	if c.infoFetcher != before {
		t.Error("nil で潰してはいけない")
	}
}

func TestSetInfoFetcher_SetRelationsFetcherと順不同で噛み合う(t *testing.T) {
	// main.go は SetInfoFetcher → SetRelationsFetcher の順で呼ぶが、
	// 逆順でも両方が missingBatch に載っている必要がある。
	for _, tc := range []struct {
		name  string
		order []string
	}{
		{"info→relations", []string{"info", "relations"}},
		{"relations→info", []string{"relations", "info"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newControllerWithSelfBuiltFetchers(t)
			client := openai.NewWithBaseURL("http://127.0.0.1:0", "gpt-4o-mini")
			info := company.NewCompanyInfoFetcher(nil, client)
			relations := company.NewCompanyRelationsFetcher(nil, nil, client)

			for _, step := range tc.order {
				switch step {
				case "info":
					c.SetInfoFetcher(info)
				case "relations":
					c.SetRelationsFetcher(relations)
				}
			}

			if c.infoFetcher != info {
				t.Error("infoFetcher が上書きで失われている")
			}
			if c.relationsFetcher != relations {
				t.Error("relationsFetcher が上書きで失われている")
			}
			if c.missingBatch == nil {
				t.Error("missingBatch が組み立てられていない")
			}
		})
	}
}
