package aibench

import (
	"Backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/services/company"
	"Backend/internal/services/prompts"
)

// #1782: 企業情報の抽出とマッチング理由の評価対象。

func TestScoreCompanyExtract(t *testing.T) {
	expected := CompanyExtractExpected{
		Industry: "製造業", Location: "浜松", FoundedYear: 1962, EmployeeCount: 1250,
		MainBusiness: "減速機", Absent: []string{"tech_stack", "welfare_details"},
	}
	tests := []struct {
		name       string
		result     company.CompanyInfoResult
		wantPassed int
		wantViol   []string
	}{
		{
			name:       "すべて一致",
			result:     company.CompanyInfoResult{Industry: "製造業", Location: "静岡県 浜松市", FoundedYear: 1962, EmployeeCount: 1250, MainBusiness: "産業用ロボットの減速機の製造"},
			wantPassed: 7,
		},
		{
			name:       "資本金を従業員数にし、技術スタックを作った",
			result:     company.CompanyInfoResult{Industry: "製造業", Location: "浜松市", FoundedYear: 1962, EmployeeCount: 9800, MainBusiness: "減速機", TechStack: "C++"},
			wantPassed: 5,
			wantViol:   []string{"field_mismatch:employee_count", "fabricated:tech_stack"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passed, total, violations := scoreCompanyExtract(tt.result, expected)
			if total != 7 || passed != tt.wantPassed || strings.Join(violations, ",") != strings.Join(tt.wantViol, ",") {
				t.Fatalf("passed=%d total=%d violations=%v", passed, total, violations)
			}
		})
	}
}

func TestEvaluateCompanyExtractResponse(t *testing.T) {
	expected := CompanyExtractExpected{Industry: "商社", Absent: []string{"employee_count"}}
	o := evaluateCompanyExtractResponse(Observation{}, expected, CallResult{Text: `{"industry":"商社","employee_count":0}`})
	if o.Broken || o.Score != 1 || o.RawScore != 2 {
		t.Fatalf("o = %+v", o)
	}
	// 業種の表記ゆれは本番と同じく正規化してから比べる（ParseCompanyInfoResult）
	o = evaluateCompanyExtractResponse(Observation{}, expected, CallResult{Text: `{"industry":"商社","employee_count":30}`})
	if o.Score != 0.5 || len(o.Violations) != 1 || o.Violations[0] != "fabricated:employee_count" {
		t.Fatalf("o = %+v", o)
	}
	for _, res := range []CallResult{
		{Text: "JSONではない"},
		{Truncated: true},
		{Err: errors.New("HTTP 500"), Unmeasured: true},
	} {
		if o := evaluateCompanyExtractResponse(Observation{}, expected, res); !o.Broken {
			t.Fatalf("破損に数えない: %+v", res)
		}
	}
}

// 本番と同じプロンプト・スキーマ（strict）・温度・上限で呼ぶこと。
func TestCompanyExtractTarget_Run呼び出し内容(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		content, _ := json.Marshal(`{"industry":"商社","location":"横浜","founded_year":1978,"employee_count":0}`)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + string(content) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	t.Setenv("OPENAI_API_KEY", "テスト用")

	c := Case{ID: "ce-x", Target: TargetCompanyExtract, Input: Input{
		CompanyName: "みなと商事株式会社", FactsText: "本社: 神奈川県横浜市\n創業: 1978年",
		Expected: &CompanyExtractExpected{Industry: "商社", Location: "横浜", FoundedYear: 1978, Absent: []string{"employee_count"}},
	}}
	o := newCompanyExtractTarget("model-x").Run(context.Background(), c)
	if o.Broken || o.Score != 1 {
		t.Fatalf("o = %+v", o)
	}
	if got["model"] != "model-x" || got["temperature"] != 0.2 || got["max_completion_tokens"] != float64(company.CompanyInfoExtractMaxTokens) {
		t.Errorf("model=%v temperature=%v max=%v", got["model"], got["temperature"], got["max_completion_tokens"])
	}
	rf, _ := got["response_format"].(map[string]any)
	schema, _ := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || schema["strict"] != true || schema["name"] != company.CompanyInfoResponseSchema().Name {
		t.Errorf("response_format = %v", rf)
	}
	msgs, _ := got["messages"].([]any)
	sys, _ := msgs[0].(map[string]any)
	wantSystem, _ := company.CompanyInfoExtractPrompts("", "", "")
	if sys["content"] != wantSystem {
		t.Error("システムプロンプトが本番と違う")
	}
}

