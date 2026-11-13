package costs

import (
	"Backend/internal/models"
	openaiPkg "Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/safego"
	"Backend/internal/services/shared"
	"Backend/internal/usagectx"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// モデル別単価テーブル (USD per 1M tokens)。
//
// 値は OpenAI の公式の価格ページ（https://developers.openai.com/api/docs/pricing）と
// 各モデルのページ（/api/docs/models/<model>）の標準料金（Batch・Flex ではない）で、2026-11-11 に確認した（#2004）。
// 提供終了（Shut down）のモデルも、過去の記録や設定の残りを正しく計算できるよう、モデルのページの単価で残している。
// 価格改定のときは、ここを直すか AI_MODEL_PRICING_JSON で上書きする。
var modelPricing = map[string][2]float64{
	// input, output per 1M tokens
	"gpt-5":                  {1.25, 10.00},
	"gpt-5.2":                {1.75, 14.00},
	"gpt-4.1":                {2.00, 8.00},
	"gpt-4.1-mini":           {0.40, 1.60},
	"gpt-4o":                 {2.50, 10.00},
	"gpt-4o-mini":            {0.15, 0.60},
	"gpt-4-turbo":            {10.00, 30.00}, // 提供終了
	"gpt-4":                  {30.00, 60.00}, // 提供終了
	"gpt-3.5-turbo":          {0.50, 1.50},   // 提供終了
	"o1":                     {15.00, 60.00}, // 提供終了
	"o1-mini":                {1.10, 4.40},   // 提供終了
	"o3":                     {2.00, 8.00},
	"o3-mini":                {1.10, 4.40}, // 提供終了
	"text-embedding-3-small": {0.02, 0.02},
	"text-embedding-3-large": {0.13, 0.13},
	// 音声系のトークン単価。STT は通常、分単位（sttPricingPerMinute）で計算するので、
	// ここはトークンで記録された場合のためのもの。
	"gpt-4o-transcribe":      {2.50, 10.00},
	"gpt-4o-mini-transcribe": {1.25, 5.00},
	// Realtime はテキストのトークン単価（音声は $40 / $80 で別）。音声面接の費用は
	// realtime_usage_service が音声の単価で別に計算している。提供終了。
	"gpt-4o-realtime-preview": {5.00, 20.00},
	// Web検索系。検索結果が固定トークンとして課金されるため入力トークンが
	// 桁違いに大きい（実測で1コール平均3万トークン）。
	// gpt-4o-mini-search-preview は最長一致が無いと gpt-4o 単価に当たり得るので
	// 明示が必要。search-preview の2つは提供終了。gpt-5-search-api は gpt-5 と同じ単価。
	"gpt-4o-search-preview":      {2.50, 10.00},
	"gpt-4o-mini-search-preview": {0.15, 0.60},
	"gpt-5-search-api":           {1.25, 10.00},
}

// 単価の根拠（#2004）。見た目では推定値か実価格かが区別できなかったので、表を分けて持つ。
const (
	// PriceBasisListed は modelPricing の単価（公開価格、または AI_MODEL_PRICING_JSON で運用側が上書きした単価）。
	// 一致したキーがモデル名そのもの・日付付きの版（-20…）・":" 付きの派生名のときだけ。
	// 音声の単価では、公開価格の表に一致した単価と、STT_COST_PER_MINUTE_USD / TTS_COST_PER_1M_CHARS_USD で上書きした単価（#2224）。
	PriceBasisListed = "listed"
	// PriceBasisEstimated は estimatedPricing の単価（実価格を確認していない推定値）。
	// 音声の単価では、表に無いモデルに既定値を当てた単価（#2224）。
	PriceBasisEstimated = "estimated"
	// PriceBasisInherited は、単価表に無いモデルが前方一致で別モデルの単価に当たったもの
	// （例: o3-pro → o3、gpt-4o-mini-tts → gpt-4o-mini）。モデル自身の単価ではない（#2151 のレビュー）。
	// 上位版（o3-pro など）は、当たった先のモデルより高いことがあり、少なく見積もりうる。バッジと警告で気づけるようにしている。
	PriceBasisInherited = "inherited"
	// PriceBasisUnknown は単価表に無いモデル。少なく見積もらないよう gpt-4o の単価で計算している。
	PriceBasisUnknown = "unknown"
)

// estimatedPricing は実価格を確認していない推定値（USD per 1M tokens）。
// 実価格を確認したら modelPricing へ移す。運用で先に直すなら AI_MODEL_PRICING_JSON で上書きする
// （上書きしたモデルは運用側が確認した単価として listed 扱いになり、ここから外れる）。
var estimatedPricing = map[string][2]float64{
	// gpt-4-turbo のプレビュー版（3つとも）。公式の価格ページ・モデルのページに記載が無いので、gpt-4-turbo の単価で推定する。
	// isSameModel の「- と4桁の数字」の規則では gpt-4 の版（$30 / $60）に当たってしまうため、明示して区別する。
	"gpt-4-1106-preview":        {10.00, 30.00},
	"gpt-4-0125-preview":        {10.00, 30.00},
	"gpt-4-1106-vision-preview": {10.00, 30.00},
}

// unknownFallbackModel は単価表に無いモデルに使う単価のモデル（過小評価しない）。
// AI_MODEL_PRICING_JSON で gpt-4o を上書きしたら、その値に追従する（#2151 のレビュー）。
const unknownFallbackModel = "gpt-4o"

// pricingWarned は、推定値・単価未登録で計算したことを、モデルごとに1回だけログに出すための記録。
var pricingWarned sync.Map

// pricingMatch は lookupPricing の結果。
type pricingMatch struct {
	prices [2]float64
	basis  string
	key    string // 一致した単価表のキー（未登録なら unknownFallbackModel）
}

// lookupPricing はモデル名から単価と、その根拠を返す。
//
// バージョン付きモデル名（gpt-4o-2024-08-06 等）に対応するため前方一致を使うが、
// 必ず**最長一致**を取る。
//
// 以前は map を range して最初に一致したもので break していた。Go の map の
// 反復順序はランダムなので、"gpt-4o-mini" が "gpt-4o" の前方一致に当たると
// 16.7倍高い gpt-4o 単価で記録されていた。しかも実行ごとに結果が変わる。
// 実測（本番相当DB, 2026-08-13以降）では gpt-4o-mini の 892万入力トークンが
// $26.32 として記録されており、正しい mini 単価なら $1.79 だった。
// コスト削減の判断材料が15倍近く狂っていたことになる。
//
// 単価の選び方は前方一致のまま変えないが、一致したのが別モデル（o3-pro → o3 など）なら、
// 根拠を inherited にして、公開価格と区別する。
func lookupPricing(model string) pricingMatch {
	lower := strings.ToLower(strings.TrimSpace(model))
	m := pricingMatch{prices: modelPricing[unknownFallbackModel], basis: PriceBasisUnknown, key: unknownFallbackModel}
	best := ""
	for _, table := range []struct {
		prices map[string][2]float64
		basis  string
	}{{modelPricing, PriceBasisListed}, {estimatedPricing, PriceBasisEstimated}} {
		for k, p := range table.prices {
			if lower != k && !strings.HasPrefix(lower, k+"-") && !strings.HasPrefix(lower, k+":") {
				continue
			}
			// 同じ長さなら先に見た表（modelPricing = listed）を優先する
			if len(k) > len(best) {
				best = k
				m = pricingMatch{prices: p, basis: table.basis, key: k}
			}
		}
	}
	// 公開価格・推定値のどちらでも、一致したのが別モデルなら inherited にする（推定値の表の
	// gpt-4-1106-preview に gpt-4-1106-preview-vision が当たる場合も、モデル自身の単価ではない。#2151 のレビュー）
	if m.basis != PriceBasisUnknown && !isSameModel(lower, m.key) {
		m.basis = PriceBasisInherited
	}
	return m
}

// isSameModel は、単価表のキーとモデル名が同じモデルかを返す。同じモデルとみなすのは、
//   - 名前がそのまま一致する
//   - キーの後ろが「- と4桁の数字」で始まる版（gpt-4o-2024-08-06、gpt-4-0613、gpt-3.5-turbo-1106-preview）
//   - キーの後ろが ":" の派生名
//
// 4桁にしているのは、gpt-4-32k のような別モデル（数字が2桁）や、o3-mini・gpt-4-turbo のような
// 別モデル（数字で始まらない）を、同じモデルの版と取り違えないため（#2151 のレビュー）。
func isSameModel(lowerModel, key string) bool {
	if lowerModel == key || strings.HasPrefix(lowerModel, key+":") {
		return true
	}
	rest, ok := strings.CutPrefix(lowerModel, key+"-")
	if !ok || len(rest) < 4 {
		return false
	}
	for _, c := range rest[:4] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(rest) == 4 || rest[4] == '-'
}

// PriceBasis はモデル名から単価の根拠（PriceBasisListed / Estimated / Inherited / Unknown）を返す。
func PriceBasis(model string) string {
	return lookupPricing(model).basis
}

// warnPricingOnce は、公開価格以外の単価で計算したことを、モデルごとに1回だけログに出す。
// 黙って別の単価で集計されると、画面の金額と請求額のずれに気づけない（#2004）。
func warnPricingOnce(model string, m pricingMatch) {
	if m.basis == PriceBasisListed {
		return
	}
	// 表の検索と同じく小文字にそろえる（GPT-5.2 と gpt-5.2 で2回出さない。#2151 のレビュー）
	name := strings.ToLower(strings.TrimSpace(model))
	if _, loaded := pricingWarned.LoadOrStore(m.basis+":"+name, struct{}{}); loaded {
		return
	}
	switch m.basis {
	case PriceBasisEstimated:
		log.Printf("[APICost] %q は推定単価で計算しています（実価格を確認したら単価表へ移すか %s で上書きしてください）", name, modelPricingOverrideEnv)
	case PriceBasisInherited:
		log.Printf("[APICost] %q は単価表に無いため、別モデル %q の単価（%s）で計算しています（%s で単価を登録してください）", name, m.key, tableNameOf(m.key), modelPricingOverrideEnv)
	case PriceBasisUnknown:
		log.Printf("[APICost] %q は単価表に無いため %s の単価で計算しています（%s で単価を登録してください）", name, unknownFallbackModel, modelPricingOverrideEnv)
	}
}

// tableNameOf は、キーがどちらの表の単価かをログ用に返す。
func tableNameOf(key string) string {
	if _, ok := modelPricing[key]; ok {
		return "公開価格"
	}
	return "推定値"
}

// isAudioPricedModel は、トークン単価ではなく音声の単価（分・文字数）で課金するモデルかを返す。
// 内訳でこれらに「単価未登録」などのバッジを付けないため（#2151 のレビュー）。
// 名前で判定するので、音声のモデル名の付け方（transcribe / whisper / tts）に頼っている。
func isAudioPricedModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "transcribe") || strings.Contains(lower, "whisper") ||
		strings.HasPrefix(lower, "tts") || strings.Contains(lower, "-tts") // gpt-4o-mini-tts など
}

