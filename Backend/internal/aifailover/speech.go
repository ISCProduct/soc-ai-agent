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
}

// Config はローカルの接続先と Circuit Breaker の設定。空の接続先の工程は縮退しない。
type Config struct {
	STTURL           string
	LLMBaseURL       string
	LLMModel         string
	TTSURL           string
	TTSSpeaker       string
	BreakerThreshold int
	BreakerCooldown  time.Duration
	LocalTimeout     time.Duration
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
//	AI_FAILOVER_LOCAL_TIMEOUT_SECONDS 既定 60（GPU なしの推論 PC は遅いので長め）
func ConfigFromEnv() Config {
	return Config{
		STTURL:           strings.TrimSpace(os.Getenv("AI_FAILOVER_LOCAL_STT_URL")),
		LLMBaseURL:       strings.TrimSpace(os.Getenv("AI_FAILOVER_LOCAL_LLM_URL")),
		LLMModel:         envOr("AI_FAILOVER_LOCAL_LLM_MODEL", "qwen3:1.7b"),
		TTSURL:           strings.TrimSpace(os.Getenv("AI_FAILOVER_LOCAL_TTS_URL")),
		TTSSpeaker:       envOr("AI_FAILOVER_LOCAL_TTS_SPEAKER", "1"),
		BreakerThreshold: envInt("AI_FAILOVER_BREAKER_THRESHOLD", 3),
		BreakerCooldown:  time.Duration(envInt("AI_FAILOVER_BREAKER_COOLDOWN_SECONDS", 30)) * time.Second,
		LocalTimeout:     time.Duration(envInt("AI_FAILOVER_LOCAL_TIMEOUT_SECONDS", 60)) * time.Second,
	}
}

// New は Speech を作る。primary が nil なら nil を返す（呼び出し側は今まで通りの経路を使う）。
func New(primary Primary, cfg Config) *Speech {
	if primary == nil {
		return nil
	}
	httpClient := &http.Client{Timeout: cfg.LocalTimeout}
	s := &Speech{
		primary:    primary,
		sttBreaker: NewBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
		llmBreaker: NewBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
		ttsBreaker: NewBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
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

// Transcribe は音声を文字にする。fallback は ローカルで処理したら true。
func (s *Speech) Transcribe(ctx context.Context, audio []byte, filename, hints string) (text string, fallback bool, err error) {
	var local func() (string, error)
	if s.stt != nil {
		local = func() (string, error) { return s.stt.Transcribe(ctx, audio, filename, hints) }
	}
	return run(ctx, StageSTT, s.sttBreaker, func() (string, error) {
		return s.primary.TranscribeWithHints(ctx, audio, filename, hints)
	}, local)
}

// Chat は面接官の応答を作る。
func (s *Speech) Chat(ctx context.Context, systemPrompt string, history []map[string]string) (text string, fallback bool, err error) {
	var local func() (string, error)
	if s.llm != nil {
		local = func() (string, error) { return s.llm.Chat(ctx, systemPrompt, history) }
	}
	return run(ctx, StageLLM, s.llmBreaker, func() (string, error) {
		return s.primary.ChatInterview(ctx, systemPrompt, history)
	}, local)
}

// Speak は読み上げの音声を作る。ローカル（VOICEVOX）は WAV を返す（OpenAI は mp3）。
func (s *Speech) Speak(ctx context.Context, text, voice string) (audio []byte, fallback bool, err error) {
	var local func() ([]byte, error)
	if s.tts != nil {
		local = func() ([]byte, error) { return s.tts.Speak(ctx, text) }
	}
	return run(ctx, StageTTS, s.ttsBreaker, func() ([]byte, error) {
		return s.primary.TTS(ctx, text, voice)
	}, local)
}

// run は OpenAI → ローカルの順に呼ぶ。
//   - ローカルが無い工程は OpenAI の結果をそのまま返す（Breaker も使わない）
//   - Breaker が開いていれば OpenAI を飛ばしてローカルへ
//   - OpenAI が縮退してよい失敗ならローカルへ。認証・入力不正などはそのまま返す
func run[T any](ctx context.Context, stage string, breaker *Breaker, primary, local func() (T, error)) (T, bool, error) {
	if local == nil {
		v, err := primary()
		return v, false, err
	}
	if breaker.Allow() {
		v, err := primary()
		if err == nil {
			breaker.Success()
			return v, false, nil
		}
		if !Degradable(ctx, err) {
			return v, false, err
		}
		breaker.Failure()
		log.Printf("[AIFailover] %s: OpenAI の一時的な障害のためローカルへ縮退します: %v", stage, err)
	} else {
		log.Printf("[AIFailover] %s: Circuit Breaker が開いているため OpenAI を呼ばずにローカルを使います", stage)
	}
	v, err := local()
	if err != nil {
		log.Printf("[AIFailover] %s: ローカルも失敗しました: %v", stage, err)
		var zero T
		return zero, true, err
	}
	return v, true, nil
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
