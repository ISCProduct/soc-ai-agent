package sttbench

import (
	"math"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

const refText = "GoとAWSを使ってDockerで動かします"

// stubReply は差し替えた transcribe の応答。
type stubReply struct {
	text string
	err  error
}

// withStub は ID ごとに決まった応答を返す transcribe に差し替える。
// 集計の母数はAPIを呼ばずに固定できる（実APIでは残高・揺れで固定できない）。
func withStub(t *testing.T, replies map[string]stubReply) {
	t.Helper()
	orig := transcribeFn
	transcribeFn = func(_, _ string, a Audio) (string, int64, error) {
		r, ok := replies[a.ID]
		if !ok {
			t.Errorf("想定外のケースが呼ばれました: %s", a.ID)
		}
		return r.text, 10, r.err
	}
	t.Cleanup(func() { transcribeFn = orig })
}

// fixture は同じ正解テキストのケースと音声を ID 分だけ作る。
func fixture(ids ...string) ([]Case, []Audio) {
	cases := make([]Case, 0, len(ids))
	audios := make([]Audio, 0, len(ids))
	for _, id := range ids {
		cases = append(cases, Case{ID: id, ReferenceText: refText})
		audios = append(audios, Audio{ID: id, Path: id + ".wav", DurationSec: 5})
	}
	return cases, audios
}

// APIエラーは分子へ 0 を足しつつ分母に残っていた。
// 失敗率に比例して CER が良く出るので、残高が途中で切れた run が
// 「CERが改善した」ように見えていた。分母は成功件数でなければならない。
func TestRunModelMeanCERExcludesAPIErrors(t *testing.T) {
	cases, audios := fixture("ok1", "ok2", "ng1", "ng2")
	withStub(t, map[string]stubReply{
		"ok1": {text: refText},                // 一致（CER 0）
		"ok2": {text: "まったく違う文章になってしまいました"},   // 誤変換（CER > 0）
		"ng1": {err: &statusError{code: 500}}, // APIエラー
		"ng2": {err: &statusError{code: 500}}, // APIエラー
	})

	s := RunModel("gpt-4o-mini-transcribe", cases, audios, "")

	if s.ErroredCases != 2 {
		t.Fatalf("ErroredCases = %d, want 2", s.ErroredCases)
	}
	if s.Cases[1].CER <= 0 {
		t.Fatalf("前提が崩れている: 誤変換ケースの CER = %f, want > 0", s.Cases[1].CER)
	}
	// APIエラーは Failed ではなく Errored として立てる（読者が区別できるように）
	for _, i := range []int{2, 3} {
		if !s.Cases[i].Errored || s.Cases[i].Failed {
			t.Errorf("%s: Errored=%v Failed=%v, want Errored=true Failed=false",
				s.Cases[i].ID, s.Cases[i].Errored, s.Cases[i].Failed)
		}
	}

	want := (s.Cases[0].CER + s.Cases[1].CER) / 2
	if math.Abs(s.MeanCER-want) > 1e-9 {
		t.Errorf("MeanCER = %f, want %f（分母は成功2件）", s.MeanCER, want)
	}
	// 旧実装（分母に全件）と一致してはいけない。一致したら退行している。
	if naive := (s.Cases[0].CER + s.Cases[1].CER) / 4; math.Abs(s.MeanCER-naive) < 1e-9 {
		t.Errorf("MeanCER = %f は全件を分母にした値と同じ。APIエラーが分母に残っている", s.MeanCER)
	}
	wantSem := (s.Cases[0].SemanticCER + s.Cases[1].SemanticCER) / 2
	if math.Abs(s.MeanSemanticCER-wantSem) > 1e-9 {
		t.Errorf("MeanSemanticCER = %f, want %f", s.MeanSemanticCER, wantSem)
	}
}

// 認識失敗（空・短すぎる出力）とAPIエラーは別に数える。
// 混ぜると RESULTS の失敗率列からどちらだったか判別できない。
func TestRunModelSeparatesRecognitionFailureFromAPIError(t *testing.T) {
	cases, audios := fixture("ok", "empty", "apierr")
	withStub(t, map[string]stubReply{
		"ok":     {text: refText},
		"empty":  {text: "  "}, // 何も取れていない
		"apierr": {err: &statusError{code: 500}},
	})

	s := RunModel("gpt-4o-transcribe", cases, audios, "")

	if s.ErroredCases != 1 {
		t.Errorf("ErroredCases = %d, want 1", s.ErroredCases)
	}
	if !s.Cases[1].Failed || s.Cases[1].Errored {
		t.Errorf("認識失敗: Failed=%v Errored=%v, want Failed=true Errored=false", s.Cases[1].Failed, s.Cases[1].Errored)
	}
	// 認識失敗率の分母はAPIが成功した2件。APIエラーを混ぜると 1/3 になる。
	if math.Abs(s.FailureRate-0.5) > 1e-9 {
		t.Errorf("FailureRate = %f, want 0.5（分母はAPI成功2件）", s.FailureRate)
	}
	// CER の母数はAPIが成功した2件。認識失敗は全ミス（空出力なのでCER=1.0）
	// として数える。外すと「何も返さないほど成績が良く見える」逆転が起きる。
	want := (s.Cases[0].CER + s.Cases[1].CER) / 2
	if math.Abs(s.MeanCER-want) > 1e-9 {
		t.Errorf("MeanCER = %f, want %f（分母はAPI成功2件）", s.MeanCER, want)
	}
	if s.Cases[1].CER < 0.99 {
		t.Errorf("無音応答のCER = %f, want ~1.0（全ミス）", s.Cases[1].CER)
	}
}

// 残高切れ（429）やキー失効（401）は後続も必ず失敗する。
// 打ち切らないと216件が一瞬で「完走」して空の結果ファイルが残る。
func TestRunModelAbortsOnConsecutive4xx(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		breakWith int // 途中に挟む別系統のステータス（0 なら挟まない）
		wantCases int
		wantAbort bool
	}{
		{name: "429が連続したら打ち切る", code: 429, wantCases: maxConsecutive4xx, wantAbort: true},
		{name: "401が連続したら打ち切る", code: 401, wantCases: maxConsecutive4xx, wantAbort: true},
		// 5xx は一時的な失敗なので連続判定をリセットする。全件走り切る。
		{name: "5xxが挟まれば打ち切らない", code: 429, breakWith: 503, wantCases: 5, wantAbort: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := []string{"c0", "c1", "c2", "c3", "c4"}
			cases, audios := fixture(ids...)
			replies := map[string]stubReply{}
			for i, id := range ids {
				code := tt.code
				if tt.breakWith != 0 && i%2 == 1 {
					code = tt.breakWith
				}
				replies[id] = stubReply{err: &statusError{code: code}}
			}
			withStub(t, replies)

			s := RunModel("gpt-4o-transcribe", cases, audios, "")

			if len(s.Cases) != tt.wantCases {
				t.Errorf("実行件数 = %d, want %d", len(s.Cases), tt.wantCases)
			}
			if s.Aborted != tt.wantAbort {
				t.Errorf("Aborted = %v, want %v", s.Aborted, tt.wantAbort)
			}
			// 全件エラーなので CER は 0 のままだが、母数が無いことが
			// ErroredCases から読める必要がある。
			if s.ErroredCases != len(s.Cases) {
				t.Errorf("ErroredCases = %d, want %d", s.ErroredCases, len(s.Cases))
			}
		})
	}
}

