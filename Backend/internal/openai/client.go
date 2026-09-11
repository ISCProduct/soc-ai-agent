package openai

import (
	"errors"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

// UsageHook はAPIコール成功時に呼ばれるコールバック。
// model: 使用モデル名, promptTokens: 入力トークン数, completionTokens: 出力トークン数
type UsageHook func(model string, promptTokens, completionTokens int)

// Client は go-openai SDK をラップします。
type Client struct {
	c            *openai.Client
	DefaultModel string
	apiKey       string
	baseURL      string
	OnUsage      UsageHook // オプション: コール成功時にトークン使用量を通知
}

var (
	// openaiPromptCacheHitRate は以前は prometheus メトリクスでしたが、CI の依存管理簡素化のため無効化しています。
	// 将来必要なら prometheus を再導入してください。
	openaiPromptCacheHitRate interface{} = nil
)

func NewFromEnv(optionalModel string) (*Client, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}

	model := optionalModel
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	cli := openai.NewClient(key)
	return &Client{c: cli, DefaultModel: model, apiKey: key, baseURL: "https://api.openai.com/v1"}, nil
}

// NewWithBaseURL はテスト用コンストラクタ。baseURL を差し替えてモックサーバーを利用できる。
func NewWithBaseURL(baseURL, model string) *Client {
	config := openai.DefaultConfig("test-key")
	config.BaseURL = baseURL
	return &Client{c: openai.NewClientWithConfig(config), DefaultModel: model, apiKey: "test-key", baseURL: baseURL}
}

func (cli *Client) BaseURL() string {
	if cli.baseURL != "" {
		return cli.baseURL
	}
	return "https://api.openai.com/v1"
}