// 音声経路の単価（#1294 DesignDoc §3.4）。
//
// STT は分単位、TTS は文字単位で課金され、トークン単価の表では表せない。
//
// STT の表は 2026-11-11、TTS の既定値（tts-1 の $15/100万文字）は 2026-11-13 に確認した公開価格に基づく。
// 価格改定が入っている可能性があるため、実請求額との突き合わせで必ず確認すること（#1193 では旧価格が3年ぶん残っていた）。
// 生の使用量（audio_seconds / characters）を保存しているので、単価が違っていても
// 後から再計算できる。
//
// 単価の根拠（api_call_logs.price_basis）は、トークン単価と同じ考え方で付ける（#2224）。
// 公開価格の表に一致した単価と、env で運用側が指定した単価（AI_MODEL_PRICING_JSON の上書きと同じく、
// 運用側が確認した単価として扱う）は listed、表に無いモデルに既定値を当てた単価は estimated。

// sttPricingPerMinute は STT のモデル別の分単価（USD per minute）。
//
// 以前はモデルに関係なく $0.006/分で計算していたが、既定のモデル gpt-transcribe は $0.0045/分、
// gpt-4o-mini-transcribe は $0.003/分で、文字起こしの料金を多めに記録していた（#2173）。
// 値は OpenAI の公式の価格ページ（https://developers.openai.com/api/docs/pricing）の標準料金で、
// 2026-11-11 に確認した（gpt-4o-transcribe-diarize は 2026-11-13 に確認して追加。#2224）。
// whisper-1・gpt-4o-transcribe・gpt-4o-mini-transcribe は 2027-02-26 に
// 提供終了の予定だが、過去の記録や設定の残りを正しく計算できるよう残す。
// 価格改定のときは、ここを直すか STT_COST_PER_MINUTE_USD で上書きする（上書きは全モデル一律の単価になる）。
// STT の評価ハーネス（internal/sttbench）も PublishedSTTCostPerMinuteUSD でこの表を引く。表を写すと片方だけ古くなるため。
var sttPricingPerMinute = map[string]float64{
	"gpt-transcribe":            0.0045,
	"gpt-4o-transcribe":         0.006,
	"gpt-4o-transcribe-diarize": 0.006,
	"gpt-4o-mini-transcribe":    0.003,
	"whisper-1":                 0.006,
}

