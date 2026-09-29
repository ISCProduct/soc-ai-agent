package aibench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifest(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "m.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("書き込み失敗: %v", err)
	}
	return path
}

// マニフェストの不正を黙って飛ばすと「評価した件数」が実態とずれ、
// 破損率も弁別力も静かに変わる。必ず止まることを固定する。
func TestLoadManifestは不正な行で止まる(t *testing.T) {
	valid := `{"id":"es-001","target":"es","label":"good","input":{"es_text":"本文"}}`
	tests := []struct {
		name    string
		lines   []string
		wantErr string
	}{
		{name: "JSONが壊れている", lines: []string{`{"id":`}, wantErr: "1行目"},
		{name: "idが無い", lines: []string{`{"target":"es","label":"good","input":{"es_text":"本文"}}`}, wantErr: "id は必須"},
		{name: "labelが規定外", lines: []string{`{"id":"a","target":"es","label":"excellent","input":{"es_text":"本文"}}`}, wantErr: "label は"},
		{name: "targetが不正", lines: []string{`{"id":"a","target":"chat","label":"good","input":{"es_text":"本文"}}`}, wantErr: "target が不正"},
		{name: "ES本文が無い", lines: []string{`{"id":"a","target":"es","label":"good","input":{}}`}, wantErr: "es_text は必須"},
		{name: "履歴書本文が無い", lines: []string{`{"id":"a","target":"resume","label":"good","input":{}}`}, wantErr: "resume_text は必須"},
		{name: "面接ログが無い", lines: []string{`{"id":"a","target":"interview-report","label":"good","input":{}}`}, wantErr: "transcript は必須"},
		{name: "idの重複", lines: []string{valid, valid}, wantErr: "重複"},
		{name: "空ファイル", lines: []string{""}, wantErr: "ケースがありません"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadManifest(writeManifest(t, tt.lines...))
			if err == nil {
				t.Fatal("エラーになるべき")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("エラー = %v, want %q を含む", err, tt.wantErr)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	path := writeManifest(t,
		`{"id":"es-001","target":"es","label":"good","input":{"es_text":"本文","question_type":"ガクチカ"},"note":"STARが揃っている"}`,
		``, // 空行は飛ばす
		`// コメント行も飛ばす`,
		`{"id":"r-001","target":"resume","label":"bad","input":{"resume_text":"職歴\n自己PR"}}`,
	)
	cases, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("読み込み失敗: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("件数 = %d, want 2", len(cases))
	}
	if cases[0].Input.QuestionType != "ガクチカ" || cases[0].Note == "" {
		t.Errorf("フィールドが読めていない: %+v", cases[0])
	}

	es := FilterByTarget(cases, TargetES)
	if len(es) != 1 || es[0].ID != "es-001" {
		t.Errorf("FilterByTarget = %+v", es)
	}
	counts := LabelCounts(cases)
	if counts[LabelGood] != 1 || counts[LabelBad] != 1 {
		t.Errorf("LabelCounts = %v", counts)
	}
}

// ゴールデンセットは「良/中/悪が揃っていること」が弁別力の前提。
// コミット済みのファイルが崩れたら気付けるようにしておく。
func TestゴールデンセットがCommitされている(t *testing.T) {
	tests := []struct {
		file   string
		target string
	}{
		{file: "es.jsonl", target: TargetES},
		{file: "resume.jsonl", target: TargetResume},
		{file: "interview-report.jsonl", target: TargetInterviewReport},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "docs", "research", "ai-eval", tt.file)
			cases, err := LoadManifest(path)
			if err != nil {
				t.Fatalf("%s の読み込みに失敗: %v", path, err)
			}
			if len(cases) < 30 {
				t.Errorf("件数 = %d, Issue #1525 は各30〜50件を要求している", len(cases))
			}
			counts := LabelCounts(cases)
			for _, label := range []string{LabelGood, LabelMid, LabelBad} {
				if counts[label] < 5 {
					t.Errorf("%s が %d件しかない（良/中/悪が偏ると順位相関が意味を持たない）", label, counts[label])
				}
			}
			for _, c := range cases {
				if c.Target != tt.target {
					t.Errorf("%s: target = %q, want %q", c.ID, c.Target, tt.target)
				}
				if strings.TrimSpace(c.Note) == "" {
					t.Errorf("%s: note（ラベルを付けた理由）が空", c.ID)
				}
			}
		})
	}
}

