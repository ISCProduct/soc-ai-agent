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

// ラベルと文字数が相関しているセットでは、弁別力が高いことと「内容の質を
// 測れている」ことが同じ数字になり区別できない（#1593）。ケースを足すときに
// この相関を悪化させないよう、現状の値を上限として固定する。
//
// 上限は「今より悪くしない」ためのもので、目標値ではない。resume は #1593 で
// 長さを揃えたケースを16件足して +0.944 → +0.456 まで下げた。es と
// interview-report は未対応なので、現状値をそのまま天井にしてある
// （下げる作業は別Issue。ここで落として赤くしても直る当てが無い）。
func Testゴールデンセットのラベルと文字数が相関しすぎていない(t *testing.T) {
	tests := []struct {
		file    string
		maxCorr float64
	}{
		{file: "es.jsonl", maxCorr: 0.90},               // 実測 +0.852（未対応）
		{file: "resume.jsonl", maxCorr: 0.60},           // 実測 +0.456（#1593 で対応）
		{file: "interview-report.jsonl", maxCorr: 0.96}, // 実測 +0.943（未対応）
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
			for _, c := range cases {
				labels = append(labels, labelRank[c.Label])
				chars = append(chars, float64(c.InputChars()))
			}
			corr := SpearmanCorrelation(labels, chars)
			if corr > tt.maxCorr {
				t.Errorf("ラベルと文字数の順位相関 = %+.3f, 上限 %.2f を超えた（長さだけでラベルが当たるセットになっている）", corr, tt.maxCorr)
			}
		})
	}
}
