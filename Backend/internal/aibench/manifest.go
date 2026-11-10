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
	"Backend/domain/entity"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
	TargetChatAnswer      = "chat-answer"
	// TargetCompanyExtract は事実テキストからの企業情報の抽出（#1782）。
	TargetCompanyExtract = "company-extract"
	// TargetMatchingReason はマッチング理由の生成（#1782）。
	TargetMatchingReason = "matching-reason"
)

// labelOptionalTargets は良/中/悪のラベルを付けない評価対象（#1782）。
//
// 他の target は LLM が付けた点がラベルの順に並ぶか（弁別力）を測る。この2つは
// LLM が点を付けるのではなく出力を作る処理なので、正解の項目との一致と指示の遵守から
// 決まった計算で点を出す。ラベルが無いケースは集計の弁別力・交絡の計算から外れる。
var labelOptionalTargets = []string{TargetCompanyExtract, TargetMatchingReason}

// LabelOptional は良/中/悪のラベルを付けない評価対象かを返す。
func LabelOptional(target string) bool { return slices.Contains(labelOptionalTargets, target) }

// ValidTargets は対応している評価対象。
func ValidTargets() []string {
	return []string{TargetES, TargetResume, TargetInterviewReport, TargetChatAnswer, TargetCompanyExtract, TargetMatchingReason}
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
	// OCRLineConfidence は ResumeText の非空行と同じ順の認識スコア（0〜1）。
	// 省略した行は信頼度不明。resume.OCRRewriteMinConfidence 未満の行へ
	// rewrite を出すと rewrite_on_low_confidence になる（#1942）。
	OCRLineConfidence []float64 `json:"ocr_line_confidence,omitempty"`
	// 面接レポート
	Transcript string `json:"transcript,omitempty"`
	Lang       string `json:"lang,omitempty"`
	// チャット診断の回答評価
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer,omitempty"`
	// 企業情報の抽出（CompanyName も使う）
	FactsText  string                  `json:"facts_text,omitempty"`
	WebsiteURL string                  `json:"website_url,omitempty"`
	Expected   *CompanyExtractExpected `json:"expected,omitempty"`
	// マッチング理由の生成。Match と UserScores は本番の型のまま受ける（フィールド名は Go の名前）。
	Match      *entity.UserCompanyMatch `json:"match,omitempty"`
	UserScores []entity.UserWeightScore `json:"user_scores,omitempty"`
	// MustMentionAny は企業側の事実の語。理由にいずれか1つ以上が入っていること
	// （プロンプトの「企業側の情報を1つ以上反映」）。
	MustMentionAny []string `json:"must_mention_any,omitempty"`
	// StrengthAny は利用者の強みの軸。理由にいずれか1つ以上が入っていること
	// （プロンプトの「一致している強み軸を言及」）。
	StrengthAny []string `json:"strength_any,omitempty"`
}

// CompanyExtractExpected は企業情報の抽出の正解（#1782）。
// 事実テキストに書かれている値だけを置き、書かれていない項目は Absent に並べる。
type CompanyExtractExpected struct {
	// Industry は業種の選択肢のいずれか（完全一致）。
	Industry string `json:"industry,omitempty"`
	// Location / MainBusiness / TechStack は、出力に含まれているべき語（部分一致）。
	Location     string `json:"location,omitempty"`
	MainBusiness string `json:"main_business,omitempty"`
	TechStack    string `json:"tech_stack,omitempty"`
	// FoundedYear / EmployeeCount は完全一致。
	FoundedYear   int `json:"founded_year,omitempty"`
	EmployeeCount int `json:"employee_count,omitempty"`
	// Absent は事実テキストに無いので空（0）でなければならない項目（JSON のキー名）。
	// 値が入っていれば、根拠の無い値を作った（fabricated）として数える。
	Absent []string `json:"absent,omitempty"`
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
	if _, ok := labelRank[c.Label]; !ok && !(c.Label == "" && slices.Contains(labelOptionalTargets, c.Target)) {
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
	case TargetChatAnswer:
		if strings.TrimSpace(c.Input.Question) == "" || strings.TrimSpace(c.Input.Answer) == "" {
			return fmt.Errorf("input.question と input.answer は必須")
		}
	case TargetCompanyExtract:
		if strings.TrimSpace(c.Input.CompanyName) == "" || strings.TrimSpace(c.Input.FactsText) == "" || c.Input.Expected == nil {
			return fmt.Errorf("input.company_name と input.facts_text と input.expected は必須")
		}
		if companyExtractCheckCount(*c.Input.Expected) == 0 {
			return fmt.Errorf("input.expected に確かめる項目が1つも無い")
		}
		for _, field := range c.Input.Expected.Absent {
			if !slices.Contains(companyExtractFields, field) {
				return fmt.Errorf("input.expected.absent の %q は企業情報の項目ではない", field)
			}
		}
	case TargetMatchingReason:
		if c.Input.Match == nil || c.Input.Match.Company == nil || strings.TrimSpace(c.Input.Match.Company.Name) == "" {
			return fmt.Errorf("input.match.Company.Name は必須")
		}
		if len(c.Input.MustMentionAny) == 0 || len(c.Input.StrengthAny) == 0 {
			return fmt.Errorf("input.must_mention_any と input.strength_any は必須")
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
	case TargetChatAnswer:
		// 質問は全ラベルで共通にできるので、長さの交絡は回答側だけを見る。
		return c.Input.Answer
	case TargetCompanyExtract:
		return c.Input.FactsText
	case TargetMatchingReason:
		if c.Input.Match != nil && c.Input.Match.Company != nil {
			company := c.Input.Match.Company
			return company.Industry + company.MainBusiness + company.Culture + company.WorkStyle
		}
	}
	return ""
}

// InputChars は入力本文の文字数（バイト数ではなく）。
func (c Case) InputChars() int { return len([]rune(c.InputText())) }

// numericTokenPattern は数値トークン（連続した数字）。全角数字も数える。
var numericTokenPattern = regexp.MustCompile(`[0-9０-９]+`)

// InputNumericTokens は入力本文に現れる数値トークンの個数。
//
// 文字数と並ぶ「表層特徴」として測る。履歴書ルーブリックは specificity
// （数値・固有名詞・期間が何種類あるか）/ achievement（結果が数値で示されて
// いるか）/ completeness（項目がいくつ埋まっているか）で新卒の70点を決めて
// おり、評価器が数値の個数を直接数えている。ラベルを数値の個数で付けると
// ゴールデンセットのラベルが評価器と同じ関数で定義される（循環）ので、
// 文字数と同じようにこの相関も上限を固定する（#1593 のレビュー指摘）。
func (c Case) InputNumericTokens() int {
	return len(numericTokenPattern.FindAllString(c.InputText(), -1))
}

// properNounPattern はゴールデンセットで使う固有名詞。合成データなので
// 社名・学校名は「サンプル」「株式会社」で始まる形に統一してある。
// 実在の固有名詞を一般に検出するものではない（そもそも実データは入れない）。
var properNounPattern = regexp.MustCompile(`(?:株式会社|サンプル)[^\s、。]*`)

// InputProperNouns は入力本文に現れる固有名詞の個数。
//
// 文字数・数値トークン数に続く3つ目の表層特徴として測る（#1637）。
// specificity のレベル定義は「数値・固有名詞・期間」を並列に数えるので、
// 数値の交絡を下げたぶんを固有名詞へ移しただけ、という逃げ方ができてしまう。
// 同じ罠を3つ目で塞ぐために上限を固定する。
func (c Case) InputProperNouns() int {
	return len(properNounPattern.FindAllString(c.InputText(), -1))
}
