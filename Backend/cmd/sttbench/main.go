// sttbench は音声認識モデルの品質を、固定の音声フィクスチャで比較する（#1209 / 音声R&D）。
//
// 目的は「安いモデルに替えてよいか」を数値で判断できるようにすること。
// 一般的な文字誤り率だけでは、面接で致命的になる固有名詞・数値の誤りが埋もれるため、
// それらを別指標として出す。
//
//	go run ./cmd/sttbench -manifest ../docs/research/interview-audio-eval/manifest.jsonl \
//	  -models gpt-4o-transcribe,gpt-4o-mini-transcribe -out /tmp/stt-result.json
//
// -hints に面接コンテキストを渡すと、本番と同じ BuildSTTHints の出力を
// prompt へ載せて測れる。補助語ありと無しを同じ指標で比べるために使う。
//
// 出力はリポジトリ外へ書くこと。認識結果には学生の発話が入りうるため、
// フィクスチャであってもリポジトリに残さない運用に揃える。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Backend/internal/services/interview"
	"Backend/internal/sttbench"
)

// parseHintsContext は `会社名|読み|職種|企業情報` を分解する。
// 足りない要素は空文字。多すぎる場合、余りは企業情報へ寄せる
// （企業情報は自由記述で、区切り文字が現れうるため）。
func parseHintsContext(s string) (name, reading, position, info string) {
	parts := strings.SplitN(s, "|", 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2], parts[3]
}

func main() {
	manifestPath := flag.String("manifest", "", "manifest.jsonl のパス（必須）")
	models := flag.String("models", "gpt-4o-transcribe,gpt-4o-mini-transcribe", "比較するモデル（カンマ区切り）")
	format := flag.String("format", "wav", "使用する音声形式: wav | webm")
	out := flag.String("out", "", "結果JSONの出力先。未指定なら標準出力のみ")
	dryRun := flag.Bool("dry-run", false, "APIを呼ばず、音声とmanifestの対応だけ検証する")
	condition := flag.String("condition", "", "評価する録音条件で絞る（例: noisy）。未指定なら全件")
	hintsCtx := flag.String("hints", "", "補助語に使う面接コンテキスト `会社名|読み|職種|企業情報`。職種は本番が補助語に使わないため無視される。未指定なら補助語なし")
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
	if *condition != "" {
		filtered := sttbench.FilterByCondition(cases, *condition)
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "条件 %q に一致するケースがありません\n", *condition)
			os.Exit(1)
		}
		fmt.Printf("=== 条件で絞り込み: %s（%d/%d件） ===\n", *condition, len(filtered), len(cases))
		cases = filtered
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

	// 補助語は本番と同じ BuildSTTHints を通す。ここで別の文字列を作ると
	// 「本番の補助語が効くのか」を測ったことにならない。
	// -dry-run でも出すのは、API費用を掛けずに補助語を確認できるようにするため。
	hints := ""
	if *hintsCtx != "" {
		// 職種は補助語に使わない（#1600）。クライアント直値で DB 由来の対応物が無く、
		// 本番の BuildSTTHints が受け取らなくなった。ここで渡すと、本番がやらない
		// 条件で測ることになる。入力の書式は変えない（手順をそのまま使えるように）。
		name, reading, _, info := parseHintsContext(*hintsCtx)
		hints = interview.BuildSTTHints(name, reading, info)
		fmt.Printf("\n補助語: %s\n", hints)
	}

	if *dryRun {
		fmt.Println("\n-dry-run のためAPIは呼びません")
		return
	}
	if _, err := sttbench.APIConfigFromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "\n音声推論先の設定エラー: %v\n", err)
		if errors.Is(err, sttbench.ErrMissingOpenAIAPIKey) {
			// キー無しでもツール自体は壊れない。CIやキー未配布の環境で
			// 「実行できなかった」ことが分かるように終了コードを分ける。
			os.Exit(3)
		}
		os.Exit(2)
	}

	modelList := strings.Split(*models, ",")
	report := sttbench.Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Format:      *format,
		Models:      map[string]*sttbench.ModelSummary{},
		Hints:       hints,
	}
	for _, m := range modelList {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		fmt.Printf("\n=== %s ===\n", m)
		summary := sttbench.RunModel(m, cases, audios, hints)
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
