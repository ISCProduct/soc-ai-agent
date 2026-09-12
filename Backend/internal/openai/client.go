package openai

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// defaultOpenAIBaseURL は OpenAI 本家のエンドポイント。
const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// localPlaceholderAPIKey はローカル推論先へ送るダミーキー。
// OpenAI 互換サーバーは値を検証しないが、Authorization ヘッダーが無いと 401 を返す実装が
// あるため空文字にはしない。
const localPlaceholderAPIKey = "local"

// providerLocal / providerOpenAI は AI_*_PROVIDER の受け付ける値。
const (
	providerOpenAI = "openai"
	providerLocal  = "local"
)

// ErrAIUnavailable は AI プロバイダが利用できないことを表す（#1293）。
//
// OPENAI_API_KEY 未設定でもサーバーは起動し、AI を使わない機能
// （企業検索・企業一覧・マッチング・選考管理・求人・キャッシュ済み説明）は動作する。
// AI を必要とする呼び出しだけがこのエラーで縮退する。
// メッセージに env 名や設定値を含めない。学生向けのレスポンスへそのまま流れる経路があるため
// （詳細は起動ログの slog.Warn を見る）。
var ErrAIUnavailable = errors.New("ai provider unavailable")

// UsageHook はAPIコール成功時に呼ばれるコールバック。
// model: 使用モデル名, promptTokens: 入力トークン数, completionTokens: 出力トークン数
type UsageHook func(model string, promptTokens, completionTokens int)

// Client は go-openai SDK をラップします。
//
// テキスト生成 / 埋め込み / 音声の3系統はそれぞれ独立に推論先を切り替えられる（#1293）。
// いずれも OpenAI 互換 API を共通インターフェースとして扱うため、Ollama / llama.cpp /
// vLLM / LocalAI / 社内 brain のどれにも密結合しない。
type Client struct {
	c            *openai.Client // テキスト生成用
	embedC       *openai.Client // 埋め込み用（AI_EMBEDDING_* 未設定時は c と同じ設定）
	DefaultModel string
	// EmbeddingModel は AI_EMBEDDING_MODEL / OPENAI_EMBEDDING_MODEL の解決結果。
	EmbeddingModel string

	apiKey string
	// textKey / audioKey は系統ごとに送る API キー。local はダミー値になる。
	textKey          string
	audioKey         string
	baseURL          string
	embeddingBaseURL string
	audioBaseURL     string

	textProvider      string
	embeddingProvider string
	audioProvider     string

	// *Available は「その系統が呼び出せる状態か」。false の呼び出しは ErrAIUnavailable で縮退する。
	textAvailable      bool
	embeddingAvailable bool
	audioAvailable     bool

	OnUsage UsageHook // オプション: コール成功時にトークン使用量を通知
}

var (
	// openaiPromptCacheHitRate は以前は prometheus メトリクスでしたが、CI の依存管理簡素化のため無効化しています。
	// 将来必要なら prometheus を再導入してください。
	openaiPromptCacheHitRate interface{} = nil
)

// firstNonEmpty は最初の空でない値を返す。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// providerFromEnv は AI_*_PROVIDER を読む。未設定・不正値は openai（現行動作）にフォールバックする。
func providerFromEnv(key string) string {
	switch strings.ToLower(firstNonEmpty(os.Getenv(key))) {
	case providerLocal:
		return providerLocal
	case "", providerOpenAI:
		return providerOpenAI
	default:
		slog.Warn("unknown AI provider, falling back to openai", "env", key, "value", os.Getenv(key))
		return providerOpenAI
	}
}

// resolveBaseURL は系統ごとの base URL を決める。
// local プロバイダは推論先が分からないと成立しないため、base URL 未設定はエラーにする。
// resolveBaseURL は系統ごとの base URL を決める。
// 第2戻り値は「その系統の base URL が env で明示指定されたか」。
// 明示指定なら operator が意図してその宛先へ送っているとみなし、実キーを渡す（プロキシ用途）。
func resolveBaseURL(
	provider, baseURLEnv, fallbackProvider, fallbackURL string, fallbackExplicit bool,
) (baseURL string, explicit bool, err error) {
	if v := firstNonEmpty(os.Getenv(baseURLEnv)); v != "" {
		// openai プロバイダでも base URL の差し替えは許す（プロキシ・Azure 互換ゲートウェイ等）
		lower := strings.ToLower(v)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			// scheme 無しは相対URLとして扱われ、最初の AI 呼び出しで分かりにくく落ちる
			return "", false, fmt.Errorf("%s は http:// または https:// で始まる必要があります: %q", baseURLEnv, v)
		}
		return strings.TrimRight(v, "/"), true, nil
	}

	// 継承元と provider が同じなら base URL も継承する。
	// provider が違う場合に継承すると「テキストはローカル、埋め込みだけ OpenAI」という
	// 明示指定が黙って無視されるため、そのときは継承しない。
	if provider == fallbackProvider && fallbackURL != "" {
		return strings.TrimRight(fallbackURL, "/"), fallbackExplicit, nil
	}
	if provider == providerLocal {
		// local は推論先が分からないと成立しない
		return "", false, fmt.Errorf("%s=%s には %s が必要です", providerEnvName(baseURLEnv), providerLocal, baseURLEnv)
	}
	return defaultOpenAIBaseURL, false, nil
}

