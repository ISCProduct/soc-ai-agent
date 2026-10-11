package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// TranscribeWithHints は補助語（prompt）を添えて音声を文字起こしする（音声R&D Task 4）。
//
// hints が空なら Transcribe と同じ動作になる。企業未選択の面接でも
// 従来どおり動かすため、補助語の有無で経路を分けない。
//
// prompt は gpt-4o-transcribe / gpt-4o-mini-transcribe / whisper-1 の
// いずれも受理することを実APIで確認済み（RESULTS_stt_hints.md）。
//
// 実測では固有名詞の認識が改善し、無関係な語を渡しても幻覚は起きなかった。
// ただし mini は実行ごとのばらつきが大きく、補助語があっても
// 高精度モデルの安定性には届かない。
func (cli *Client) TranscribeWithHints(ctx context.Context, audio []byte, filename, hints string) (string, error) {
	return cli.TranscribeWithModel(ctx, audio, filename, hints, resolveTranscribeModel())
}

// TranscribeWithModel はモデルを明示して文字起こしする（音声R&D Task 5）。
//
// 選択的フォールバックで、既定モデルとは別の高精度モデルへ
// 同じ音声を再送するために使う。
func (cli *Client) TranscribeWithModel(ctx context.Context, audio []byte, filename, hints, model string) (string, error) {
	if err := cli.ensureAudio(); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	WriteTranscribeContext(w, model, hints)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.AudioBaseURL()+"/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.audioKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := cli.httpClientFor("audio", 60*time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		// 補助語には企業名が含まれる。エラー本文をそのまま返すと
		// リクエスト内容がログへ流れうるため、状態コードだけにする。
		return "", fmt.Errorf("transcription failed: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// WriteTranscribeContext は model / 言語 / 補助語のフォーム値を書く。
//
// gpt-transcribe は言語を `languages`、語を `keywords` で受ける（`language` と併送不可）。
// 旧モデルは語の羅列を prompt で受けるが、gpt-4o-transcribe は実音声で
// その羅列をそのまま認識結果として返すことがあった（13発話中9件）。
// keywords の1語に <, >, 改行が含まれるとリクエスト全体が拒否されるため、その語は落とす。
func WriteTranscribeContext(w *multipart.Writer, model, hints string) {
	_ = w.WriteField("model", model)
	if model != GPTTranscribeModel {
		_ = w.WriteField("language", "ja")
		if strings.TrimSpace(hints) != "" {
			_ = w.WriteField("prompt", hints)
		}
		return
	}
	_ = w.WriteField("languages[]", "ja")
	for _, k := range strings.Split(hints, ",") {
		k = strings.TrimSpace(k)
		if k == "" || strings.ContainsAny(k, "<>\r\n") {
			continue
		}
		_ = w.WriteField("keywords[]", k)
	}
}

// GPTTranscribeModel は録音音声向けの推奨モデル。
const GPTTranscribeModel = "gpt-transcribe"

// DefaultTranscribeModel は STT の既定モデル。
// ずれると補助語ありと無しで別モデルが使われ、比較が成立しない。
//
// 実際の面接音声（2026-10-11, 13発話）の文字誤り率は
// gpt-4o-mini-transcribe 23% に対し gpt-transcribe 18%。単価は $0.003 → $0.0045/分。
const DefaultTranscribeModel = GPTTranscribeModel

// resolveTranscribeModel は環境変数を見て使用するSTTモデルを決める。
//
// Transcribe と TranscribeWithHints が別々に既定値を持っていた頃は、
// 片方だけ変えても誰も気づかず、ログに出るモデル名と実際に叩くモデルが
// 食い違いうる状態だった。解決はここ1箇所に集約する。
func resolveTranscribeModel() string {
	if m := os.Getenv("OPENAI_WHISPER_MODEL"); m != "" {
		return m
	}
	return DefaultTranscribeModel
}
