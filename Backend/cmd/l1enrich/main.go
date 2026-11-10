// cmd/l1enrich は L1 カタログのシード CSV を、Claude の Web 検索で補う一度きりのローカル用ツール。
// 業種の確認・公式サイトの補完・IT 企業でない社名の除外を行い、CSV を更新する。
// 本番の処理ではない（企業取得に OpenAI を使わない方針のため、ここだけ Claude を使う）。
//
// 使い方（Backend ディレクトリで）:
//
//	ANTHROPIC_API_KEY=... go run ./cmd/l1enrich -limit 5          # まず少数で確認
//	ANTHROPIC_API_KEY=... go run ./cmd/l1enrich                   # 全件。中断しても再実行で続きから
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// 業種の選択肢。固定にして表記ゆれを防ぐ。
var industries = []string{"情報サービス・SI", "ソフトウェア", "インターネット・Web", "通信", "製造業", "金融・保険", "小売・サービス", "建設・不動産", "運輸・エネルギー", "その他"}

// 使用量の合計。費用の見積りに使う。
var inTok, outTok, searches atomic.Int64

// costUSD は Haiku 4.5 の単価（入力 $1/100万、出力 $5/100万、Web 検索 $10/1000回）での推定費用。
// 他のモデルでは単価が違うので目安にとどまる。
func costUSD() float64 {
	return float64(inTok.Load())*1e-6 + float64(outTok.Load())*5e-6 + float64(searches.Load())*0.01
}

type result struct {
	Name        string `json:"name"`
	Industry    string `json:"industry"`
	IsIT        bool   `json:"is_it_company"`
	WebsiteURL  string `json:"website_url"`
	Description string `json:"description"`
}

func main() {
	in := flag.String("in", "config/l1_catalog_seed.csv", "更新する CSV")
	cachePath := flag.String("cache", "l1_enrich_cache.jsonl", "結果の保存先。あれば続きから実行する")
	model := flag.String("model", "claude-haiku-4-5", "使うモデル。質を上げるなら claude-sonnet-5-5 など")
	limit := flag.Int("limit", 0, "未処理のうち先頭から N 社だけ処理する（0 は全件）")
	workers := flag.Int("workers", 8, "並列数")
	maxUSD := flag.Float64("max-usd", 0, "推定費用がこの金額（ドル）に達したら止める（0 は無制限）。Haiku 4.5 の単価で概算")
	keepNonIT := flag.Bool("keep-non-it", false, "IT 企業でないと判定された社名も残す")
	flag.Parse()

	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		log.Fatal("ANTHROPIC_API_KEY を環境変数で渡してください")
	}
	rows := readCSV(*in)
	cache := loadCache(*cachePath)

	var todo []string
	for _, r := range rows[1:] {
		if _, ok := cache[r[0]]; !ok {
			todo = append(todo, r[0])
		}
	}
	if *limit > 0 && len(todo) > *limit {
		todo = todo[:*limit]
	}
	log.Printf("対象 %d 社（処理済み %d 社）", len(todo), len(cache))

	client := anthropic.NewClient(option.WithAPIKey(key))
	var mu sync.Mutex
	cf, err := os.OpenFile(*cachePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer cf.Close()

	jobs := make(chan string)
	var wg sync.WaitGroup
	for range *workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range jobs {
				res, err := enrich(context.Background(), client, *model, name)
				if err != nil {
					log.Printf("失敗（次回やり直し）: %s: %v", name, err)
					continue
				}
				mu.Lock()
				cache[name] = res
				b, _ := json.Marshal(res)
				fmt.Fprintln(cf, string(b))
				mu.Unlock()
			}
		}()
	}
	for _, n := range todo {
		if *maxUSD > 0 && costUSD() >= *maxUSD {
			log.Printf("予算 $%.2f に達したので止めます（推定 $%.2f）。再実行すれば続きから処理します", *maxUSD, costUSD())
			break
		}
		jobs <- n
	}
	close(jobs)
	wg.Wait()

	out, dropped := apply(rows, cache, *keepNonIT)
	writeCSV(*in, out)
	log.Printf("CSV を更新: %d 社（IT 企業でないため除外 %d 社）", len(out)-1, dropped)
	log.Printf("使用量: 入力 %d トークン / 出力 %d トークン / Web 検索 %d 回（推定 $%.2f）", inTok.Load(), outTok.Load(), searches.Load(), costUSD())
}

