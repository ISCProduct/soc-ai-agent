package openai

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// FallbackGuard は OpenAI へのフォールバックを許可するかを判定する（#1293）。
//
// ローカル推論先の障害時に無条件で OpenAI へ流すと、障害が続く間ずっと全トラフィックが
// 従量課金へ移り高額請求になる。上限の判定には利用実績（コスト集計）が必要で
// internal/openai から DB を触りたくないため、インターフェースで受け取り
// 実装は internal/services/costs に置く。
type FallbackGuard interface {
	// AllowFallback は許可なら (true, "")、拒否なら (false, 理由) を返す。
	AllowFallback() (bool, string)
}

// fallbackTransport はローカル推論先の失敗時に OpenAI へ1回だけ再試行する RoundTripper。
//
// SDK 経路（chat/embeddings）と生HTTP経路（音声）の両方がこのトランスポートを通るため、
// 呼び出し口ごとに再試行を書かなくても全系統に一様に効く。
type fallbackTransport struct {
	base            http.RoundTripper
	primaryBaseURL  string // ローカル推論先（この URL 宛のときだけ再試行する）
	fallbackBaseURL string // 通常 https://api.openai.com/v1
	fallbackKey     string
	guard           FallbackGuard
	system          string // "text" / "embedding" / "audio"（ログ用）
}

func (t *fallbackTransport) transport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}

func (t *fallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 再送のためにボディをバッファする。音声アップロードもここを通るのでメモリを2重に持つが、
	// 呼び出し側が []byte で音声を保持している時点で同程度のため許容する。
	// ponytail: 全量バッファ。ストリーミングを扱うようになったら io.TeeReader 方式へ。
	body, err := bufferRequestBody(req)
	if err != nil {
		return nil, err
	}

	resp, err := t.transport().RoundTrip(withBody(req, body))
	if !shouldFallback(resp, err) {
		return resp, err
	}

	allowed, reason := true, ""
	if t.guard != nil {
		allowed, reason = t.guard.AllowFallback()
	}
	if !allowed {
		// 縮退運転。ローカルのエラーをそのまま返し、OpenAI は呼ばない
		slog.Warn("openai fallback suppressed", "system", t.system, "reason", reason)
		return resp, err
	}

	fbReq, buildErr := t.rewriteToFallback(req, body)
	if buildErr != nil {
		return resp, err
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	slog.Warn("falling back to openai", "system", t.system, "path", req.URL.Path)
	return t.transport().RoundTrip(fbReq)
}

// shouldFallback は「ローカル側の障害」と判断できる場合だけ true を返す。
// 4xx はリクエスト自体の問題で OpenAI でも同じく失敗するため対象にしない。
func shouldFallback(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	return resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
}

func bufferRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	return body, nil
}

func withBody(req *http.Request, body []byte) *http.Request {
	clone := req.Clone(req.Context())
	if body == nil {
		clone.Body = nil
		return clone
	}
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return clone
}

// rewriteToFallback はローカル宛のリクエストを OpenAI 宛に書き換える。
func (t *fallbackTransport) rewriteToFallback(req *http.Request, body []byte) (*http.Request, error) {
	primary, err := url.Parse(t.primaryBaseURL)
	if err != nil {
		return nil, err
	}
	fallback, err := url.Parse(t.fallbackBaseURL)
	if err != nil {
		return nil, err
	}

	// base URL のパス接頭辞（例 /v1）を差し替える
	suffix := strings.TrimPrefix(req.URL.Path, strings.TrimRight(primary.Path, "/"))
	if suffix == "" {
		suffix = req.URL.Path
	}

	clone := withBody(req, body)
	clone.URL = &url.URL{
		Scheme:   fallback.Scheme,
		Host:     fallback.Host,
		Path:     strings.TrimRight(fallback.Path, "/") + suffix,
		RawQuery: req.URL.RawQuery,
	}
	clone.Host = fallback.Host
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+t.fallbackKey)
	return clone, nil
}

