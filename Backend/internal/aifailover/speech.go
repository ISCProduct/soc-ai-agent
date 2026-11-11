package aifailover

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// 工程の名前（応答のメタ ai_fallback とログに出す）。
const (
	StageSTT = "stt"
	StageLLM = "llm"
	StageTTS = "tts"
)

// Primary は OpenAI 側の呼び出し（*openai.Client が満たす）。
type Primary interface {
	TranscribeWithHints(ctx context.Context, audio []byte, filename, hints string) (string, error)
	ChatInterview(ctx context.Context, systemPrompt string, history []map[string]string) (string, error)
	TTS(ctx context.Context, text, voice string) ([]byte, error)
}

// Outcome は1回の呼び出しで何が起きたか。
type Outcome struct {
	// Local はローカルで処理した（またはローカルを試した）。
	Local bool
	// PrimaryTried は OpenAI を呼んだ。失敗しても課金されている可能性があるので、
	// 利用量の記録はこれで決める（Breaker で OpenAI を飛ばしたときだけ false）。
	PrimaryTried bool
}

// Speech は音声面接の STT / LLM / TTS を、工程ごとに OpenAI → ローカルの順で呼ぶ。
// ローカルの接続先が無い工程は OpenAI だけを使う（今までと同じ）。
type Speech struct {
	primary Primary
	stt     *LocalSTT
	llm     *LocalLLM
	tts     *LocalTTS

	sttBreaker *Breaker
	llmBreaker *Breaker
	ttsBreaker *Breaker

	primaryTimeout time.Duration
	localTimeout   time.Duration
}

// Config はローカルの接続先と Circuit Breaker・待ち時間の設定。空の接続先の工程は縮退しない。
type Config struct {
	STTURL           string
	LLMBaseURL       string
	LLMModel         string
	TTSURL           string
	TTSSpeaker       string
	BreakerThreshold int
	BreakerCooldown  time.Duration
	// PrimaryTimeout は縮退できる工程で OpenAI を待つ上限。障害中に OpenAI の既定の待ち時間
	// （60秒）を使い切ると、ローカルに回す時間が残らない。
	PrimaryTimeout time.Duration
	// LocalTimeout はローカルの1工程を待つ上限。呼び出し元の ctx の期限（ターン全体の予算）が
	// これより短ければ、そちらで切れる。
	LocalTimeout time.Duration
}

// ConfigFromEnv は環境変数から設定を読む。
//
//	AI_FAILOVER_LOCAL_STT_URL     Whisper.cpp の server（例 http://10.0.0.5:8080）
//	AI_FAILOVER_LOCAL_LLM_URL     OpenAI 互換の API（例 http://10.0.0.5:11434/v1）
//	AI_FAILOVER_LOCAL_LLM_MODEL   既定 qwen3:1.7b
//	AI_FAILOVER_LOCAL_TTS_URL     VOICEVOX エンジン（例 http://10.0.0.5:50021）
//	AI_FAILOVER_LOCAL_TTS_SPEAKER 既定 1
//	AI_FAILOVER_BREAKER_THRESHOLD 既定 3（連続失敗で開く回数）
//	AI_FAILOVER_BREAKER_COOLDOWN_SECONDS 既定 30
//	AI_FAILOVER_PRIMARY_TIMEOUT_SECONDS 既定 20（縮退できる工程で OpenAI を待つ上限）
//	AI_FAILOVER_LOCAL_TIMEOUT_SECONDS 既定 25（ローカルの1工程を待つ上限）
func ConfigFromEnv() Config {
	return Config{
		STTURL:           strings.TrimSpace(os.Getenv("AI_FAILOVER_LOCAL_STT_URL")),
		LLMBaseURL:       strings.TrimSpace(os.Getenv("AI_FAILOVER_LOCAL_LLM_URL")),
		LLMModel:         envOr("AI_FAILOVER_LOCAL_LLM_MODEL", "qwen3:1.7b"),
		TTSURL:           strings.TrimSpace(os.Getenv("AI_FAILOVER_LOCAL_TTS_URL")),
		TTSSpeaker:       envOr("AI_FAILOVER_LOCAL_TTS_SPEAKER", "1"),
		BreakerThreshold: envInt("AI_FAILOVER_BREAKER_THRESHOLD", 3),
		BreakerCooldown:  time.Duration(envInt("AI_FAILOVER_BREAKER_COOLDOWN_SECONDS", 30)) * time.Second,
		PrimaryTimeout:   time.Duration(envInt("AI_FAILOVER_PRIMARY_TIMEOUT_SECONDS", 20)) * time.Second,
		LocalTimeout:     time.Duration(envInt("AI_FAILOVER_LOCAL_TIMEOUT_SECONDS", 25)) * time.Second,
	}
}