// sttPerMinuteEstimated は、分単価が公式の価格ページで「目安（est.）」として載っているモデル。
// これらはトークン課金（入力・出力のトークン単価）で、分単価は公式の概算にすぎない。出力のトークン数で
// 実額が変わるので、根拠は listed ではなく estimated にする（#2229 のレビュー、2026-11-11 の価格ページで確認）。
// 分単価そのものが公開価格なのは gpt-transcribe と whisper-1 だけ。
var sttPerMinuteEstimated = map[string]bool{
	"gpt-4o-transcribe":         true,
	"gpt-4o-transcribe-diarize": true,
	"gpt-4o-mini-transcribe":    true,
}

// STTPerMinuteIsEstimate は、表の分単価が公式の概算（est.）かを返す。表に無いモデルは false。
// 評価ハーネス（internal/sttbench）が、見積もりの根拠を「公開単価」と「公開単価の目安」に分けるために使う。
func STTPerMinuteIsEstimate(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	for k := range sttPerMinuteEstimated {
		if isSameModel(lower, k) {
			return true
		}
	}
	return false
}

// sttFallbackCostPerMinuteUSD は、表に無い STT モデルに使う分単価。
// 表の中で最も高い単価（以前の一律の既定値と同じ）にして、少なく見積もらないようにする。
const sttFallbackCostPerMinuteUSD = 0.006

// sttCostOverrideEnv は、STT の分単価をモデルに関係なく上書きする env。
// 以前からある設定なので、設定されていれば今までどおり表より優先する。
const sttCostOverrideEnv = "STT_COST_PER_MINUTE_USD"

// sttPricingWarned は、表に無い STT モデルで計算したことを、モデルごとに1回だけログに出すための記録。
var sttPricingWarned sync.Map

// PublishedSTTCostPerMinuteUSD は、STT のモデルの公開価格（分単価）を表から返す。表に無ければ false。
// env の上書きは見ない。評価ハーネス（internal/sttbench）が「公開価格での見積もり」を出すために使う。
//
// モデル名の照合はトークン単価表と同じく isSameModel を使い、日付付きの版名
// （gpt-4o-mini-transcribe-2025-12-15 など）も同じモデルとみなす。前方一致にしないのは、
// gpt-transcribe-pro のような表に無い別モデルに、別のモデルの単価を黙って当てないため。
func PublishedSTTCostPerMinuteUSD(model string) (float64, bool) {
	lower := strings.ToLower(strings.TrimSpace(model))
	for k, p := range sttPricingPerMinute {
		// キー同士は isSameModel で互いに一致しないので、一致するキーは高々1つ（反復順に依存しない）
		if isSameModel(lower, k) {
			return p, true
		}
	}
	return 0, false
}

