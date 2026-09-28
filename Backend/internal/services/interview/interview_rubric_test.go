package interview

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/flywheel"
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

// testUtterances は照合テスト用の面接ログ。
// 面接官(ai)の発話も混ぜてあるのは、それが照合先に入ってしまわないかを見るため。
func testUtterances() []models.InterviewUtterance {
	return []models.InterviewUtterance{
		{Role: "ai", Text: "学生時代に力を入れたことを教えてください。海外留学の経験はありますか？"},
		{Role: "user", Text: "はい、私は大学時代に軽音サークルの代表を務めていました。入学した当初は部員が8人しかいなくて、このままだと廃部になるという話が出ていました。"},
		{Role: "ai", Text: "そこではどう動きましたか？"},
		{Role: "user", Text: "そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。"},
		{Role: "user", Text: "えー、その、あ、すみません、言い直します。プログラミングは独学で、毎日2時間くらい続けています。"},
	}
}

// 完了定義: evidence が実際の受験者発話に基づくかを照合できること（#1527）。
//
// 捏造された根拠は学生向け・教員向けの両レポートに出る。スコアの値域と違って
// スキーマ検証では捕まらないため、実発話との照合が唯一の検出手段になる。
func TestValidateEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		evidence      map[string]string
		utterances    []models.InterviewUtterance
		wantUnmatched []string
		wantChecked   int
		why           string
	}{
		{
			name: "発話と完全一致なら照合できる",
			evidence: map[string]string{
				"logic":         "結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。",
				"specificity":   "入学した当初は部員が8人しかいなくて",
				"ownership":     "そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。",
				"communication": "プログラミングは独学で、毎日2時間くらい続けています。",
				"enthusiasm":    "私は大学時代に軽音サークルの代表を務めていました。",
			},
			utterances:  testUtterances(),
			wantChecked: 5,
			why:         "実際の発言をそのまま引用したのに未照合にしている",
		},
		{
			name: "要約された根拠も照合できる",
			evidence: map[string]string{
				"logic":       "新歓ライブを企画してSNS告知を担当し、部員数を3倍にした",
				"specificity": "部員が8人から23人に増えた",
			},
			utterances:  testUtterances(),
			wantChecked: 2,
			why:         "要約された正しい根拠まで落としている（しきい値が厳しすぎる）",
		},
		{
			name: "表記ゆれ（全角半角・句読点・助詞）も照合できる",
			evidence: map[string]string{
				"logic":         "翌年の新入部員は２３人まで増えて 部員数を３倍にすることができました",
				"specificity":   "入学した当初は部員が8人しかいなくて、",           // 句読点の差
				"ownership":     "私が新歓ライブの企画も提案し、SNSでの告知も担当しました", // 助詞の差
				"communication": "プログラミングは独学で毎日2時間続けている",         // 言い直し
			},
			utterances:  testUtterances(),
			wantChecked: 4,
			why:         "表記ゆれだけで未照合にしている",
		},
		{
			name: "完全に捏造された根拠は未照合になる",
			evidence: map[string]string{
				"logic":         "結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。",
				"specificity":   "TOEICで900点を取得し、英語での商談経験もあります。",
				"ownership":     "高校時代は野球部でキャプテンを務め、県大会でベスト8に入りました。",
				"communication": "アルバイト先で売上を前年比150%に伸ばし、店長から表彰されました。",
			},
			utterances:    testUtterances(),
			wantUnmatched: []string{"communication", "ownership", "specificity"},
			wantChecked:   4,
			why:           "言っていない発言を根拠として通している",
		},
		{
			name: "面接官の発話は根拠として認めない",
			evidence: map[string]string{
				"logic": "学生時代に力を入れたことを教えてください。海外留学の経験はありますか？",
			},
			utterances:    testUtterances(),
			wantUnmatched: []string{"logic"},
			wantChecked:   1,
			why:           "面接官の質問文をそのまま根拠にできてしまっている",
		},
		{
			name: "受験者の発話が無ければ全項目が未照合になる",
			evidence: map[string]string{
				"logic":       "結果として、部員数を3倍にすることができました。",
				"specificity": "部員が8人しかいなかった",
			},
			utterances: []models.InterviewUtterance{
				{Role: "ai", Text: "学生時代に力を入れたことを教えてください。"},
			},
			wantUnmatched: []string{"logic", "specificity"},
			wantChecked:   2,
			why:           "照合先が無いのに根拠を通している",
		},
		{
			name:       "発話そのものが無ければ全項目が未照合になる",
			evidence:   map[string]string{"logic": "部員数を3倍にした"},
			utterances: nil,
			// レポート自体は保存される（この関数は未照合キーを返すだけで、生成を落とさない）
			wantUnmatched: []string{"logic"},
			wantChecked:   1,
			why:           "発話0件なのに根拠を通している",
		},
		{
			name:        "空文字の項目は照合対象にしない",
			evidence:    map[string]string{"logic": "", "specificity": "   "},
			utterances:  testUtterances(),
			wantChecked: 0,
			why:         "既に欠落している項目を未照合として扱い、無駄な再生成を招いている",
		},
		{
			name: "記号や絵文字だけの項目は未照合にする",
			evidence: map[string]string{
				"logic":         "。。。！？",
				"specificity":   "🎉🎉🎉",
				"ownership":     "-----",
				"communication": "???",
			},
			utterances:    testUtterances(),
			wantUnmatched: []string{"communication", "logic", "ownership", "specificity"},
			wantChecked:   4,
			why:           "正規化すると空になる文字列を照合成功として通し、そのまま画面に出している",
		},
		{
			name: "しきい値の境界",
			evidence: map[string]string{
				// 0.2857: 「私が新歓ライブの企画を提案して」の言い換え。通すべき
				"logic": "自分から提案した",
				// 0.1818: 発話に無い内容。弾くべき
				"specificity": "顧問と相談した",
			},
			utterances:    testUtterances(),
			wantUnmatched: []string{"specificity"},
			wantChecked:   2,
			why:           "しきい値が境界の内側/外側に動いている（EvidenceMatchThreshold の実測表を引き直すこと）",
		},
		{
			name:        "evidence が空なら何も返さない",
			evidence:    nil,
			utterances:  testUtterances(),
			wantChecked: 0,
			why:         "検証対象が無いのに未照合を報告している",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ValidateEvidence(tt.evidence, SpokenText(tt.utterances))
			if !slices.Equal(got.Unmatched, tt.wantUnmatched) {
				t.Errorf("未照合=%v want %v: %s", got.Unmatched, tt.wantUnmatched, tt.why)
			}
			// Checked は照合率ログの分母。空文字を数えると指標が上方に歪む。
			if got.Checked != tt.wantChecked {
				t.Errorf("照合対象数=%d want %d", got.Checked, tt.wantChecked)
			}
			if got.Matched() != tt.wantChecked-len(tt.wantUnmatched) {
				t.Errorf("Matched()=%d want %d", got.Matched(), tt.wantChecked-len(tt.wantUnmatched))
			}
		})
	}
}

