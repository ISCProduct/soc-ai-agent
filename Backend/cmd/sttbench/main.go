// sttbench は音声認識モデルの品質を、固定の音声フィクスチャで比較する（#1209 / 音声R&D）。
//
// 目的は「安いモデルに替えてよいか」を数値で判断できるようにすること。
// 一般的な文字誤り率だけでは、面接で致命的になる固有名詞・数値の誤りが埋もれるため、
// それらを別指標として出す。
//
//	go run ./cmd/sttbench -manifest ../docs/research/interview-audio-eval/manifest.jsonl \
//	  -models gpt-4o-transcribe,gpt-4o-mini-transcribe -out /tmp/stt-result.json
//
// 出力はリポジトリ外へ書くこと。認識結果には学生の発話が入りうるため、
// フィクスチャであってもリポジトリに残さない運用に揃える。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Backend/internal/sttbench"
)

func main() {
	manifestPath := flag.String("manifest", "", "manifest.jsonl のパス（必須）")
	models := flag.String("models", "gpt-4o-transcribe,gpt-4o-mini-transcribe", "比較するモデル（カンマ区切り）")
	format := flag.String("format", "wav", "使用する音声形式: wav | webm")
	out := flag.String("out", "", "結果JSONの出力先。未指定なら標準出力のみ")
	dryRun := flag.Bool("dry-run", false, "APIを呼ばず、音声とmanifestの対応だけ検証する")
	flag.Parse()

	if *manifestPath == "" {
		fmt.Fprintln(os.Stderr, "-manifest は必須です")
		os.Exit(2)
	}

	cases, err := sttbench.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "manifest読み込み失敗: %v\n", err)
		os.Exit(1)
	}
	baseDir := filepath.Dir(*manifestPath)

	// 音声の実体・形式・長さを先に検証する。
	// ここを飛ばすと、欠損や途中切れをモデルの精度差と誤認する。
	audios, err := sttbench.InspectAll(cases, baseDir, *format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "音声の検証に失敗: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("=== 音声フィクスチャ（%s） ===\n", *format)
	fmt.Printf("%-26s %10s %10s %s\n", "ケース", "秒", "サイズ", "形式")
	for _, a := range audios {
		fmt.Printf("%-26s %10.2f %9dB %s\n", a.ID, a.DurationSec, a.Bytes, a.Format)
	}

	if *dryRun {
		fmt.Println("\n-dry-run のためAPIは呼びません")
		return
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		// キー無しでもツール自体は壊れない。CIやキー未配布の環境で
		// 「実行できなかった」ことが分かるように終了コードを分ける。
		fmt.Fprintln(os.Stderr, "\nOPENAI_API_KEY が未設定のため実APIの比較をスキップしました")
		os.Exit(3)
	}

	modelList := strings.Split(*models, ",")
	report := sttbench.Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Format:      *format,
		Models:      map[string]*sttbench.ModelSummary{},
	}
	for _, m := range modelList {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		fmt.Printf("\n=== %s ===\n", m)
		summary := sttbench.RunModel(m, cases, audios)
		report.Models[m] = summary
		sttbench.PrintSummary(os.Stdout, m, summary)
	}

	sttbench.PrintComparison(os.Stdout, modelList, report)

	if *out != "" {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "結果の整形に失敗: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*out, b, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "結果の書き出しに失敗: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n結果を %s に書き出しました（リポジトリ外に置くこと）\n", *out)
	}
}