func TestScoreMatchingReason(t *testing.T) {
	in := Input{MustMentionAny: []string{"自治体", "業務システム"}, StrengthAny: []string{"技術"}}
	good := "アオバシステムは自治体向けの業務システムを開発しており、チームで設計を議論する文化があります。" +
		"あなたの技術志向の高さは、要件を深く理解して形にする開発の現場で強みになるはずです。" +
		"ハイブリッドな働き方の中で、利用者の声を聞きながら成長できる環境だと考えられます。"
	tests := []struct {
		name     string
		reason   string
		wantViol []string
	}{
		{name: "すべて満たす", reason: good},
		{name: "短く、企業の事実も強みも無い", reason: "良い会社です。おすすめです。", wantViol: []string{"length_out_of_range", "no_company_fact", "no_strength_axis"}},
		{name: "箇条書き", reason: "- " + good, wantViol: []string{"format_violation"}},
		{name: "1文", reason: strings.ReplaceAll(good, "。", "、") + "。", wantViol: []string{"sentence_count"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, total, violations := scoreMatchingReason(tt.reason, in)
			if total != 5 || strings.Join(violations, ",") != strings.Join(tt.wantViol, ",") {
				t.Fatalf("violations = %v（%d字）", violations, runeLen(tt.reason))
			}
		})
	}
}

func TestCountSentences(t *testing.T) {
	for text, want := range map[string]int{"一文。二文。": 2, "一文。二文": 2, "一文": 1, "": 0, "問い？答え！": 2} {
		if got := countSentences(text); got != want {
			t.Errorf("countSentences(%q) = %d, want %d", text, got, want)
		}
	}
}