// sttCostPerMinuteUSD は、STT のモデル名から分単価と、その根拠を返す。
// STT_COST_PER_MINUTE_USD の上書きと、分単価が公開価格のモデルは listed。分単価が公式の概算のモデル
// （sttPerMinuteEstimated）と、表に無く既定値で計算したときは estimated（#2224、#2229 のレビュー）。
func sttCostPerMinuteUSD(model string) (float64, string) {
	if v, ok := audioRateOverride(sttCostOverrideEnv); ok {
		return v, PriceBasisListed
	}
	if p, ok := PublishedSTTCostPerMinuteUSD(model); ok {
		if STTPerMinuteIsEstimate(model) {
			return p, PriceBasisEstimated
		}
		return p, PriceBasisListed
	}
	// 表の検索と同じく小文字にそろえる（大文字・小文字の違いで2回出さない）
	lower := strings.ToLower(strings.TrimSpace(model))
	if _, loaded := sttPricingWarned.LoadOrStore(lower, struct{}{}); !loaded {
		log.Printf("[APICost] 音声認識のモデル %q は分単価の表に無いため $%g/分で計算しています（%s で上書きするか、表に単価を登録してください）",
			lower, sttFallbackCostPerMinuteUSD, sttCostOverrideEnv)
	}
	return sttFallbackCostPerMinuteUSD, PriceBasisEstimated
}

// ttsCostOverrideEnv は、TTS の文字単価をモデルに関係なく上書きする env。
const ttsCostOverrideEnv = "TTS_COST_PER_1M_CHARS_USD"

// ttsDefaultCostPer1MCharsUSD は TTS の既定の文字単価（USD per 1M characters）。
// tts-1 の公開価格（$15.00 / 1M characters）で、2026-11-13 に公式の価格ページで確認した。
const ttsDefaultCostPer1MCharsUSD = 15.0

// ttsDefaultPricedModel は、既定の文字単価が公開価格そのものであるモデル（今の既定の TTS モデル）。
const ttsDefaultPricedModel = "tts-1"

// ttsCostPer1MCharsUSD は、TTS の文字単価と、その根拠を返す。単価はモデルに関係なく同じ（以前どおり）。
//
// 根拠は、TTS_COST_PER_1M_CHARS_USD の上書きと tts-1 は listed。それ以外のモデルは estimated にする。
// $15 は tts-1 の公開価格で、ほかのモデルの単価ではない（tts-1-hd は $30 / 100万文字、
// gpt-4o-mini-tts はトークン課金。どちらも 2026-11-13 の公式の価格ページ）。印を付けないと、
// OPENAI_TTS_MODEL を変えたときに少なく見積もっていても管理画面で気づけない（#2224）。
func ttsCostPer1MCharsUSD(model string) (float64, string) {
	if v, ok := audioRateOverride(ttsCostOverrideEnv); ok {
		return v, PriceBasisListed
	}
	if isSameModel(strings.ToLower(strings.TrimSpace(model)), ttsDefaultPricedModel) {
		return ttsDefaultCostPer1MCharsUSD, PriceBasisListed
	}
	return ttsDefaultCostPer1MCharsUSD, PriceBasisEstimated
}