// SetFallbackGuard はフォールバックの可否判定を注入する（#1293）。
// main.go から利用実績ベースのガードを渡す。nil のままなら上限判定なしで再試行する。
func (cli *Client) SetFallbackGuard(guard FallbackGuard) {
	if cli == nil {
		return
	}
	cli.fallbackGuard = guard
	for _, t := range cli.fallbacks {
		t.guard = guard
	}
}

// httpClientFor は系統に応じた *http.Client を返す。
// フォールバックが設定されていない系統では通常のクライアントを返す。
func (cli *Client) httpClientFor(system string, timeout time.Duration) *http.Client {
	if cli == nil {
		return &http.Client{Timeout: timeout}
	}
	if t, ok := cli.fallbacks[system]; ok {
		return &http.Client{Timeout: timeout, Transport: t}
	}
	return &http.Client{Timeout: timeout}
}

// setupFallbacks はローカル推論先を使う系統に、OpenAI へのフォールバックを設定する（#1293）。
//
// 有効化の条件:
//   - OPENAI_FALLBACK_ENABLED が false でない
//   - AI_FALLBACK_PROVIDER が openai（既定）
//   - 実の OPENAI_API_KEY がある（無ければ逃げ先が無い）
//   - その系統の推論先が OpenAI 本家でない（本家から本家へ逃げる意味はない）
//
// 上限判定（日次/月次USD・分間リクエスト数）は SetFallbackGuard で注入される
// ガードが行う。ガード未設定のままでは上限なしで再試行するため、main.go で必ず設定する。
func (cli *Client) setupFallbacks(realKey string) {
	if cli == nil {
		return
	}
	if !fallbackEnabledFromEnv() || realKey == "" {
		return
	}
	if provider := firstNonEmpty(os.Getenv("AI_FALLBACK_PROVIDER"), providerOpenAI); strings.ToLower(provider) != providerOpenAI {
		return
	}

	// フォールバック先は既定で OpenAI 本家。プロキシや互換ゲートウェイを経由させたい場合、
	// およびテストのために env で差し替えられる。
	fallbackBaseURL := strings.TrimRight(firstNonEmpty(os.Getenv("AI_FALLBACK_BASE_URL"), defaultOpenAIBaseURL), "/")

	systems := map[string]string{
		"text":      cli.baseURL,
		"embedding": cli.embeddingBaseURL,
		"audio":     cli.audioBaseURL,
	}
	fallbacks := make(map[string]*fallbackTransport)
	for system, baseURL := range systems {
		// 推論先がフォールバック先と同じなら再試行しても意味がない
		if baseURL == "" || baseURL == fallbackBaseURL || isOpenAIEndpoint(baseURL) {
			continue
		}
		fallbacks[system] = &fallbackTransport{
			primaryBaseURL:  baseURL,
			fallbackBaseURL: fallbackBaseURL,
			fallbackKey:     realKey,
			system:          system,
		}
	}
	if len(fallbacks) == 0 {
		return
	}
	cli.fallbacks = fallbacks

	// SDK 経路（chat/completions, embeddings）にもトランスポートを通す
	if t, ok := fallbacks["text"]; ok {
		cli.c = newSDKClientWithTransport(cli.textKey, cli.baseURL, t)
	}
	if t, ok := fallbacks["embedding"]; ok {
		if cli.embeddingBaseURL == cli.baseURL {
			// テキストと同じ推論先なら SDK クライアントを共有したままにする
			cli.embedC = cli.c
		} else {
			cli.embedC = newSDKClientWithTransport(cli.embeddingKey, cli.embeddingBaseURL, t)
		}
	}
}

// fallbackEnabledFromEnv は OPENAI_FALLBACK_ENABLED を読む（既定 true）。
func fallbackEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OPENAI_FALLBACK_ENABLED"))) {
	case "false", "0", "no":
		return false
	default:
		return true
	}
}
