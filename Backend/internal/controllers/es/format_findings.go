package es

import (
	"encoding/json"
	"errors"

	"Backend/internal/services/shared/textcheck"
)

// formatFinding は ES 添削・ESリライトのレスポンスに載せる形式チェックの指摘（#1530）。
type formatFinding = textcheck.Finding

// formatFindings は学生の原文に形式チェックをかける（#1530）。
//
// 検査は入力テキストだけで決まり、RAG の応答（改善文）には依存しない。そのため
// postESReview の中ではなく、添削・リライトの両ハンドラで原文に対して走らせる。
// 字数の指定が無いときは char_limit を見ない。指摘が無いときも null ではなく [] を返す。
func formatFindings(text string, charLimit *int, mode string) []formatFinding {
	rules := []textcheck.Rule{
		textcheck.MixedWidth{},
		textcheck.MixedNumeral{},
		textcheck.MixedEra{},
		textcheck.MixedStyle{},
		textcheck.DateOrder{},
	}
	if charLimit != nil {
		rules = append(rules, textcheck.CharLimit{Limit: *charLimit, Mode: mode})
	}
	if fs := textcheck.Run(text, rules...); fs != nil {
		return fs
	}
	return []formatFinding{}
}

// withFormatFindings は RAG の応答 JSON に format_findings を足す。
// 添削は RAG の応答を素通しで返しているため、型で受けると型に無い項目が落ちる。
// キーを保ったまま1項目だけ足す。
func withFormatFindings(body []byte, findings []formatFinding) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("RAG の応答が JSON オブジェクトではない")
	}
	raw, err := json.Marshal(findings)
	if err != nil {
		return nil, err
	}
	m["format_findings"] = raw
	return json.Marshal(m)
}