// 本番と同じく、上限 100 で切れたら 200 で1回だけ呼び直す。
func TestMatchingReasonTarget_Run上限到達で呼び直す(t *testing.T) {
	var limits []float64
	var first map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if first == nil {
			first = body
		}
		limits = append(limits, body["max_output_tokens"].(float64))
		if len(limits) == 1 {
			_, _ = w.Write([]byte(`{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[],"usage":{"input_tokens":10,"output_tokens":100}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"短い理由です。"}]}],"usage":{"input_tokens":10,"output_tokens":20}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	t.Setenv("OPENAI_API_KEY", "テスト用")

	c := Case{ID: "mr-x", Target: TargetMatchingReason, Input: Input{
		Match:          &entity.UserCompanyMatch{MatchScore: 80, Company: models.Company{Name: "テスト株式会社", Industry: "商社"}},
		UserScores:     []entity.UserWeightScore{{WeightCategory: "技術志向", Score: 80}},
		MustMentionAny: []string{"商社"}, StrengthAny: []string{"技術"},
	}}
	o := newMatchingReasonTarget("model-x").Run(context.Background(), c)
	if len(limits) != 2 || limits[0] != 100 || limits[1] != 200 {
		t.Fatalf("max_output_tokens = %v, want [100 200]", limits)
	}
	if o.Broken || o.PromptTokens != 20 || o.CompletionTokens != 120 || !strings.Contains(strings.Join(o.Violations, ","), "retried_max_output_tokens") {
		t.Fatalf("o = %+v", o)
	}
	if first["model"] != "model-x" || first["temperature"] != 0.4 {
		t.Errorf("model=%v temperature=%v", first["model"], first["temperature"])
	}
	if !strings.Contains(string(mustJSON(first)), jsonEscape(prompts.MatchingReasonSystemPrompt[:30])) {
		t.Error("システムプロンプトが本番と違う")
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return strings.Trim(string(b), `"`)
}

// 同梱の固定セットが読み込めること（ラベルを付けない2工程）。
func TestBundledManifests_CompanyAndMatching(t *testing.T) {
	for target, path := range map[string]string{
		TargetCompanyExtract: "../../../docs/research/ai-eval/company-extract.jsonl",
		TargetMatchingReason: "../../../docs/research/ai-eval/matching-reason.jsonl",
	} {
		cases, err := LoadManifest(path)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if len(FilterByTarget(cases, target)) < 10 {
			t.Fatalf("%s: 件数が少ない", target)
		}
		if !LabelOptional(target) {
			t.Fatalf("%s はラベル不要のはず", target)
		}
	}
	// 他の target は今まで通りラベルが必須
	if err := validateCase(Case{ID: "x", Target: TargetChatAnswer, Input: Input{Question: "q", Answer: "a"}}); err == nil {
		t.Fatal("chat-answer のラベル無しを通してしまう")
	}
}

// 固定セットの must_mention_any は、企業側の事実から取った語(架空の企業の項目に書いてある)。
// 読み込み後の Company に、その語がすべて残っていること。
// 固定セットのキーが Company の型の json タグと食い違うと、値が黙って読み込まれず、ここで落ちる。
func TestBundledMatchingReason_MustMentionSurvivesDecode(t *testing.T) {
	cases, err := LoadManifest("../../../docs/research/ai-eval/matching-reason.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range FilterByTarget(cases, TargetMatchingReason) {
		var facts strings.Builder
		v := reflect.ValueOf(c.Input.Match.Company)
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).Kind() == reflect.String {
				facts.WriteString(v.Field(i).String())
				facts.WriteString("\n")
			}
		}
		for _, word := range c.Input.MustMentionAny {
			if !strings.Contains(facts.String(), word) {
				t.Errorf("%s: must_mention_any の %q が、読み込み後の Company のどこにも無い(キーの食い違いの疑い)", c.ID, word)
			}
		}
	}
}

// ラベルなしの target は平均スコアの低下を劣化として検知する（弁別力が無いため）。
func TestDiff_ラベルなしは平均スコアの低下を劣化にする(t *testing.T) {
	prev := &Summary{Target: TargetCompanyExtract, MeasuredRuns: 24, MeanScore: 0.90}
	cur := &Summary{Target: TargetCompanyExtract, MeasuredRuns: 24, MeanScore: 0.80}
	regs, _ := Diff(prev, cur)
	found := false
	for _, r := range regs {
		found = found || r.Metric == "平均スコア"
	}
	if !found {
		t.Fatalf("平均スコアの低下を検知しない: %+v", regs)
	}
	// ラベルありの target と、計測0件の側では比べない
	for _, tc := range []struct{ p, c *Summary }{
		{&Summary{Target: TargetChatAnswer, MeasuredRuns: 10, MeanScore: 0.9}, &Summary{Target: TargetChatAnswer, MeasuredRuns: 10, MeanScore: 0.5}},
		{&Summary{Target: TargetCompanyExtract, MeasuredRuns: 0}, &Summary{Target: TargetCompanyExtract, MeasuredRuns: 24, MeanScore: 0.5}},
	} {
		regs, _ := Diff(tc.p, tc.c)
		for _, r := range regs {
			if r.Metric == "平均スコア" {
				t.Fatalf("比べてはいけない組で平均スコアを比べた: %+v", tc)
			}
		}
	}
}

func TestAggregate_平均スコア(t *testing.T) {
	obs := []Observation{
		{CaseID: "a", Score: 1.0}, {CaseID: "a", Score: 0.5},
		{CaseID: "b", Score: 0.25}, {CaseID: "b", Broken: true, BrokenReason: BrokenJSONInvalid},
	}
	s := Aggregate(TargetCompanyExtract, "m", "e", "t", obs)
	// ケースごとの平均（0.75, 0.25）の平均
	if s.MeanScore != 0.5 {
		t.Fatalf("MeanScore = %v, want 0.5", s.MeanScore)
	}
}

func TestScoreCompanyExtract_勤務形態と福利厚生と従業員数の基準(t *testing.T) {
	e := CompanyExtractExpected{EmployeeCountBasis: "standalone", WorkStyle: "オフィス", WelfareDetails: "住宅手当"}
	ok := company.CompanyInfoResult{EmployeeCountBasis: "standalone", WorkStyle: "オフィス", WelfareDetails: "住宅手当、資格取得支援"}
	if passed, total, _ := scoreCompanyExtract(ok, e); passed != 3 || total != 3 {
		t.Fatalf("passed=%d total=%d", passed, total)
	}
	ng := company.CompanyInfoResult{EmployeeCountBasis: "consolidated", WorkStyle: "リモート"}
	_, _, violations := scoreCompanyExtract(ng, e)
	if strings.Join(violations, ",") != "field_mismatch:employee_count_basis,field_mismatch:work_style,field_mismatch:welfare_details" {
		t.Fatalf("violations = %v", violations)
	}
}
