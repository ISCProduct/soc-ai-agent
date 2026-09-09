package interview

import (
	"errors"
	"strings"
	"testing"
)

// 完了定義: スキーマ違反の LLM 出力を検知して弾けること。
func TestValidateRubricScores_RejectsViolations(t *testing.T) {
	valid := map[string]int{
		"logic": 3, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4,
	}
	withChange := func(mutate func(map[string]int)) map[string]int {
		m := map[string]int{}
		for k, v := range valid {
			m[k] = v
		}
		mutate(m)
		return m
	}

	tests := []struct {
		name    string
		scores  map[string]int
		wantErr string
	}{
		{"正常", valid, ""},
		{"境界値の下限", withChange(func(m map[string]int) { m["logic"] = RubricScoreMin }), ""},
		{"境界値の上限", withChange(func(m map[string]int) { m["logic"] = RubricScoreMax }), ""},
		{"nil", nil, "スコアが空"},
		{"空", map[string]int{}, "スコアが空"},
		{"項目欠落", withChange(func(m map[string]int) { delete(m, "ownership") }), "欠けている"},
		{"未知の項目", withChange(func(m map[string]int) { m["creativity"] = 3 }), "未知の評価項目"},
		{"上限超過", withChange(func(m map[string]int) { m["logic"] = 10 }), "範囲外"},
		{"100点満点で返した", withChange(func(m map[string]int) { m["specificity"] = 80 }), "範囲外"},
		{"負の値", withChange(func(m map[string]int) { m["enthusiasm"] = -1 }), "範囲外"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRubricScores(tt.scores)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("弾くべきでない出力を弾いた: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("スキーマ違反を通した")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want に %q を含む", err, tt.wantErr)
			}
		})
	}
}

// エラー文にどの項目が問題かが出ること。出ないと運用時に原因が追えない。
func TestValidateRubricScores_ErrorNamesOffendingKey(t *testing.T) {
	err := ValidateRubricScores(map[string]int{
		"logic": 99, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4,
	})
	if err == nil || !strings.Contains(err.Error(), "logic=99") {
		t.Errorf("問題の項目を示していない: %v", err)
	}
}

// 完了定義: 面接ログから各項目のスコアと理由を含む JSON が生成されること。
func TestParseReportPayload(t *testing.T) {
	raw := "```json\n" + `{
  "summary": "落ち着いて回答できていました。",
  "scores": {"logic": 3, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4},
  "evidence": {"logic": "結論から述べていた", "specificity": "数値がなかった", "ownership": "私が提案した", "communication": "簡潔だった", "enthusiasm": "志望理由が具体的"},
  "strengths": ["結論から話せる"],
  "improvements": ["数値を添える"],
  "teacher": {"overall_comment": "指導しやすい", "coaching_points": ["数値を促す"]}
}` + "\n```"

	got, err := parseReportPayload(raw)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, key := range RubricKeys() {
		if _, ok := got.Scores[key]; !ok {
			t.Errorf("スコア %s が無い", key)
		}
		if got.Evidence[key] == "" {
			t.Errorf("根拠 %s が無い", key)
		}
	}
	if got.Teacher == nil || got.Teacher.OverallComment == "" {
		t.Error("教員向けの内容が読めていない")
	}
}

