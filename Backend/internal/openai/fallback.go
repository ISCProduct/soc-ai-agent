package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// localAttemptDeadlineRatio は「呼び出し側の残り時間」のうち1回目（ローカル）に
// 割り当てる割合。
//
// ローカル推論の実際の障害は即時 5xx より「遅い・返らない」（モデルロード待ち・OOM・
// GPU 競合）が主。1回目が持ち時間を使い切ると、フォールバックは0バイトも送れずに
// 終わる（ログ上は「フォールバックした」と出るのに実際は届かない）。
// 残り時間の一部を2回目のために必ず取り置く。
//
// 固定の上限秒にしないのは、ローカルの大きめモデルが正常に長時間生成している
// ケースを打ち切ってしまうため。呼び出し側が持つ期限に比例させる。
const localAttemptDeadlineRatio = 0.7

// fallbackFlagKey は「このリクエストがフォールバックで処理されたか」を
// 呼び出し側（使用量記録）へ伝えるためのコンテキストキー。
type fallbackFlagKey struct{}

// withFallbackFlag は使用量記録用のフォールバック判定フラグをコンテキストに載せる。
// トランスポートが書き、使用量記録が読む。
func withFallbackFlag(ctx context.Context) context.Context {
	return context.WithValue(ctx, fallbackFlagKey{}, &atomic.Bool{})
}

func markFallbackUsed(ctx context.Context) {
	if flag, ok := ctx.Value(fallbackFlagKey{}).(*atomic.Bool); ok {
		flag.Store(true)
	}
}

// fallbackUsed はこのリクエストがフォールバックで処理されたかを返す。
func fallbackUsed(ctx context.Context) bool {
	flag, ok := ctx.Value(fallbackFlagKey{}).(*atomic.Bool)
	return ok && flag.Load()
}

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
	fallbackModel   string // OpenAI 側のモデル名（空ならボディを書き換えない）
	guard           FallbackGuard
	system          string // "text" / "embedding" / "audio"（ログ用）

	// localAttemptTimeout は呼び出し側に期限が無いときの1回目の上限。0 なら無制限。
	localAttemptTimeout time.Duration
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

	localCtx, cancelLocal := t.localAttemptContext(req.Context())
	resp, err := t.transport().RoundTrip(withBody(req.WithContext(localCtx), body))

	// ローカルのレスポンスを呼び出し元へ返す場合、localCtx はまだ生かしておく必要がある。
	// resp.Body の読み出しは http.Transport の readLoop 経由で、その ctx が
	// キャンセルされるとバッファ済みの分（既定4KB）までしか読めなくなる。
	// Body を閉じるタイミングまでキャンセルを遅らせる。
	returnLocal := func() (*http.Response, error) {
		if resp == nil || resp.Body == nil {
			cancelLocal()
			return resp, err
		}
		resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancelLocal}
		return resp, err
	}

	if !shouldFallback(resp, err) {
		return returnLocal()
	}

	// 呼び出し側（ユーザー切断・上位のタイムアウト）が終了している場合は再試行しない。
	// 投げても無駄で、ガードの分間カウンタだけ消費する。
	if reqErr := req.Context().Err(); reqErr != nil {
		return returnLocal()
	}

	// ローカルを自前の期限で打ち切ったのか、ローカル自身が失敗したのかを
	// 運用で切り分けられるようにする。
	localTimedOut := localCtx.Err() != nil
	if localTimedOut {
		slog.Warn("local ai attempt exceeded its budget", "system", t.system, "path", req.URL.Path)
	}

	// ガード未注入は「上限判定できない」= 拒否（fail-closed）。
	// cmd/server 以外の main（crawl/cli/api）も NewFromEnv を呼ぶため、
	// 許可側に倒すと注入漏れの経路が無制限に課金される（#1293 レビュー）。
	allowed, reason := false, "fallback guard is not configured"
	if t.guard != nil {
		allowed, reason = t.guard.AllowFallback()
	}
	if !allowed {
		// 縮退運転。ローカルのエラーをそのまま返し、OpenAI は呼ばない
		slog.Warn("openai fallback suppressed", "system", t.system, "reason", reason)
		return returnLocal()
	}

	fbReq, buildErr := t.rewriteToFallback(req, body)
	if buildErr != nil {
		slog.Warn("openai fallback request build failed", "system", t.system, "error", buildErr)
		return returnLocal()
	}

	// ここから先はローカルのレスポンスを捨てるので、予算も解放してよい
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	cancelLocal()

	slog.Warn("falling back to openai", "system", t.system, "path", req.URL.Path, "local_timeout", localTimedOut)
	markFallbackUsed(req.Context())
	fbResp, fbErr := t.transport().RoundTrip(fbReq)
	if fbErr != nil {
		// 2回目も失敗。呼び出し側のエラー文字列判定（モデル未検出など）が
		// 誤った文言で動かないよう、ローカル側のエラーを優先して返す。
		slog.Warn("openai fallback also failed", "system", t.system, "error", fbErr)
		if err != nil {
			return nil, err
		}
		return nil, fbErr
	}
	return fbResp, nil
}

// cancelOnCloseBody は Body を閉じるまで ctx のキャンセルを遅らせるラッパー。
//
// RoundTrip が返った時点でレスポンスヘッダーは届いているが、ボディはまだ
// ネットワークから読み出している途中。ここで ctx をキャンセルすると
// バッファ済みの分しか読めず、正常な応答が途中で切れる。
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// localAttemptContext は1回目（ローカル）用の ctx を返す。
//
// 呼び出し側に期限があればその一部だけを渡し、残りをフォールバックのために取り置く。
// 期限が無い経路（http.Client.Timeout だけで制御している音声など）は
// AI_LOCAL_ATTEMPT_TIMEOUT_SECONDS が設定されていればそれを使う。
// どちらも無ければ制限しない（正常な長時間生成を打ち切らないため）。
func (t *fallbackTransport) localAttemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			return context.WithTimeout(parent, time.Duration(float64(remaining)*localAttemptDeadlineRatio))
		}
	}
	if limit := t.localAttemptTimeout; limit > 0 {
		return context.WithTimeout(parent, limit)
	}
	return context.WithCancel(parent)
}

