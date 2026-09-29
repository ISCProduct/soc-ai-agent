// Package aibench は AI 出力（ES添削・履歴書レビュー・面接レポート）の品質を
// 固定の入力セットで測る評価ロジック。
//
// 目的は「誰が測っても同じ数字が出ること」。プロンプト・モデル・パラメータを
// 変えたときに良くなったのか悪くなったのかを、実行者に依らず判定できるようにする。
// 単発の実験スクリプトと違い、入力（manifest）・手順（cmd/aibench）・出力（JSON）を
// 固定し、前回結果との差分を取れる形にしてある。
//
// ネットワークに触れる部分（targets）と指標の計算（metrics/aggregate）は分けてある。
// APIキーが無い環境でも集計の正しさを単体テストで固定できるようにするため。
package aibench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// ゴールドラベル。良/中/悪の3値のみ。
//
// 5段階にしないのは、人がラベルを付け直したときに揺れない粒度に留めるため。
// 弁別力は順位相関で見るので、段階数より「順序が付いていること」が重要。
const (
	LabelGood = "good"
	LabelMid  = "mid"
	LabelBad  = "bad"
)

// labelRank はラベルの順序（悪 < 中 < 良）。順位相関の一方の系列に使う。
var labelRank = map[string]float64{LabelBad: 1, LabelMid: 2, LabelGood: 3}

// 評価対象の名前。CLI の -target とマニフェストの target が一致する必要がある。
const (
	TargetES              = "es"
	TargetResume          = "resume"
	TargetInterviewReport = "interview-report"
)

// ValidTargets は対応している評価対象。
func ValidTargets() []string {
	return []string{TargetES, TargetResume, TargetInterviewReport}
}

// Case はマニフェスト（jsonl）1行。
type Case struct {
	ID     string `json:"id"`
	Target string `json:"target"`
	Label  string `json:"label"`
	Input  Input  `json:"input"`
	// Note はラベルを付けた理由。人がラベルを見直すときの根拠として残す。
	Note string `json:"note,omitempty"`
}

// Input は評価対象ごとの入力。使うフィールドは target によって決まる。
//
// target ごとに別の型へ分けないのは、マニフェストを1ファイル1形式に保つため。
// 必須フィールドの検証は LoadManifest が target ごとに行う。
type Input struct {
	// ES添削
	ESText       string `json:"es_text,omitempty"`
	QuestionType string `json:"question_type,omitempty"`
	CompanyName  string `json:"company_name,omitempty"`
	// 履歴書レビュー
	ResumeText    string `json:"resume_text,omitempty"`
	JobTitle      string `json:"job_title,omitempty"`
	CandidateType string `json:"candidate_type,omitempty"`
	// 面接レポート
	Transcript string `json:"transcript,omitempty"`
	Lang       string `json:"lang,omitempty"`
}

// LoadManifest は jsonl を読む。
//
// 1行でも壊れていれば止める。欠けたケースを黙って飛ばすと
// 「評価した件数」が実態とずれ、破損率・弁別力が静かに変わる。
// ID の重複も止める。同じ ID で2件あると再現性の集計が混ざる。
func LoadManifest(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []Case
	seen := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "//") {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(text), &c); err != nil {
			return nil, fmt.Errorf("manifest %d行目: %w", line, err)
		}
		if err := validateCase(c); err != nil {
			return nil, fmt.Errorf("manifest %d行目(%s): %w", line, c.ID, err)
		}
		if prev, dup := seen[c.ID]; dup {
			return nil, fmt.Errorf("manifest %d行目: id %q が %d行目と重複しています", line, c.ID, prev)
		}
		seen[c.ID] = line
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("manifest にケースがありません: %s", path)
	}
	return cases, nil
}

func validateCase(c Case) error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("id は必須")
	}
	if _, ok := labelRank[c.Label]; !ok {
		return fmt.Errorf("label は %s / %s / %s のいずれか（実際: %q）", LabelGood, LabelMid, LabelBad, c.Label)
	}
	if !slices.Contains(ValidTargets(), c.Target) {
		return fmt.Errorf("target が不正: %q", c.Target)
	}
	switch c.Target {
	case TargetES:
		if strings.TrimSpace(c.Input.ESText) == "" {
			return fmt.Errorf("input.es_text は必須")
		}
	case TargetResume:
		if strings.TrimSpace(c.Input.ResumeText) == "" {
			return fmt.Errorf("input.resume_text は必須")
		}
	case TargetInterviewReport:
		if strings.TrimSpace(c.Input.Transcript) == "" {
			return fmt.Errorf("input.transcript は必須")
		}
	}
	return nil
}

// FilterByTarget は指定した target のケースだけを残す。
func FilterByTarget(cases []Case, target string) []Case {
	out := make([]Case, 0, len(cases))
	for _, c := range cases {
		if c.Target == target {
			out = append(out, c)
		}
	}
	return out
}

// TakePerLabel は各ラベルから先頭 n 件ずつ残す（マニフェストの順序は保つ）。
//
// 先頭 n 件を単純に取らないのは、マニフェストがラベル順に並んでいるため。
// 先頭だけ取ると good しか入らず、弁別力が測れないまま動作確認を終える。
func TakePerLabel(cases []Case, n int) []Case {
	if n < 1 {
		return cases
	}
	taken := map[string]int{}
	out := make([]Case, 0, n*3)
	for _, c := range cases {
		if taken[c.Label] >= n {
			continue
		}
		taken[c.Label]++
		out = append(out, c)
	}
	return out
}

// LabelCounts はラベルごとの件数を返す。良/中/悪が揃っているかの確認用。
func LabelCounts(cases []Case) map[string]int {
	out := map[string]int{}
	for _, c := range cases {
		out[c.Label]++
	}
	return out
}

// InputText は target ごとの入力本文を返す。
//
// 文字数の交絡（スコアが内容ではなく長さに従っていないか）を測るのに使う。
// target ごとに入力フィールドが違うので、集計側で switch を書かないための一箇所。
func (c Case) InputText() string {
	switch c.Target {
	case TargetES:
		return c.Input.ESText
	case TargetResume:
		return c.Input.ResumeText
	case TargetInterviewReport:
		return c.Input.Transcript
	}
	return ""
}

// InputChars は入力本文の文字数（バイト数ではなく）。
func (c Case) InputChars() int { return len([]rune(c.InputText())) }