// スキーマ違反はレポートとして採用しない。
func TestParseReportPayload_RejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"JSONとして壊れている", `{"summary": "途中で切れ`},
		{"scoresが無い", `{"summary": "よかった", "strengths": []}`},
		{"スコアが値域外", `{"summary": "s", "scores": {"logic": 8, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4}}`},
		{"項目が欠けている", `{"summary": "s", "scores": {"logic": 3, "specificity": 2}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseReportPayload(tt.raw); err == nil {
				t.Error("スキーマ違反を通した")
			}
		})
	}
}

// プロンプトの評価基準は定義から生成する。
// 定義とプロンプトが別々だと、片方だけ増えたときに静かに欠落する（#795）。
func TestBuildRubricPromptSection_CoversAllCriteria(t *testing.T) {
	section := BuildRubricPromptSection()
	for _, c := range RubricCriteria() {
		for _, want := range []string{c.Key, c.Label, c.Description} {
			if !strings.Contains(section, want) {
				t.Errorf("プロンプトに %q が無い", want)
			}
		}
	}
	if !strings.Contains(section, "0〜5の整数") {
		t.Errorf("値域を明示していない: %s", section)
	}
}

// 検証側とプロンプト側が同じ項目を見ていること。
func TestRubricKeys_MatchesValidation(t *testing.T) {
	scores := map[string]int{}
	for _, key := range RubricKeys() {
		scores[key] = 3
	}
	if err := ValidateRubricScores(scores); err != nil {
		t.Errorf("プロンプトが求める項目を検証が弾いた: %v", err)
	}
}

// スコアだけが不正なとき、講評は学生へ届けつつスコアは捨てる。
// レポートごと捨てると、面接したのに何も表示されない。
func TestDropInvalidScores_KeepsBody(t *testing.T) {
	raw := `{
  "summary": "落ち着いて回答できていました。",
  "scores": {"logic": 99, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4},
  "evidence": {"logic": "結論から述べていた"},
  "strengths": ["結論から話せる"],
  "improvements": ["数値を添える"],
  "teacher": {"overall_comment": "指導しやすい"}
}`
	// JSON としては読める
	payload, err := parseReportJSON(raw)
	if err != nil {
		t.Fatalf("本文が読めない: %v", err)
	}
	// スコアは不正
	if err := ValidateRubricScores(payload.Scores); err == nil {
		t.Fatal("値域外のスコアを通した")
	}

	got := dropInvalidScores(payload)
	if got.Summary == "" || len(got.Strengths) == 0 || len(got.Improvements) == 0 {
		t.Error("講評まで捨てている")
	}
	if got.Teacher == nil {
		t.Error("教員向けの内容まで捨てている")
	}
	if len(got.Scores) != 0 || len(got.Evidence) != 0 {
		t.Errorf("不正なスコアが残っている: scores=%v evidence=%v", got.Scores, got.Evidence)
	}
}

// 空のスコアは "null" や "{}" ではなく空文字で保存する。
//
// "null"/"{}" だと UpdateScoresFromInterviewReport の
// ScoresJSON == "" 早期リターンに乗らずスコア反映へ進み、
// 画面側も空オブジェクトを truthy と見て平均が NaN になる。
func TestMarshalOrEmpty(t *testing.T) {
	if got := marshalOrEmpty(map[string]int(nil)); got != "" {
		t.Errorf("nil map = %q, want 空文字", got)
	}
	if got := marshalOrEmpty(map[string]int{}); got != "" {
		t.Errorf("空 map = %q, want 空文字", got)
	}
	if got := marshalOrEmpty(map[string]string(nil)); got != "" {
		t.Errorf("nil map(string) = %q, want 空文字", got)
	}
	if got := marshalOrEmpty(map[string]int{"logic": 3}); got != `{"logic":3}` {
		t.Errorf("中身があるとき = %q", got)
	}
}

// 保存すべき内容の決定。JSONが読めたかとスコアの妥当性で3通りに分かれる。
func TestFinalizeReportPayload(t *testing.T) {
	body := reportPayload{
		Summary:      "落ち着いて回答できていました。",
		Scores:       map[string]int{"logic": 99},
		Evidence:     map[string]string{"logic": "根拠"},
		Strengths:    []string{"結論から話せる"},
		Improvements: []string{"数値を添える"},
	}

	t.Run("JSONが読めなければ保存しない", func(t *testing.T) {
		if _, err := finalizeReportPayload(reportPayload{}, false, errors.New("broken")); err == nil {
			t.Error("読めていないのに保存しようとした")
		}
	})

	t.Run("読めなかったのに理由が無くてもエラーにする", func(t *testing.T) {
		if _, err := finalizeReportPayload(reportPayload{}, false, nil); err == nil {
			t.Error("理由が無いと成功扱いになっている")
		}
	})

	t.Run("スコアが妥当ならそのまま", func(t *testing.T) {
		got, err := finalizeReportPayload(body, true, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got.Scores) == 0 {
			t.Error("妥当なスコアまで捨てている")
		}
	})

	t.Run("スコアだけ不正なら講評を残してスコアを捨てる", func(t *testing.T) {
		got, err := finalizeReportPayload(body, true, errors.New("範囲外"))
		if err != nil {
			t.Fatalf("レポートごと捨てている: %v", err)
		}
		if got.Summary == "" || len(got.Strengths) == 0 {
			t.Error("講評まで捨てている")
		}
		if len(got.Scores) != 0 || len(got.Evidence) != 0 {
			t.Errorf("不正なスコアが残っている: %v / %v", got.Scores, got.Evidence)
		}
	})
}