// providerEnvName は base URL の env 名から対応する PROVIDER env 名を導く（エラーメッセージ用）。
func providerEnvName(baseURLEnv string) string {
	return strings.TrimSuffix(baseURLEnv, "_BASE_URL") + "_PROVIDER"
}

// NewFromEnv は環境変数からクライアントを構築する。
//
// OPENAI_API_KEY が無くてもエラーにしない（#1293）。OpenAI を使う系統は
// *Available=false となり、その系統の呼び出しだけが ErrAIUnavailable で縮退する。
// local プロバイダはキー不要なので、キー無しでも通常どおり動作する。
// エラーを返すのは設定自体が成立しない場合（local なのに base URL が無い等）だけ。
func NewFromEnv(optionalModel string) (*Client, error) {
	textProvider := providerFromEnv("AI_TEXT_PROVIDER")
	// 埋め込み・音声は provider 未指定ならテキストの provider を継承する。
	// base URL だけ継承して provider が openai のままだと、ローカル推論先へ
	// OpenAI の実キーを送ることになる（#1293 レビュー指摘）。
	embeddingProvider := providerFromEnvOr("AI_EMBEDDING_PROVIDER", textProvider)
	audioProvider := providerFromEnvOr("AI_AUDIO_PROVIDER", textProvider)

	// テキストは継承元が無い（fallbackProvider を空にして継承を発生させない）。
	// local なら base URL が必須、openai なら本家が既定になる。
	textBaseURL, textExplicit, err := resolveBaseURL(
		textProvider, "AI_TEXT_BASE_URL", "", defaultOpenAIBaseURL, false)
	if err != nil {
		return nil, err
	}
	// 埋め込み・音声の base URL 未設定時はテキストと同じ推論先を使う（1台に全部載せる構成が多いため）。
	// provider も継承されるため、テキストを local にすれば埋め込み・音声も
	// 同じローカル推論先・ダミーキーになる。別の推論先に分けたい場合だけ個別に指定する。
	embeddingBaseURL, embeddingExplicit, err := resolveBaseURL(
		embeddingProvider, "AI_EMBEDDING_BASE_URL", textProvider, textBaseURL, textExplicit)
	if err != nil {
		return nil, err
	}
	audioBaseURL, audioExplicit, err := resolveBaseURL(
		audioProvider, "AI_AUDIO_BASE_URL", textProvider, textBaseURL, textExplicit)
	if err != nil {
		return nil, err
	}

	key := firstNonEmpty(os.Getenv("OPENAI_API_KEY"))
	model := firstNonEmpty(optionalModel, os.Getenv("AI_TEXT_MODEL"), os.Getenv("OPENAI_MODEL"), "gpt-4o-mini")
	embeddingModel := firstNonEmpty(
		os.Getenv("AI_EMBEDDING_MODEL"), os.Getenv("OPENAI_EMBEDDING_MODEL"), "text-embedding-3-small")

	cli := &Client{
		DefaultModel:       model,
		EmbeddingModel:     embeddingModel,
		apiKey:             key,
		textKey:            keyFor(textProvider, textBaseURL, textExplicit, key),
		audioKey:           keyFor(audioProvider, audioBaseURL, audioExplicit, key),
		baseURL:            textBaseURL,
		embeddingBaseURL:   embeddingBaseURL,
		audioBaseURL:       audioBaseURL,
		textProvider:       textProvider,
		embeddingProvider:  embeddingProvider,
		audioProvider:      audioProvider,
		textAvailable:      textProvider == providerLocal || key != "",
		embeddingAvailable: embeddingProvider == providerLocal || key != "",
		audioAvailable:     audioProvider == providerLocal || key != "",
	}

	cli.c = newSDKClient(cli.textKey, textBaseURL)
	if embeddingBaseURL == textBaseURL && embeddingProvider == textProvider {
		cli.embedC = cli.c
	} else {
		cli.embedC = newSDKClient(keyFor(embeddingProvider, embeddingBaseURL, embeddingExplicit, key), embeddingBaseURL)
	}

	if !cli.textAvailable || !cli.embeddingAvailable || !cli.audioAvailable {
		slog.Warn("AI provider is degraded: OPENAI_API_KEY が未設定のため一部のAI機能が利用できません",
			"text", cli.textProvider, "text_available", cli.textAvailable,
			"embedding", cli.embeddingProvider, "embedding_available", cli.embeddingAvailable,
			"audio", cli.audioProvider, "audio_available", cli.audioAvailable)
	}
	return cli, nil
}