// audioRateOverride は、音声の単価を上書きする env を読む。未設定・解釈できない値は無視する
// （以前の shared.GetFloatEnv と同じ扱い。計測の設定ミスでサーバーを止めない）。
func audioRateOverride(env string) (float64, bool) {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// webSearchCostPerCallUSD は web_search ツール1コールあたりの料金。
//
// このツールはトークンとは別に1コール単位で課金される。単価表はトークンしか
// 持っていなかったため、検索コストの大半が記録から抜けていた。実測では
// 2026-08 の web_search 772 コールぶん、$7.72 が api_call_logs に現れていない。
//
// 検索1回の内訳は「ツール料 $0.010 + 固定8,000入力トークン」で、ツール料が
// 85% を占める。ここが抜けていると、モデルを安くしても下がらないコストが
// 見えず、削減の判断材料にならない。
//
// 既定値は 2026-09 時点の公開価格（$10 / 1,000 コール）。価格改定に備えて
// 生の回数を api_call_logs.web_search_calls に残してあるので、後から再計算できる。
func webSearchCostPerCallUSD() float64 {
	return shared.GetFloatEnv("WEB_SEARCH_COST_PER_CALL_USD", 0.010)
}

// calculateAudioCost は音声経路のコストを返す。ローカル推論は 0。
func calculateAudioCost(provider, model string, audioSeconds float64, characters int) float64 {
	cost, _ := audioCost(provider, model, audioSeconds, characters)
	return cost
}

// audioCost は、音声の単価での料金と、使った単価の根拠を同じ計算で返す（#2224）。
// STT の分単価は model で引く（#2173）。TTS の文字単価はモデルに関係なく同じ。
// ローカル推論は単価を使わないので、料金は 0、根拠は空。
//
// 秒から分へは按分で換算し、切り上げない（30秒なら0.5分ぶん）。実際の課金が分単位の丸めかどうかは
// 価格表からは分からないため、#2173 以前と同じ近似のまま。生の秒数（audio_seconds）を残しているので、
// 丸めると分かったら後から再計算できる。
//
// 秒数と文字数の両方がある行は、STT と TTS の根拠のうち注意が必要なほうを返す。
func audioCost(provider, model string, audioSeconds float64, characters int) (float64, string) {
	if provider != "" && !strings.EqualFold(provider, "openai") {
		return 0, ""
	}
	cost := 0.0
	basis := ""
	if audioSeconds > 0 {
		rate, b := sttCostPerMinuteUSD(model)
		cost += audioSeconds / 60.0 * rate
		basis = b
	}
	if characters > 0 {
		rate, b := ttsCostPer1MCharsUSD(model)
		cost += float64(characters) / 1_000_000.0 * rate
		// worsePriceBasis は空を listed と同じ順位で扱うので、STT の根拠が無いときはそのまま使う
		if basis == "" {
			basis = b
		} else {
			basis = worsePriceBasis(basis, b)
		}
	}
	return cost, basis
}

// modelPricingOverrideEnv は単価表を再ビルドなしに差し替える env（#1294 DesignDoc §3.4）。
//
//	AI_MODEL_PRICING_JSON={"gpt-4o":[2.5,10.0],"gpt-5.2":[1.25,5.0]}
//
// 価格改定は不定期に起きるのに、テーブルは定数なのでデプロイを待つことになる。
// 実測値が狂うと「ローカル化でいくら減ったか」の判断材料そのものが狂うため、
// 運用側で先に直せるようにする。
const modelPricingOverrideEnv = "AI_MODEL_PRICING_JSON"

func init() {
	applyModelPricingOverride(os.Getenv(modelPricingOverrideEnv))
}

// applyModelPricingOverride は JSON の単価をテーブルへ上書きする。
// 不正な値は無視してログに残す。計測の設定ミスでサーバーを起動不能にしない。
func applyModelPricingOverride(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	var override map[string][]float64
	if err := json.Unmarshal([]byte(raw), &override); err != nil {
		log.Printf("[APICost] %s を解釈できませんでした（既定の単価表を使います）: %v", modelPricingOverrideEnv, err)
		return
	}
	for model, rates := range override {
		key := strings.ToLower(strings.TrimSpace(model))
		// 入力・出力の両方が0の単価は受け付けない。0円で黙って集計されると、請求額とのずれに気づけない（#2151 のレビュー）
		if key == "" || len(rates) != 2 || rates[0] < 0 || rates[1] < 0 || (rates[0] == 0 && rates[1] == 0) {
			log.Printf("[APICost] %s の %q を無視しました（[入力単価, 出力単価] の非負の2要素で、両方0は不可）", modelPricingOverrideEnv, model)
			continue
		}
		modelPricing[key] = [2]float64{rates[0], rates[1]}
		// 運用側が単価を指定したので、推定値ではなくなる（#2004）
		delete(estimatedPricing, key)
	}
}

// calculateCost は入力/出力トークン数とモデル名からUSDコストを計算する。
//
// provider が openai 以外のときは 0 を返す。ローカル推論は無料であり、
// かつローカルのモデル名（gpt-oss-20b 等）は単価表に無いため
// 未知モデルの既定（gpt-4o 単価）が架空のコストとして記録されてしまう（#1293）。
func calculateCost(provider, model string, promptTokens, completionTokens int) float64 {
	cost, _ := tokenCost(provider, model, promptTokens, completionTokens)
	return cost
}

// tokenCost は、トークン単価での料金と、使った単価の根拠を同じ計算で返す（#2004）。
// 単価表を使わなかったとき（ローカル推論）は、根拠は空。
func tokenCost(provider, model string, promptTokens, completionTokens int) (float64, string) {
	if provider != "" && !strings.EqualFold(provider, "openai") {
		return 0, ""
	}
	m := lookupPricing(model)
	warnPricingOnce(model, m)

	inputCost := float64(promptTokens) * m.prices[0] / 1_000_000
	outputCost := float64(completionTokens) * m.prices[1] / 1_000_000
	return inputCost + outputCost, m.basis
}

// EstimateCostUSD は単価表を使ってトークン数から料金を見積もる。
//
// 評価ハーネス（cmd/aibench）が1件あたりのコストを出すために使う。
// 単価表をハーネス側へ写すと値上げのたびに2箇所を直すことになり、
// 片方だけ古いまま「安くなった」と誤判定する（#1193 で実際に起きた）。
func EstimateCostUSD(model string, promptTokens, completionTokens int) float64 {
	return calculateCost("openai", model, promptTokens, completionTokens)
}

// APICostService はAPIコスト記録・集計・月次閾値アラートを担当する
type APICostService struct {
	repo              *repositories.APICallLogRepository
	alertThresholdUSD float64 // 月次閾値（既定 $40）

	mu               sync.Mutex
	lastAlertMonthID string // "2006-01" UTC。同一月は1回のみ通知

	// テスト用フック（nil なら本番実装を使う）
	totalCostFn      func() (float64, error)
	modelBreakdownFn func(since time.Time) ([]ModelCostSummary, error)
	notifySlackFn    func(text string) error
	notifyDiscordFn  func(content string) error
}

func NewAPICostService(repo *repositories.APICallLogRepository) *APICostService {
	// OPENAI_COST_ALERT_THRESHOLD_USD を優先、未設定時は旧 API_COST_ALERT_THRESHOLD_USD、最終既定 $40
	threshold := shared.GetFloatEnv("OPENAI_COST_ALERT_THRESHOLD_USD", 0)
	if threshold <= 0 {
		threshold = shared.GetFloatEnv("API_COST_ALERT_THRESHOLD_USD", 40)
	}
	return &APICostService{repo: repo, alertThresholdUSD: threshold}
}

// usageCostUSD は1コールぶんの課金額を返す。
//
// トークン・音声・ツール料の3つを1か所にまとめる。以前はトークンだけを見ており、
// web_search のツール料が丸ごと抜けていた。
func usageCostUSD(u openaiPkg.Usage) float64 {
	cost, _ := usageCostAndBasis(u)
	return cost
}

// usageCostAndBasis は、1コールぶんの課金額と、単価の根拠を同じ計算で返す（#2004）。
// 根拠は記録する時点の単価で決まり、api_call_logs.price_basis に残す。トークン単価表を使った行と、
// 音声の単価（分・文字数）で計算した行（#2224）に付け、ローカル推論の行・トークンが0の行（web_search のツール料だけの行）は空にする。
// 料金の計算と根拠の判定を別々に書くと、片方だけ変わったときに根拠が料金の計算方法と食い違うので、まとめている。
func usageCostAndBasis(u openaiPkg.Usage) (float64, string) {
	// 音声経路（STT/TTS）はトークンが返らない。秒数・文字数から計算する。
	// 音声ではトークン単価表を引かない（引くと「単価表に無い」という実際と違う警告が出る。#2151 のレビュー）
	var cost float64
	basis := ""
	switch {
	case u.AudioSeconds > 0 || u.Characters > 0:
		cost, basis = audioCost(u.Provider, u.Model, u.AudioSeconds, u.Characters)
	case u.PromptTokens+u.CompletionTokens > 0:
		cost, basis = tokenCost(u.Provider, u.Model, u.PromptTokens, u.CompletionTokens)
	}
	// web_search はトークンとは別に1コール単位で課金される。
	// ローカル推論(provider != openai)には存在しないので加算しない。
	if u.WebSearchCalls > 0 && (u.Provider == "" || strings.EqualFold(u.Provider, "openai")) {
		cost += float64(u.WebSearchCalls) * webSearchCostPerCallUSD()
	}
	return cost, basis
}

// LogUsage は非同期でAPIコールログをDBに記録する。
//
// provider / via_fallback を残すのは、この表を「OpenAI への課金額」として
// 使えるようにするため（#1293）。フォールバックの USD 上限は via_fallback だけを
// 集計するので、通常の OpenAI 利用（企業検索など）が保険の予算を食わない。
func (s *APICostService) LogUsage(u openaiPkg.Usage) {
	safego.Go(func() {
		entry := apiCallLogFromUsage(u, time.Now().UTC())
		if err := s.repo.Create(entry); err != nil {
			log.Printf("[APICost] failed to log: %v", err)
			return
		}
		s.checkAndNotifyThreshold()
	})
}

// apiCallLogFromUsage は使用量を api_call_logs の1行へ変換する。
func apiCallLogFromUsage(u openaiPkg.Usage, calledAt time.Time) *models.APICallLog {
	cost, basis := usageCostAndBasis(u)
	return &models.APICallLog{
		Model:            u.Model,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.PromptTokens + u.CompletionTokens,
		CostUSD:          cost,
		Provider:         u.Provider,
		ViaFallback:      u.ViaFallback,
		CalledAt:         calledAt,
		// 配賦軸（#1294）。Feature が空の経路は unknown として残し、
		// 「まだ計測できていない経路」を件数で追えるようにする。
		Feature:        featureOrUnknown(u.Feature),
		PromptStage:    promptStageOrUnknown(u.PromptStage),
		PromptVersion:  u.PromptVersion,
		PromptABGroup:  u.PromptABGroup,
		UserID:         u.UserID,
		OrganizationID: u.OrganizationID,
		AudioSeconds:   u.AudioSeconds,
		Characters:     u.Characters,
		WebSearchCalls: u.WebSearchCalls,
		PriceBasis:     basis,
		LatencyMs:      u.LatencyMs,
		CacheHit:       u.CacheHit,
	}
}

// checkAndNotifyThreshold は当月（UTC）累計が閾値を超えたら Slack/Discord に通知する。
// 同一月は1回のみ。webhook 未設定時はログのみで落ちない。
func (s *APICostService) checkAndNotifyThreshold() {
	totalFn := s.totalCostFn
	if totalFn == nil {
		totalFn = s.GetCurrentMonthTotal
	}
	total, err := totalFn()
	if err != nil {
		return
	}
	s.NotifyIfMonthCostExceeded(total, time.Now().UTC().Format("2006-01"))
}

// NotifyIfMonthCostExceeded は閾値判定・月次デデュープ・通知送信を行う（テスト可能）。
// 通知を送った場合 true。
func (s *APICostService) NotifyIfMonthCostExceeded(total float64, monthID string) bool {
	// 「超過」は閾値より大きい場合のみ（ちょうど $40 では通知しない）
	if total <= s.alertThresholdUSD {
		return false
	}

	s.mu.Lock()
	if s.lastAlertMonthID == monthID {
		s.mu.Unlock()
		return false
	}
	s.lastAlertMonthID = monthID
	s.mu.Unlock()

	subject := fmt.Sprintf("[SOC AI] OpenAI API月次コスト閾値超過 (%s)", monthID)
	body := fmt.Sprintf(
		"OpenAI API monthly cost exceeded threshold.\nmonth=%s\ntotal_usd=%.4f\nthreshold_usd=%.2f\n",
		monthID, total, s.alertThresholdUSD,
	)
	if breakdown := s.formatModelBreakdown(monthID); breakdown != "" {
		body += "models:\n" + breakdown
	}
	log.Printf("[APICost] ALERT %s", strings.ReplaceAll(body, "\n", " "))

	text := subject + "\n" + body
	s.sendSlackAlert(text)
	s.sendDiscordAlert(text)
	return true
}

func (s *APICostService) formatModelBreakdown(monthID string) string {
	t, err := time.ParseInLocation("2006-01", monthID, time.UTC)
	if err != nil {
		return ""
	}
	since := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	fn := s.modelBreakdownFn
	if fn == nil {
		fn = s.GetModelBreakdown
	}
	rows, err := fn(since)
	if err != nil || len(rows) == 0 {
		return ""
	}
	const maxModels = 5
	var b strings.Builder
	for i, r := range rows {
		if i >= maxModels {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(rows)-maxModels))
			break
		}
		b.WriteString(fmt.Sprintf("  - %s: $%.4f (%d calls)\n", r.Model, r.TotalCostUSD, r.CallCount))
	}
	return b.String()
}