// しきい値そのものを固定する（#1527）。
//
// 実測分布から決めた値なので、勘で動かされると捏造が通るか正当な要約が落ちる。
// EvidenceMatchThreshold を書き換えるなら、この定数と
// interview_rubric.go / docs/wiki/scoring.md §2-4 の実測表を必ず一緒に引き直すこと。
func TestEvidenceMatchThreshold_Pinned(t *testing.T) {
	t.Parallel()
	if EvidenceMatchThreshold != 0.25 {
		t.Errorf("EvidenceMatchThreshold = %v。実測表を引き直さずに変更していないか確認すること", EvidenceMatchThreshold)
	}
}

// 照合で検出できない捏造を明示的に固定する（#1527 / Issue #1566）。
//
// 文字bigramの一致度である以上、**発話の言い回しを流用した捏造は弾けない**。
// これを「たまたま通っている」ではなくテストで明示しておく。
// #1566 でアルゴリズムを直したらここが落ちるので、
// interview_rubric.go と docs/wiki/scoring.md §2-4 の表も一緒に更新すること。
func TestValidateEvidence_KnownLimitation(t *testing.T) {
	t.Parallel()

	spoken := SpokenText(testUtterances())

	tests := []struct {
		name     string
		evidence map[string]string
		why      string
	}{
		{
			name: "述語末尾を流用した捏造（内容語は100%でっち上げ）",
			evidence: map[string]string{
				// 「〜することができました」「〜を担当しました」「〜を提案して」だけが発話由来
				"logic":       "国際特許を3件取得することができました",
				"specificity": "学部長賞の選考を担当しました",
				"ownership":   "研究室のサーバー移行を提案して",
			},
			why: "実測 0.27〜0.56。発話の述語を流用すると内容が全部嘘でも通る",
		},
		{
			name: "実引用に事実を継ぎ足した捏造",
			evidence: map[string]string{
				// 前半は実発話、後半は捏造（部長・3年間・50人はどこにも出てこない）
				"logic": "入学した当初は部員が8人しかいなくて、私が部長として3年間で部員数を50人まで増やしました",
				// 発話の語彙を転記しつつ事実を入れ替えた捏造
				"specificity": "部員が8人から100人に増えて、部費を3倍にすることができました",
			},
			why: "実測 0.45〜0.58。正当な要約（0.32〜0.56）と同じ帯に入る",
		},
		{
			name: "フィラー・相槌をそのまま根拠にしたもの",
			evidence: map[string]string{
				// 発話の部分文字列なので完全一致になる
				"logic":       "はい",
				"specificity": "すみません",
				"ownership":   "えー、その、あ",
			},
			why: "実測 1.00。部分文字列判定が効くため、中身が無くても最高点になる",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidateEvidence(tt.evidence, spoken); len(got.Unmatched) != 0 {
				t.Errorf("未照合=%v（%s）。弾けるようになったなら "+
					"interview_rubric.go と docs/wiki/scoring.md §2-4 の「検出できないもの」を更新すること",
					got.Unmatched, tt.why)
			}
		})
	}
}