// enrich は 1 社分を Claude の Web 検索で調べる。
func enrich(ctx context.Context, client anthropic.Client, model, name string) (result, error) {
	prompt := fmt.Sprintf(`日本の企業「%s」について Web 検索で調べ、次の JSON オブジェクトだけを返してください（前後に説明やコードフェンスを付けない）。
- industry: 次のどれか1つ: %s
- is_it_company: 主な事業が IT（システム開発・ソフトウェア・IT サービス）なら true
- website_url: 公式サイトの URL。確認できなければ空文字
- description: 事業内容を日本語で60字以内。確認できなければ空文字
推測で埋めず、確認できないものは空文字か false にすること。
{"industry":"","is_it_company":false,"website_url":"","description":""}`, name, strings.Join(industries, " / "))

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 8000, // 思考トークンも含まれる
		Tools:     []anthropic.ToolUnionParam{searchTool(model)},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	}
	for range 4 { // pause_turn は続行する
		resp, err := client.Messages.New(ctx, params)
		if err != nil {
			return result{}, err
		}
		inTok.Add(resp.Usage.InputTokens)
		outTok.Add(resp.Usage.OutputTokens)
		searches.Add(resp.Usage.ServerToolUse.WebSearchRequests)
		switch resp.StopReason {
		case anthropic.StopReasonPauseTurn:
			params.Messages = append(params.Messages, resp.ToParam())
			continue
		case anthropic.StopReasonRefusal:
			return result{}, fmt.Errorf("refused: %s", resp.StopDetails.Category)
		}
		return parseResult(name, resp)
	}
	return result{}, fmt.Errorf("pause_turn が続いた")
}

// searchTool は Web 検索ツール。件数が多いので 1 社 1 回に絞る。
// 動的フィルタ版（_20260209）は新しいモデルだけなので、Haiku 4.5 などは基本版を使う。
func searchTool(model string) anthropic.ToolUnionParam {
	if strings.Contains(model, "haiku") {
		return anthropic.ToolUnionParam{OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{MaxUses: anthropic.Int(1)}}
	}
	return anthropic.ToolUnionParam{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{MaxUses: anthropic.Int(1)}}
}

func parseResult(name string, resp *anthropic.Message) (result, error) {
	var sb strings.Builder
	for _, b := range resp.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	text := sb.String()
	s, e := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if s == -1 || e <= s {
		return result{}, fmt.Errorf("JSON が見つからない: %.80s", text)
	}
	var r result
	if err := json.Unmarshal([]byte(text[s:e+1]), &r); err != nil {
		return result{}, err
	}
	r.Name = name
	if !contains(industries, r.Industry) {
		r.Industry = "" // 選択肢外は採用しない
	}
	if !strings.HasPrefix(r.WebsiteURL, "http://") && !strings.HasPrefix(r.WebsiteURL, "https://") {
		r.WebsiteURL = ""
	}
	return r, nil
}

// apply は調べた結果を CSV の行に反映する。業種と URL は、確認できたものだけ上書きする。
func apply(rows [][]string, cache map[string]result, keepNonIT bool) ([][]string, int) {
	out, dropped := [][]string{rows[0]}, 0
	for _, r := range rows[1:] {
		res, ok := cache[r[0]]
		if ok && !res.IsIT && !keepNonIT {
			dropped++
			continue
		}
		row := append([]string(nil), r...)
		if ok {
			if res.Industry != "" {
				row[1] = res.Industry
			}
			if res.WebsiteURL != "" {
				row[3] = res.WebsiteURL
			}
			if res.Description != "" {
				row[5] = res.Description
			}
		}
		out = append(out, row)
	}
	return out, dropped
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func readCSV(path string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		log.Fatalf("CSV を読めません: %v", err)
	}
	// 列は name,industry,segment,website_url,publish,description。description が無い CSV は空で補う。
	for i := range rows {
		for len(rows[i]) < 6 {
			rows[i] = append(rows[i], "")
		}
	}
	rows[0][5] = "description"
	return rows
}

func writeCSV(path string, rows [][]string) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		log.Fatal(err)
	}
}

func loadCache(path string) map[string]result {
	cache := map[string]result{}
	f, err := os.Open(path)
	if err != nil {
		return cache
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r result
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Name != "" {
			cache[r.Name] = r
		}
	}
	return cache
}