func (s *APICostService) sendSlackAlert(text string) {
	if s.notifySlackFn != nil {
		if err := s.notifySlackFn(text); err != nil {
			log.Printf("[APICost] slack alert failed: %v", err)
		}
		return
	}
	webhook := strings.TrimSpace(os.Getenv("OPENAI_COST_ALERT_SLACK_WEBHOOK_URL"))
	if webhook == "" {
		webhook = strings.TrimSpace(os.Getenv("REALTIME_ALERT_SLACK_WEBHOOK_URL"))
	}
	if webhook == "" {
		return
	}
	if err := postSlackAlert(webhook, text); err != nil {
		log.Printf("[APICost] slack alert failed: %v", err)
	}
}

func (s *APICostService) sendDiscordAlert(content string) {
	if s.notifyDiscordFn != nil {
		if err := s.notifyDiscordFn(content); err != nil {
			log.Printf("[APICost] discord alert failed: %v", err)
		}
		return
	}
	webhook := strings.TrimSpace(os.Getenv("OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL"))
	if webhook == "" {
		return
	}
	if err := postDiscordAlert(webhook, content); err != nil {
		log.Printf("[APICost] discord alert failed: %v", err)
	}
}

type DailyCostSummary struct {
	Date         string  `json:"date"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalTokens  int64   `json:"total_tokens"`
	CallCount    int64   `json:"call_count"`
}

type MonthlyCostSummary struct {
	Month        string  `json:"month"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalTokens  int64   `json:"total_tokens"`
	CallCount    int64   `json:"call_count"`
}