// New は Speech を作る。primary が nil なら nil を返す（呼び出し側は今まで通りの経路を使う）。
func New(primary Primary, cfg Config) *Speech {
	if primary == nil {
		return nil
	}
	// 待ち時間は呼び出しごとに ctx で切る（工程の上限とターン全体の予算の短い方）
	httpClient := &http.Client{}
	s := &Speech{
		primary:        primary,
		sttBreaker:     NewBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
		llmBreaker:     NewBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
		ttsBreaker:     NewBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
		primaryTimeout: cfg.PrimaryTimeout,
		localTimeout:   cfg.LocalTimeout,
	}
	if cfg.STTURL != "" {
		s.stt = &LocalSTT{URL: cfg.STTURL, HTTP: httpClient}
	}
	if cfg.LLMBaseURL != "" {
		s.llm = &LocalLLM{BaseURL: cfg.LLMBaseURL, Model: cfg.LLMModel, HTTP: httpClient}
	}
	if cfg.TTSURL != "" {
		s.tts = &LocalTTS{URL: cfg.TTSURL, Speaker: cfg.TTSSpeaker, HTTP: httpClient}
	}
	return s
}

// Enabled はローカルへ縮退できる工程が1つでもあるかを返す。
func (s *Speech) Enabled() bool {
	return s != nil && (s.stt != nil || s.llm != nil || s.tts != nil)
}

// Transcribe は音声を文字にする。
func (s *Speech) Transcribe(ctx context.Context, audio []byte, filename, hints string) (string, Outcome, error) {
	var local func(context.Context) (string, error)
	if s.stt != nil {
		local = func(c context.Context) (string, error) { return s.stt.Transcribe(c, audio, filename, hints) }
	}
	return run(ctx, s, StageSTT, s.sttBreaker, func(c context.Context) (string, error) {
		return s.primary.TranscribeWithHints(c, audio, filename, hints)
	}, local)
}

// Chat は面接官の応答を作る。
func (s *Speech) Chat(ctx context.Context, systemPrompt string, history []map[string]string) (string, Outcome, error) {
	var local func(context.Context) (string, error)
	if s.llm != nil {
		local = func(c context.Context) (string, error) { return s.llm.Chat(c, systemPrompt, history) }
	}
	return run(ctx, s, StageLLM, s.llmBreaker, func(c context.Context) (string, error) {
		return s.primary.ChatInterview(c, systemPrompt, history)
	}, local)
}

// Speak は読み上げの音声を作る。ローカル（VOICEVOX）は WAV を返す（OpenAI は mp3）。
func (s *Speech) Speak(ctx context.Context, text, voice string) ([]byte, Outcome, error) {
	var local func(context.Context) ([]byte, error)
	if s.tts != nil {
		local = func(c context.Context) ([]byte, error) { return s.tts.Speak(c, text) }
	}
	return run(ctx, s, StageTTS, s.ttsBreaker, func(c context.Context) ([]byte, error) {
		return s.primary.TTS(c, text, voice)
	}, local)
}

// run は OpenAI → ローカルの順に呼ぶ。
//   - ローカルが無い工程は OpenAI の結果をそのまま返す（Breaker も待ち時間の上限も使わない）
//   - Breaker が開いていれば OpenAI を飛ばしてローカルへ
//   - OpenAI が縮退してよい失敗ならローカルへ。認証・入力不正などはそのまま返す
//     （OpenAI は応答しているので Breaker は閉じる。半開の試しもここで終わる）
func run[T any](ctx context.Context, s *Speech, stage string, breaker *Breaker, primary, local func(context.Context) (T, error)) (T, Outcome, error) {
	if local == nil {
		v, err := primary(ctx)
		return v, Outcome{PrimaryTried: true}, err
	}
	out := Outcome{}
	if breaker.Allow() {
		out.PrimaryTried = true
		pctx, cancel := withTimeout(ctx, s.primaryTimeout)
		v, err := primary(pctx)
		cancel()
		if err == nil {
			breaker.Success()
			return v, out, nil
		}
		if !Degradable(ctx, err) {
			breaker.Success()
			return v, out, err
		}
		breaker.Failure()
		log.Printf("[AIFailover] %s: OpenAI の一時的な障害（%s）のためローカルへ縮退します", stage, failureKind(err))
	} else {
		log.Printf("[AIFailover] %s: Circuit Breaker が開いているため OpenAI を呼ばずにローカルを使います", stage)
	}
	out.Local = true
	lctx, cancel := withTimeout(ctx, s.localTimeout)
	defer cancel()
	v, err := local(lctx)
	if err != nil {
		log.Printf("[AIFailover] %s: ローカルも失敗しました: %v", stage, err)
		var zero T
		return zero, out, err
	}
	return v, out, nil
}

// withTimeout は d が正なら期限を付ける。呼び出し元の期限の方が短ければそちらが効く。
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func envInt(name string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || v <= 0 {
		return def
	}
	return v
}