// 補助語の有無を測り分けるには、空のときに prompt を送らないことが前提になる。
// 空文字の prompt を送ると「補助語なし」の測定が成立しない。
func TestBuildTranscribeForm(t *testing.T) {
	tests := []struct {
		name       string
		hints      string
		wantPrompt string
		wantExists bool
	}{
		{name: "補助語なしなら prompt を付けない", hints: "", wantExists: false},
		{name: "補助語ありなら prompt を付ける", hints: "御社, Go, AWS", wantPrompt: "御社, Go, AWS", wantExists: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Audio{ID: "x", Path: "audio/x.wav", Data: []byte("RIFFdummy")}
			buf, contentType, err := buildTranscribeForm("gpt-4o-mini-transcribe", tt.hints, a)
			if err != nil {
				t.Fatalf("buildTranscribeForm: %v", err)
			}
			fields, files := parseForm(t, buf.String(), contentType)

			if got, ok := fields["prompt"]; ok != tt.wantExists || got != tt.wantPrompt {
				t.Errorf("prompt = %q (存在 %v), want %q (存在 %v)", got, ok, tt.wantPrompt, tt.wantExists)
			}
			if fields["model"] != "gpt-4o-mini-transcribe" {
				t.Errorf("model = %q", fields["model"])
			}
			// 言語を固定しないとモデルが英語で書き起こすことがあり、CERが意味を失う
			if fields["language"] != "ja" {
				t.Errorf("language = %q, want ja", fields["language"])
			}
			if files["file"] != "x.wav" {
				t.Errorf("file のファイル名 = %q, want x.wav", files["file"])
			}
		})
	}
}

// parseForm は multipart 本体をフィールド名 -> 値、ファイル名の対応に分解する。
func parseForm(t *testing.T, body, contentType string) (map[string]string, map[string]string) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("Content-Type の解析に失敗: %v", err)
	}
	r := multipart.NewReader(strings.NewReader(body), params["boundary"])
	form, err := r.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("multipart の解析に失敗: %v", err)
	}
	fields := map[string]string{}
	for k, v := range form.Value {
		fields[k] = v[0]
	}
	files := map[string]string{}
	for k, v := range form.File {
		files[k] = v[0].Filename
	}
	return fields, files
}