// ラベルが表層特徴（文字数・数値トークン数）だけで当たるセットでは、弁別力が
// 高いことと「内容の質を測れている」ことが同じ数字になり区別できない（#1593）。
// ケースを足すときにこの相関を悪化させないよう、上限を固定する。
//
// 数値トークン数も測るのは、履歴書ルーブリックがその軸を直接数えているため。
// specificity（数値・固有名詞・期間が何種類あるか）+ achievement（結果が数値で
// 示されているか）+ completeness（項目がいくつ埋まっているか）で新卒の70/100点が
// 決まる。ラベルを数値の個数で付けると、ゴールデンセットのラベルが評価器と
// 同じ関数で定義される（循環）。#1593 の追加16件は実際にこれを起こしており、
// 数値トークン数だけでラベルが 16/16 当たる（レンジが完全に非重複）。
//
// 上限の決め方: **実測値 + 0.05（0.01刻みで切り上げ）**。0.05 は report.go の
// 他の劣化閾値（破損率・σ）と同じ幅で、「測り直しの揺れでは動かないが、
// ケースを数件足して悪化させたら落ちる」幅として揃えている。目標値ではなく
// 「今より悪くしない」ための天井なので、実測が下がったらここも下げる。
//
// 現状: resume の文字数は #1593 で +0.944 → +0.456 まで下げた。
// **resume の数値トークン数は +0.870 → +0.901 で悪化している**（追加16件が
// 数値の個数でラベルを分けているため。下げるには「数値はあるが内容が薄い bad」
// 「数値が無いが検証可能な具体がある good」を足す必要がある → 別Issue #____）。
// es と interview-report も未対応で、現状値をそのまま天井にしてある
// （ここで落として赤くしても直る当てが無い → 別Issue #____）。
func Testゴールデンセットのラベルが表層特徴だけで当たらない(t *testing.T) {
	tests := []struct {
		file string
		// maxChars / maxNumeric は「ラベル vs 文字数」「ラベル vs 数値トークン数」の上限。
		maxChars   float64
		maxNumeric float64
	}{
		// 実測 chars +0.852 / numeric +0.830（どちらも未対応）
		{file: "es.jsonl", maxChars: 0.91, maxNumeric: 0.88},
		// 実測 chars +0.456（#1593 で対応）/ numeric +0.901（**未対応・悪化**）
		{file: "resume.jsonl", maxChars: 0.51, maxNumeric: 0.96},
		// 実測 chars +0.943 / numeric +0.838（どちらも未対応）
		{file: "interview-report.jsonl", maxChars: 0.99, maxNumeric: 0.89},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "docs", "research", "ai-eval", tt.file)
			cases, err := LoadManifest(path)
			if err != nil {
				t.Fatalf("%s の読み込みに失敗: %v", path, err)
			}
			labels := make([]float64, 0, len(cases))
			chars := make([]float64, 0, len(cases))
			numeric := make([]float64, 0, len(cases))
			for _, c := range cases {
				labels = append(labels, labelRank[c.Label])
				chars = append(chars, float64(c.InputChars()))
				numeric = append(numeric, float64(c.InputNumericTokens()))
			}
			features := []struct {
				name   string
				values []float64
				max    float64
			}{
				{name: "文字数", values: chars, max: tt.maxChars},
				{name: "数値トークン数", values: numeric, max: tt.maxNumeric},
			}
			for _, f := range features {
				corr := SpearmanCorrelation(labels, f.values)
				// 実測は常に出す。天井を割っているかだけでなく、どの表層特徴に
				// どれだけ寄っているかを読めるようにしておく。
				t.Logf("ラベル vs %s = %+.3f（上限 %.2f）", f.name, corr, f.max)
				if corr > f.max {
					t.Errorf("ラベルと%sの順位相関 = %+.3f, 上限 %.2f を超えた（%sだけでラベルが当たるセットになっている）",
						f.name, corr, f.max, f.name)
				}
			}
		})
	}
}

// InputText / InputChars / InputNumericTokens が target ごとに正しいフィールドを
// 見ているか。ここが壊れると交絡の測定が全 target で静かに死ぬ（InputChars が
// 常に 0 を返しても、弁別力は出るのでテストは緑のままになる）。
func TestCaseの入力本文と表層特徴(t *testing.T) {
	tests := []struct {
		name      string
		c         Case
		wantText  string
		wantChars int
		wantNums  int
	}{
		{
			name:      "es は es_text を見る",
			c:         Case{Target: TargetES, Input: Input{ESText: "売上を12%伸ばした", ResumeText: "無関係", Transcript: "無関係"}},
			wantText:  "売上を12%伸ばした",
			wantChars: 10,
			wantNums:  1,
		},
		{
			name:      "resume は resume_text を見る",
			c:         Case{Target: TargetResume, Input: Input{ESText: "無関係", ResumeText: "2024年4月〜2025年3月 店長", Transcript: "無関係"}},
			wantText:  "2024年4月〜2025年3月 店長",
			wantChars: 18,
			wantNums:  4,
		},
		{
			name:      "interview-report は transcript を見る",
			c:         Case{Target: TargetInterviewReport, Input: Input{ESText: "無関係", ResumeText: "無関係", Transcript: "User: ３名で対応しました"}},
			wantText:  "User: ３名で対応しました",
			wantChars: 15,
			wantNums:  1, // 全角数字も数値トークンとして数える
		},
		{
			name:      "未知の target は空（数えない）",
			c:         Case{Target: "chat", Input: Input{ESText: "本文1"}},
			wantText:  "",
			wantChars: 0,
			wantNums:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.InputText(); got != tt.wantText {
				t.Errorf("InputText() = %q, want %q", got, tt.wantText)
			}
			if got := tt.c.InputChars(); got != tt.wantChars {
				t.Errorf("InputChars() = %d, want %d", got, tt.wantChars)
			}
			if got := tt.c.InputNumericTokens(); got != tt.wantNums {
				t.Errorf("InputNumericTokens() = %d, want %d", got, tt.wantNums)
			}
		})
	}
}