// shouldFallback は「ローカル側の障害」と判断できる場合だけ true を返す。
// 4xx はリクエスト自体の問題で OpenAI でも同じく失敗するため対象にしない。
func shouldFallback(resp *http.Response, err error) bool {
	if err != nil {
		// 呼び出し側のキャンセルはローカルの障害ではない。
		// 1回目のタイムアウト（localCtx 由来）はローカルの障害なので、
		// ここでは区別せず true にし、呼び出し側 ctx の生存は RoundTrip 側で確認する。
		return !errors.Is(err, context.Canceled)
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

	// ローカルのモデル名（gpt-oss-20b 等）は OpenAI に存在しないため、
	// JSON ボディの model を差し替える。未設定ならそのまま送る（互換ゲートウェイ用）。
	if t.fallbackModel != "" && isJSONRequest(req) {
		body, err = rewriteModelInJSON(body, t.fallbackModel)
		if err != nil {
			return nil, err
		}
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

func isJSONRequest(req *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(req.Header.Get("Content-Type")), "application/json")
}

// rewriteModelInJSON は OpenAI 互換リクエストの model フィールドだけを差し替える。
// 他のフィールドは触らない（json.Marshal の順序変更は許容範囲）。
func rewriteModelInJSON(body []byte, model string) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		// JSON として読めないなら触らない（multipart 等）
		return body, nil
	}
	if _, ok := payload["model"]; !ok {
		return body, nil
	}
	payload["model"] = model
	return json.Marshal(payload)
}

// SetFallbackGuard はフォールバックの可否判定を注入する（#1293）。
//
// 未注入のままだとフォールバックしない（fail-closed）。NewFromEnv は
// cmd/server 以外の main からも呼ばれるため、許可側に倒すと注入漏れの経路が
// 上限なしで従量課金へ流れる。
func (cli *Client) SetFallbackGuard(guard FallbackGuard) {
	if cli == nil {
		return
	}
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
// ガードが行う。未注入ならフォールバックしない（fail-closed）。
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
	fallbackBaseURL, urlErr := validatedFallbackBaseURL(os.Getenv("AI_FALLBACK_BASE_URL"))
	if urlErr != nil {
		// 不正な宛先にフォールバックすると、実キーが意図しないホストへ平文で出る。
		// 起動時に気づけるようログに出し、フォールバックは設定しない（縮退運転）。
		slog.Error("AI_FALLBACK_BASE_URL is invalid; openai fallback disabled", "error", urlErr)
		return
	}

	// ローカルのモデル名（gpt-oss-20b 等）は OpenAI に存在しないため、
	// フォールバック時に差し替える名前。
	//
	// フォールバック先が OpenAI 本家のときだけ既定値を入れる。既定が無いと
	// .env.example の推奨構成のままフォールバックしても model_not_found で
	// 必ず失敗する = 機構はあるが既定で無効、になってしまう。
	// 互換ゲートウェイ経由（AI_FALLBACK_BASE_URL 指定）はモデル名を
	// 勝手に決められないので空のままにする。
	toOpenAI := fallbackBaseURL == defaultOpenAIBaseURL
	fallbackModels := map[string]string{
		"text":      firstNonEmptyOr(os.Getenv("AI_FALLBACK_MODEL"), defaultIf(toOpenAI, "gpt-4o-mini")),
		"embedding": firstNonEmptyOr(os.Getenv("AI_FALLBACK_EMBEDDING_MODEL"), defaultIf(toOpenAI, "text-embedding-3-small")),
	}

	// 呼び出し側に期限が無い経路（音声）で fail-fast させたいときの1回目の上限。
	localAttemptTimeout := time.Duration(0)
	if secs, err := strconv.Atoi(strings.TrimSpace(os.Getenv("AI_LOCAL_ATTEMPT_TIMEOUT_SECONDS"))); err == nil && secs > 0 {
		localAttemptTimeout = time.Duration(secs) * time.Second
	}

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
			primaryBaseURL:      baseURL,
			fallbackBaseURL:     fallbackBaseURL,
			fallbackKey:         realKey,
			fallbackModel:       fallbackModels[system],
			system:              system,
			localAttemptTimeout: localAttemptTimeout,
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

// firstNonEmptyOr は env 値が空なら既定値を返す。
func firstNonEmptyOr(value, def string) string {
	if v := strings.TrimSpace(value); v != "" {
		return v
	}
	return def
}

func defaultIf(cond bool, value string) string {
	if cond {
		return value
	}
	return ""
}

// validatedFallbackBaseURL はフォールバック先 URL を検証する。
//
// 実キーを送る宛先なので http:// は拒否する（isOpenAIEndpoint が scheme を見るのと同じ理由）。
// scheme 無しの指定も拒否する。実際の障害時に初めて
// "unsupported protocol scheme" で気づくのが最悪のタイミングだから。
func validatedFallbackBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return defaultOpenAIBaseURL, nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", errors.New("host が空: " + trimmed)
	}
	// 判定は keyFor と共有する（allowsRealKey）。同じ「平文だが安全か」を
	// 2箇所で別々に判断すると必ず食い違う。
	if !allowsRealKey(trimmed) {
		return "", errors.New("scheme must be https (実APIキーを送るため平文は許可しない): " + trimmed)
	}
	return trimmed, nil
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