// keyFor は系統ごとに送る API キーを返す。
//
// 実キーを渡すのは「provider が openai」かつ「OpenAI 本家宛、または base URL が
// env で明示指定されている（= プロキシ・互換ゲートウェイを operator が意図して指定した）」場合だけ。
//
//   - provider=local: 常にダミー。「OPENAI_API_KEY を残したまま provider を local に
//     切り替える」という最も自然な移行手順で実キーが外部へ出るのを防ぐ
//   - 継承した base URL: 明示指定ではないのでダミー（テキストのローカル推論先を
//     埋め込み・音声が継承したケース）
func keyFor(provider, baseURL string, explicit bool, key string) string {
	if provider != providerOpenAI {
		return localPlaceholderAPIKey
	}
	if isOpenAIEndpoint(baseURL) || explicit {
		return key
	}
	return localPlaceholderAPIKey
}

// isOpenAIEndpoint は base URL が OpenAI 本家（https）かを判定する。
// scheme も見るのは、http:// の typo で実キーが平文送信されるのを防ぐため。
func isOpenAIEndpoint(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	if strings.ToLower(u.Scheme) != "https" {
		return false
	}
	return strings.ToLower(u.Hostname()) == "api.openai.com"
}

// providerFromEnvOr は env 未設定時に fallbackProvider を返す providerFromEnv。
func providerFromEnvOr(key, fallbackProvider string) string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fallbackProvider
	}
	return providerFromEnv(key)
}

func newSDKClient(key, baseURL string) *openai.Client {
	config := openai.DefaultConfig(key)
	config.BaseURL = baseURL
	return openai.NewClientWithConfig(config)
}

// NewWithBaseURL はテスト用コンストラクタ。baseURL を差し替えてモックサーバーを利用できる。
func NewWithBaseURL(baseURL, model string) *Client {
	cli := &Client{
		DefaultModel:       model,
		EmbeddingModel:     "text-embedding-3-small",
		apiKey:             "test-key",
		textKey:            "test-key",
		audioKey:           "test-key",
		baseURL:            baseURL,
		embeddingBaseURL:   baseURL,
		audioBaseURL:       baseURL,
		textProvider:       providerOpenAI,
		embeddingProvider:  providerOpenAI,
		audioProvider:      providerOpenAI,
		textAvailable:      true,
		embeddingAvailable: true,
		audioAvailable:     true,
	}
	cli.c = newSDKClient("test-key", baseURL)
	cli.embedC = cli.c
	return cli
}

// BaseURL はテキスト生成の base URL を返す。
func (cli *Client) BaseURL() string {
	if cli.baseURL != "" {
		return cli.baseURL
	}
	return defaultOpenAIBaseURL
}

// EmbeddingBaseURL は埋め込みの base URL を返す。
func (cli *Client) EmbeddingBaseURL() string {
	if cli.embeddingBaseURL != "" {
		return cli.embeddingBaseURL
	}
	return cli.BaseURL()
}

// AudioBaseURL は音声(STT/TTS)の base URL を返す。
func (cli *Client) AudioBaseURL() string {
	if cli.audioBaseURL != "" {
		return cli.audioBaseURL
	}
	return cli.BaseURL()
}

// Providers は系統ごとのプロバイダ名を返す（起動ログ・管理画面用）。
func (cli *Client) Providers() (text, embedding, audio string) {
	if cli == nil {
		return "", "", ""
	}
	return cli.textProvider, cli.embeddingProvider, cli.audioProvider
}

// Degraded は縮退している系統があるかを返す。
func (cli *Client) Degraded() bool {
	if cli == nil {
		return true
	}
	return !cli.textAvailable || !cli.embeddingAvailable || !cli.audioAvailable
}

// ensureText / ensureEmbedding / ensureAudio は各系統が呼び出せる状態かを確認する。
// nil クライアント・SDK未初期化も同じエラーに寄せて、呼び出し側が縮退を1種類のエラーで扱えるようにする。
func (cli *Client) ensureText() error {
	if cli == nil || cli.c == nil || !cli.textAvailable {
		return ErrAIUnavailable
	}
	return nil
}

func (cli *Client) ensureEmbedding() error {
	if cli == nil || cli.embedC == nil || !cli.embeddingAvailable {
		return ErrAIUnavailable
	}
	return nil
}

func (cli *Client) ensureAudio() error {
	if cli == nil || !cli.audioAvailable {
		return ErrAIUnavailable
	}
	return nil
}

// ensureRealtime は Realtime API が呼べる状態かを確認する。
//
// Realtime は OpenAI 固有の API で OpenAI 互換サーバーには存在しないため、
// 音声系統をローカルへ向けている構成では縮退させる（ダミーキーを本家へ送っても 401 になる）。
// 実キーが必要なので apiKey を直接見る。
func (cli *Client) ensureRealtime() error {
	if cli == nil || cli.audioProvider != providerOpenAI || cli.apiKey == "" {
		return ErrAIUnavailable
	}
	return nil
}