// 一部だけ未照合のとき、その項目だけ空になり他は残ること（#1527）。
func TestBlankUnmatchedEvidence(t *testing.T) {
	t.Parallel()

	evidence := map[string]string{
		"logic":       "結果として、部員数を3倍にすることができました。",
		"specificity": "TOEICで900点を取得しました。",
		"ownership":   "私が新歓ライブの企画を提案して",
	}
	got := evidence
	blankUnmatchedEvidence(got, []string{"specificity", "unknown_key"})

	if got["specificity"] != "" {
		t.Errorf("未照合の項目が残っている: %q", got["specificity"])
	}
	if got["logic"] == "" || got["ownership"] == "" {
		t.Errorf("照合できた項目まで空にしている: %v", got)
	}
	if _, ok := got["specificity"]; !ok {
		t.Error("キーごと消している。中身の有無で欠落を表すべき")
	}
	if _, ok := got["unknown_key"]; ok {
		t.Error("存在しないキーを追加している")
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

// TestRubricKeys_MatchesFlywheelMapping はプロンプトのルーブリックキーと
// user_weight_scores への写像キーが一致していることを固定する（#1554）。
//
// ずれると flywheel 側は interviewScores からその項目を引けず、
// 対応カテゴリが一切書かれないまま成功が返る（ログも出ない）。
// flywheel は interview を import できない（interview → flywheel の依存がある）ため、
// 一致の検証はこちら側に置く。
func TestRubricKeys_MatchesFlywheelMapping(t *testing.T) {
	prompt := slices.Sorted(slices.Values(RubricKeys()))
	mapped := slices.Sorted(slices.Values(flywheel.InterviewRubricKeys()))

	if !slices.Equal(prompt, mapped) {
		t.Errorf("プロンプトのキー %v と写像のキー %v が一致していない。"+
			"どちらかを変えたら両方を揃えること（flywheel/cross_feature_integration_service.go）",
			prompt, mapped)
	}
}
