// aibench は AI出力（ES添削・履歴書レビュー・面接レポート）の品質を、
// 固定のゴールデンセットで測る（#1525）。
//
// 目的は「誰が測っても同じ数字が出ること」。プロンプト・モデル・パラメータを
// 変えたときに良くなったのか悪くなったのかを判定できるようにする。
//
//	go run ./cmd/aibench -target es -manifest ../docs/research/ai-eval/es.jsonl -n 3 -out /tmp/es.json
//	go run ./cmd/aibench -target resume -manifest ../docs/research/ai-eval/resume.jsonl -model gpt-4o
//	go run ./cmd/aibench -target interview-report -manifest ../docs/research/ai-eval/interview-report.jsonl
//	go run ./cmd/aibench -diff /tmp/prev.json /tmp/es.json
//
// CIでは動かさない（課金と実行時間のため）。手動バッチとして運用する。
// 結果JSONはリポジトリ外へ書くこと。LLM の出力本文が入るため。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"Backend/internal/aibench"
)

func main() {
	target := flag.String("target", "", "評価対象: "+strings.Join(aibench.ValidTargets(), " | "))
	manifestPath := flag.String("manifest", "", "マニフェスト(jsonl)のパス")
	n := flag.Int("n", 1, "同一入力の実行回数（再現性の測定に使う。2以上で意味を持つ）")
	model := flag.String("model", "", "モデル名。未指定なら本番と同じ既定値を使う")
	out := flag.String("out", "", "結果JSONの出力先。リポジトリ外を指定すること")
	limit := flag.Int("limit", 0, "各ラベルから先頭N件だけ実行する（動作確認用。0で全件）")
	yes := flag.Bool("yes", false, "概算コストの確認を省略する")
	dryRun := flag.Bool("dry-run", false, "APIを呼ばず、マニフェストの検証と概算コストだけ出す")
	diff := flag.String("diff", "", "比較モード: 前回と今回の結果JSONを 'prev.json,cur.json' で渡す")
	flag.Parse()

	if *diff != "" {
		os.Exit(runDiff(*diff, flag.Args()))
	}
	if *target == "" || *manifestPath == "" {
		fmt.Fprintln(os.Stderr, "-target と -manifest は必須です（-diff で比較モード）")
		flag.Usage()
		os.Exit(2)
	}

	cases, err := aibench.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "manifest読み込み失敗: %v\n", err)
		os.Exit(1)
	}
	cases = aibench.FilterByTarget(cases, *target)
	if len(cases) == 0 {
		fmt.Fprintf(os.Stderr, "target %q のケースがありません\n", *target)
		os.Exit(1)
	}
	if *limit > 0 {
		cases = aibench.TakePerLabel(cases, *limit)
	}

	counts := aibench.LabelCounts(cases)
	fmt.Printf("=== %s: %d件（good=%d mid=%d bad=%d）× %d回 ===\n",
		*target, len(cases), counts[aibench.LabelGood], counts[aibench.LabelMid], counts[aibench.LabelBad], *n)
	if counts[aibench.LabelGood] == 0 || counts[aibench.LabelBad] == 0 {
		// 良と悪が揃っていないと順位相関が意味を持たない。
		// -limit で絞ったときに起きるので、止めずに警告する。
		fmt.Println("⚠ good/bad が揃っていないため弁別力は参考値になりません")
	}

	tgt, err := aibench.NewTarget(*target, *model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "評価対象の準備に失敗: %v\n", err)
		os.Exit(1)
	}

	// 実行前に概算コストを出す。間違ったマニフェストで数千件を回す事故を止める。
	est := aibench.EstimateCost(tgt, cases, *n)
	fmt.Printf("モデル %s / 概算コスト $%.4f（出力上限いっぱいを仮定した上限見積もり）\n", tgt.Model(), est)

	if *dryRun {
		fmt.Println("-dry-run のためAPIは呼びません")
		return
	}
	if !*yes && !confirm() {
		fmt.Println("中止しました")
		os.Exit(130)
	}
	if os.Getenv("OPENAI_API_KEY") == "" && *target != "es" {
		// ES は RAG 側がキーを持つのでハーネスには不要。
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY が未設定です")
		os.Exit(3)
	}

	summary, err := aibench.RunAndAggregate(context.Background(), tgt, cases, *n, *manifestPath, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n実行を中止しました: %v\n", err)
		os.Exit(4)
	}
	aibench.PrintSummary(os.Stdout, summary)

	if *out != "" {
		if err := writeJSON(*out, summary); err != nil {
			fmt.Fprintf(os.Stderr, "結果の書き出しに失敗: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n結果を %s に書き出しました（リポジトリ外に置くこと）\n", *out)
	}
}

// runDiff は2つの結果JSONを比べる。劣化を検知したら終了コード1を返す。
func runDiff(first string, rest []string) int {
	paths := strings.Split(first, ",")
	paths = append(paths, rest...)
	if len(paths) != 2 {
		fmt.Fprintln(os.Stderr, "-diff は前回と今回の2ファイルを指定してください（例: -diff prev.json,cur.json）")
		return 2
	}
	prev, err := readSummary(strings.TrimSpace(paths[0]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "前回結果の読み込み失敗: %v\n", err)
		return 1
	}
	cur, err := readSummary(strings.TrimSpace(paths[1]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "今回結果の読み込み失敗: %v\n", err)
		return 1
	}
	if aibench.PrintDiff(os.Stdout, prev, cur) {
		return 1
	}
	return 0
}

func readSummary(path string) (*aibench.Summary, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s aibench.Summary
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Target == "" {
		return nil, errors.New("target が無い（aibench の結果JSONではない）")
	}
	return &s, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func confirm() bool {
	fmt.Print("実行しますか？ [y/N]: ")
	var ans string
	if _, err := fmt.Scanln(&ans); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(ans), "y")
}
