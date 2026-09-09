package sttbench

import (
	"bytes"
	"encoding/json"
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
	ID              string   `json:"id"`
	DurationSec     float64  `json:"duration_sec"`
	LatencyMS       int64    `json:"latency_ms"`
	CER             float64  `json:"cer"`
	SemanticCER     float64  `json:"semantic_cer"`
	NumbersMatched  int      `json:"numbers_matched"`
	NumbersTotal    int      `json:"numbers_total"`
	KeywordsHit     []string `json:"keywords_hit"`
	KeywordsMiss    []string `json:"keywords_miss"`
	Failed          bool     `json:"recognition_failed"`
	Error           string   `json:"error,omitempty"`
	Transcript      string   `json:"transcript"`
	TranscriptChars int      `json:"transcript_chars"`
}

// ModelSummary はモデル単位の集計。
type ModelSummary struct {
	Model            string       `json:"model"`
	Cases            []CaseResult `json:"cases"`
	MeanCER          float64      `json:"mean_cer"`
	MeanSemanticCER  float64      `json:"mean_semantic_cer"`
	KeywordAccuracy  float64      `json:"keyword_accuracy"`
	NumberAccuracy   float64      `json:"number_accuracy"`
	FailureRate      float64      `json:"failure_rate"`
	MeanLatencyMS    int64        `json:"mean_latency_ms"`
	TotalAudioSec    float64      `json:"total_audio_sec"`
	EstCostPerMinUSD float64      `json:"est_cost_per_min_usd"`
}

// Report は比較全体の結果。
type Report struct {
	GeneratedAt string                   `json:"generated_at"`
	Format      string                   `json:"format"`
	Models      map[string]*ModelSummary `json:"models"`
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

// RunModel は1モデルで全ケースを実行して集計する。
func RunModel(model string, cases []Case, audios []Audio) *ModelSummary {
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.ID] = c
	}

	s := &ModelSummary{Model: model, EstCostPerMinUSD: costPerMinUSD[model]}
	var totalCER, totalSemCER float64
	var kwHit, kwTotal, numHit, numTotal, failed int
	var totalLatency int64

	for _, a := range audios {
		c := byID[a.ID]
		text, latency, err := transcribe(model, a)
		r := CaseResult{ID: a.ID, DurationSec: a.DurationSec, LatencyMS: latency}
		if err != nil {
			r.Error = err.Error()
			r.Failed = true
			failed++
			s.Cases = append(s.Cases, r)
			continue
		}
		r.Transcript = text
		r.TranscriptChars = len([]rune(text))
		r.CER = CER(c.ReferenceText, text)
		r.SemanticCER = SemanticCER(c.ReferenceText, text)
		r.NumbersMatched, r.NumbersTotal = NumberAccuracy(c.ReferenceText, text)
		r.KeywordsHit, r.KeywordsMiss = KeywordHits(CheckableKeywords(c.Checks, c.ReferenceText), text)
		r.Failed = IsRecognitionFailure(text, a.DurationSec, minCharsPerSec)

		totalCER += r.CER
		totalSemCER += r.SemanticCER
		kwHit += len(r.KeywordsHit)
		kwTotal += len(r.KeywordsHit) + len(r.KeywordsMiss)
		numHit += r.NumbersMatched
		numTotal += r.NumbersTotal
		if r.Failed {
			failed++
		}
		totalLatency += latency
		s.TotalAudioSec += a.DurationSec
		s.Cases = append(s.Cases, r)
	}

	n := len(audios)
	if n > 0 {
		s.MeanCER = totalCER / float64(n)
		s.MeanSemanticCER = totalSemCER / float64(n)
		s.FailureRate = float64(failed) / float64(n)
		s.MeanLatencyMS = totalLatency / int64(n)
	}
	if kwTotal > 0 {
		s.KeywordAccuracy = float64(kwHit) / float64(kwTotal)
	}
	if numTotal > 0 {
		s.NumberAccuracy = float64(numHit) / float64(numTotal)
	}
	return s
}

func transcribe(model string, a Audio) (string, int64, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", model)
	_ = w.WriteField("language", "ja")
	part, err := w.CreateFormFile("file", filepath.Base(a.Path))
	if err != nil {
		return "", 0, err
	}
	if _, err := part.Write(a.Data); err != nil {
		return "", 0, err
	}
	if err := w.Close(); err != nil {
		return "", 0, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
	req.Header.Set("Content-Type", w.FormDataContentType())

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
		return "", latency, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", latency, err
	}
	return out.Text, latency, nil
}
