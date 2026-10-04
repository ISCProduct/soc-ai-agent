package sttbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// CaseResult は1ケース1モデルの評価結果。
//
// Transcript を持つのは、失敗ケースを人が確認するため。
// ただし出力先はリポジトリ外に限る運用とし、ログには出さない。
type CaseResult struct {
	ID             string   `json:"id"`
	DurationSec    float64  `json:"duration_sec"`
	LatencyMS      int64    `json:"latency_ms"`
	CER            float64  `json:"cer"`
	SemanticCER    float64  `json:"semantic_cer"`
	NumbersMatched int      `json:"numbers_matched"`
	NumbersTotal   int      `json:"numbers_total"`
	KeywordsHit    []string `json:"keywords_hit"`
	KeywordsMiss   []string `json:"keywords_miss"`
	// Failed は認識失敗（空・短すぎる出力）。Errored は API 呼び出し自体の失敗。
	// 混ぜると「APIが落ちていた」のか「聞き取れなかった」のかが読めなくなる。
	Failed          bool   `json:"recognition_failed"`
	Errored         bool   `json:"api_error"`
	Error           string `json:"error,omitempty"`
	Transcript      string `json:"transcript"`
	TranscriptChars int    `json:"transcript_chars"`
	Condition       string `json:"condition"`
	Source          string `json:"source"`
}

// ModelSummary はモデル単位の集計。
type ModelSummary struct {
	Model           string       `json:"model"`
	Cases           []CaseResult `json:"cases"`
	MeanCER         float64      `json:"mean_cer"`
	MeanSemanticCER float64      `json:"mean_semantic_cer"`
	KeywordAccuracy float64      `json:"keyword_accuracy"`
	NumberAccuracy  float64      `json:"number_accuracy"`
	// FailureRate は認識失敗率。分母は API が成功した件数で、APIエラーは含めない。
	FailureRate float64 `json:"failure_rate"`
	// ErroredCases は API 呼び出し自体が失敗した件数。CER の母数には入れない。
	// 残高切れ（429）で大半が落ちた run を「CERが改善した」と読まないため、
	// 必ず件数を添えて出す。
	ErroredCases int `json:"errored_cases"`
	// Aborted は 4xx が続いたので途中で打ち切ったことを示す。
	Aborted          bool    `json:"aborted"`
	MeanLatencyMS    int64   `json:"mean_latency_ms"`
	TotalAudioSec    float64 `json:"total_audio_sec"`
	EstCostPerMinUSD float64 `json:"est_cost_per_min_usd"`
	// 録音条件別・由来別の内訳（#1484）。合成だけの結果を全体平均に
	// 埋もれさせないため、条件と由来で分けても見られるようにする。
	ByCondition []GroupSummary `json:"by_condition"`
	BySource    []GroupSummary `json:"by_source"`
}

// Report は比較全体の結果。
type Report struct {
	GeneratedAt string                   `json:"generated_at"`
	Format      string                   `json:"format"`
	Models      map[string]*ModelSummary `json:"models"`
	// Hints は実際に prompt へ渡した補助語。どちらの条件で測ったのか
	// 出力ファイルだけで分かるように残す。BuildSTTHints の出力そのものなので
	// 会社名と読みを含む。出力先をリポジトリ外に限る運用はこのためでもある。
	Hints string `json:"hints,omitempty"`
}

// 公表単価（2026-09-09 時点）。$/分。
// コード内の定数は過去に実価格と10倍ずれていたことがあるため、
// 変更時は必ず公表価格を確認すること。
var costPerMinUSD = map[string]float64{
	"gpt-4o-transcribe":      0.006,
	"gpt-4o-mini-transcribe": 0.003,
	"whisper-1":              0.006,
}

// minCharsPerSec は認識失敗とみなす下限。
// 日本語の自然な発話はおおむね 4〜7文字/秒。
// 1.0 は「明らかに何も取れていない」水準に絞っている。
const minCharsPerSec = 1.0

// maxConsecutive4xx は 4xx が続いたときに打ち切る本数。
//
// 残高切れ（429）やキー失効（401）は後続も必ず失敗するので、
// 全件エラーのまま一瞬で「完走」した結果ファイルが残るのを防ぐ。
const maxConsecutive4xx = 3

// transcribeFn は集計のテストで実APIを呼ばないようにするための差し替え口。
var transcribeFn = transcribe