type ModelCostSummary struct {
	Model        string  `json:"model"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalTokens  int64   `json:"total_tokens"`
	CallCount    int64   `json:"call_count"`
	// PriceBasis は単価の根拠（listed / estimated / inherited / unknown, #2004）。画面で「推定」「別モデル」「単価未登録」を示す。
	// 金額が0のモデル（ローカル推論など、単価を使っていないもの）は空にする。
	PriceBasis string `json:"price_basis,omitempty"`
}

func (s *APICostService) GetDailyCosts(nDays int) ([]DailyCostSummary, error) {
	rows, err := s.repo.DailyCosts(nDays)
	if err != nil {
		return nil, err
	}
	result := make([]DailyCostSummary, len(rows))
	for i, r := range rows {
		result[i] = DailyCostSummary{
			Date:         r.Date,
			TotalCostUSD: r.TotalCostUSD,
			TotalTokens:  r.TotalTokens,
			CallCount:    r.CallCount,
		}
	}
	return result, nil
}

func (s *APICostService) GetMonthlyCosts(nMonths int) ([]MonthlyCostSummary, error) {
	rows, err := s.repo.MonthlyCosts(nMonths)
	if err != nil {
		return nil, err
	}
	result := make([]MonthlyCostSummary, len(rows))
	for i, r := range rows {
		result[i] = MonthlyCostSummary{
			Month:        r.Month,
			TotalCostUSD: r.TotalCostUSD,
			TotalTokens:  r.TotalTokens,
			CallCount:    r.CallCount,
		}
	}
	return result, nil
}

func (s *APICostService) GetModelBreakdown(since time.Time) ([]ModelCostSummary, error) {
	rows, err := s.repo.ModelBreakdown(since)
	if err != nil {
		return nil, err
	}
	result := make([]ModelCostSummary, len(rows))
	for i, r := range rows {
		result[i] = ModelCostSummary{
			Model:        r.Model,
			TotalCostUSD: r.TotalCostUSD,
			TotalTokens:  r.TotalTokens,
			CallCount:    r.CallCount,
		}
		result[i].PriceBasis = summaryPriceBasis(r.Model, r.TotalCostUSD, r.RecordedPriceBasis, r.UnrecordedCalls > 0)
	}
	return result, nil
}

// summaryPriceBasis は内訳の1行に付ける単価の根拠を返す。金額が0の行（ローカル推論など、
// 単価を使っていない行）は空にする。単価表に無いローカルのモデル名が「単価未登録」と出ないようにするため。
// 音声の単価（分・文字数）で課金するモデルは、記録した根拠（#2224）だけを出し、トークン単価表では判定しない
// （トークン単価表を使っていないので「単価未登録」などと出さない。#2151 のレビュー）。根拠を記録する前の
// 音声の行は空のまま（今の単価で判定し直すと、過去の行の単価の状態を正しく表さないため）。
//
// 根拠は、記録した時点の値（api_call_logs.price_basis）を使う（#2004）。recorded は、期間内の行のうち
// 一番注意が必要な根拠（unknown > inherited > estimated > listed）。hasUnrecorded は、トークン単価で計算したのに
// 記録の無い行（price_basis の列を足す前の行）があるか。あるときは、それらを今の単価表で判定し、注意が必要なほうを出す。
// どちらも無ければ、トークン単価を使った行が無いので空にする。デプロイ直後の、古い行と新しい行が
// 混ざる期間に、新しい行だけで「公開価格」と表示して古い行の状態を隠さないため。
func summaryPriceBasis(model string, totalCostUSD float64, recorded string, hasUnrecorded bool) string {
	if totalCostUSD <= 0 {
		return ""
	}
	if isAudioPricedModel(model) {
		// 音声の行そのものは ModelBreakdown の hasUnrecorded に数えない（秒数・文字数のある行は除く）。
		// ただし同じモデル名で、トークンで課金された記録の無い行が期間内にあれば、それはトークン単価表で
		// 判定し、注意が必要なほうを出す（音声の行の記録だけで、トークンの行の状態を隠さない。#2229 のレビュー）。
		if hasUnrecorded {
			if recorded == "" {
				return PriceBasis(model)
			}
			return worsePriceBasis(recorded, PriceBasis(model))
		}
		return recorded
	}
	if recorded == "" {
		// 記録が無く、記録の無いトークンの行も無い ＝ トークン単価を使った行が無い（Web 検索のツール料だけなど）
		if !hasUnrecorded {
			return ""
		}
		return PriceBasis(model)
	}
	if hasUnrecorded {
		return worsePriceBasis(recorded, PriceBasis(model))
	}
	return recorded
}

// priceBasisAttention は、注意が必要な順（大きいほど注意が必要）。
var priceBasisAttention = map[string]int{
	PriceBasisListed:    0,
	PriceBasisEstimated: 1,
	PriceBasisInherited: 2,
	PriceBasisUnknown:   3,
}

// worsePriceBasis は、2つの根拠のうち注意が必要なほうを返す。
func worsePriceBasis(a, b string) string {
	if priceBasisAttention[b] > priceBasisAttention[a] {
		return b
	}
	return a
}

// UsageBreakdownSummary は軸ごとの利用量集計（#1294）。
type UsageBreakdownSummary struct {
	Key           string  `json:"key"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	TotalTokens   int64   `json:"total_tokens"`
	CallCount     int64   `json:"call_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	CacheHitCount int64   `json:"cache_hit_count"`
}

// GetUsageBreakdown は指定軸（機能/プロバイダ/モデル/組織）で利用量を集計する（#1294）。
// 未知の軸はエラーにする。呼び出し元のクエリパラメータをそのまま SQL へ渡さないため。
func (s *APICostService) GetUsageBreakdown(ctx context.Context, since time.Time, dim repositories.BreakdownDimension) ([]UsageBreakdownSummary, error) {
	rows, err := s.repo.UsageBreakdown(ctx, since, dim)
	if err != nil {
		return nil, err
	}
	result := make([]UsageBreakdownSummary, len(rows))
	for i, r := range rows {
		result[i] = UsageBreakdownSummary{
			Key:           r.Key,
			TotalCostUSD:  r.TotalCostUSD,
			TotalTokens:   r.TotalTokens,
			CallCount:     r.CallCount,
			AvgLatencyMs:  r.AvgLatencyMs,
			CacheHitCount: r.CacheHitCount,
		}
	}
	return result, nil
}

func (s *APICostService) GetCurrentMonthTotal() (float64, error) {
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return s.repo.TotalCostSince(since)
}

// AlertThresholdUSD は設定済みの月次アラート閾値を返す（テスト/管理用）。
func (s *APICostService) AlertThresholdUSD() float64 {
	return s.alertThresholdUSD
}

// SetAlertHooksForTest は単体テスト用に通知フックと閾値を差し替える。
func (s *APICostService) SetAlertHooksForTest(
	threshold float64,
	slackFn func(text string) error,
	discordFn func(content string) error,
) {
	s.alertThresholdUSD = threshold
	s.notifySlackFn = slackFn
	s.notifyDiscordFn = discordFn
	s.modelBreakdownFn = func(time.Time) ([]ModelCostSummary, error) {
		return nil, nil
	}
}

// featureOrUnknown は空の機能名を 'unknown' に寄せる（#1294）。
// DB 側の既定値と一致させ、集計時に空文字と unknown が分かれないようにする。
func featureOrUnknown(feature string) string {
	if feature == "" {
		return usagectx.FeatureUnknown
	}
	return feature
}

func promptStageOrUnknown(stage string) string {
	if stage == "" {
		return usagectx.FeatureUnknown
	}
	return stage
}