// RunModel は1モデルで全ケースを実行して集計する。
//
// hints は Transcription API の prompt に渡す補助語。空なら渡さない。
// 補助語の有無で結果が変わるかを同じ指標で比べるために引数で受ける。
//
// 平均値の母数は「APIが成功し、かつ認識失敗でもない件数」。
// APIエラーを分母に残すと分子へ 0 を足すだけになり、失敗が多い run ほど
// CER が良く出てしまう（残高切れの run が「改善」に見える）。
func RunModel(model string, cases []Case, audios []Audio, hints string) *ModelSummary {
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.ID] = c
	}

	s := &ModelSummary{Model: model, EstCostPerMinUSD: costPerMinUSD[model]}
	var totalLatency int64
	var apiOK, consecutive4xx int

	for _, a := range audios {
		c := byID[a.ID]
		text, latency, err := transcribeFn(model, hints, a)
		r := CaseResult{ID: a.ID, DurationSec: a.DurationSec, LatencyMS: latency, Condition: c.ConditionOf(), Source: c.SourceOf()}
		if err != nil {
			r.Error = err.Error()
			r.Errored = true
			s.ErroredCases++
			s.Cases = append(s.Cases, r)
			var se *statusError
			if errors.As(err, &se) && se.code >= 400 && se.code < 500 {
				consecutive4xx++
			} else {
				consecutive4xx = 0
			}
			if consecutive4xx >= maxConsecutive4xx {
				s.Aborted = true
				break
			}
			continue
		}
		consecutive4xx = 0
		apiOK++
		r.Transcript = text
		r.TranscriptChars = len([]rune(text))
		r.CER = CER(c.ReferenceText, text)
		r.SemanticCER = SemanticCER(c.ReferenceText, text)
		r.NumbersMatched, r.NumbersTotal = NumberAccuracy(c.ReferenceText, text)
		r.KeywordsHit, r.KeywordsMiss = KeywordHits(CheckableKeywords(c.Checks, c.ReferenceText), text)
		r.Failed = IsRecognitionFailure(text, a.DurationSec, minCharsPerSec)

		totalLatency += latency
		s.TotalAudioSec += a.DurationSec
		s.Cases = append(s.Cases, r)
	}

	if apiOK > 0 {
		s.MeanLatencyMS = totalLatency / int64(apiOK)
	}
	// 指標はすべて Breakdown に通す。モデル単位と条件単位で母数の扱いが
	// 食い違わないようにするため（同じ分母を二重に実装しない）。
	if all := Breakdown(s.Cases, func(CaseResult) string { return "all" }); len(all) == 1 {
		g := all[0]
		s.MeanCER, s.MeanSemanticCER = g.MeanCER, g.MeanSemanticCER
		s.KeywordAccuracy, s.NumberAccuracy = g.KeywordAccuracy, g.NumberAccuracy
		s.FailureRate = g.FailureRate
	}
	s.ByCondition = Breakdown(s.Cases, func(r CaseResult) string { return r.Condition })
	s.BySource = Breakdown(s.Cases, func(r CaseResult) string { return r.Source })
	return s
}

// buildTranscribeForm は Transcription API へ送る multipart 本体を組み立てる。
//
// hints が空のときに prompt フィールドを付けないのが要点。空文字を送るのと
// 送らないのを同じ扱いにすると、「補助語なし」の測定が成立しない。
func buildTranscribeForm(model, hints string, a Audio) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", model)
	_ = w.WriteField("language", "ja")
	if hints != "" {
		_ = w.WriteField("prompt", hints)
	}
	part, err := w.CreateFormFile("file", filepath.Base(a.Path))
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(a.Data); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// statusError は Transcription API が 2xx 以外を返したことを表す。
// 4xx の連続で打ち切る判定にステータスコードが必要なので型で持つ。
type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("HTTP %d", e.code) }

func transcribe(model, hints string, a Audio) (string, int64, error) {
	buf, contentType, err := buildTranscribeForm(model, hints, a)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/audio/transcriptions", buf)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
	req.Header.Set("Content-Type", contentType)

	start := time.Now()
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return "", latency, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", latency, err
	}
	if resp.StatusCode != http.StatusOK {
		// APIキーが本文に含まれることはないが、念のため本文全体は出さない
		return "", latency, &statusError{code: resp.StatusCode}
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", latency, err
	}
	return out.Text, latency, nil
}
