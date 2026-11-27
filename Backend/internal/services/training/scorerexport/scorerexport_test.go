package scorerexport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/training/masking"
)

// testKey は 32 バイトちょうどの鍵（MinHMACKeyBytes の境界）。
var testKey = []byte("0123456789abcdef0123456789abcdef")

func testConfig() Config {
	return Config{HMACKey: testKey, Bucket: "train-bucket", Prefix: "scorer/private"}
}

const validScores = `{"logic":3,"specificity":4,"ownership":2,"communication":5,"enthusiasm":0}`

type finishCall struct {
	id                 uint
	status             string
	students, sessions int
	errMsg             string
	ctxErr             error
}

// fakeStore は Store の差し替え。学生の判定は SQL 側（リポジトリのテスト）で確かめるので、
// ここでは一覧と単体の判定の返り値をテストごとに決める。
type fakeStore struct {
	subjects   []repositories.TrainingSubject
	eligible   map[uint]bool
	sessions   map[uint][]repositories.TrainingSession
	utterances map[uint][]repositories.TrainingUtterance
	terms      repositories.TrainingMaskingTerms

	listSubjectsErr error
	startErr        error
	finishErr       error
	termsErr        func(ctx context.Context) error

	startedDest string
	finished    []finishCall
	eligCalls   []uint
	// eligSeq を設定すると、その学生の判定は呼ばれるたびに先頭から順に返す（書く前と書いた後で変えるため）。
	eligSeq map[uint][]bool
	eligErr map[uint][]error

	// 書き出し済みのデータの削除（Sweep）で使う。eligibleIDs が nil なら、subjects のうち eligible のものを返す。
	eligibleIDs    []uint
	eligibleIDsErr error
	sweepStartErr  error
	sweepFinishErr error
	sweepStarted   []string
	sweepFinished  []sweepFinishCall
}

type sweepFinishCall struct {
	id           uint
	status       string
	deletedFiles int
	errMsg       string
	ctxErr       error
}

func (f *fakeStore) ListTrainingEligibleUserIDs(context.Context) ([]uint, error) {
	if f.eligibleIDsErr != nil {
		return nil, f.eligibleIDsErr
	}
	if f.eligibleIDs != nil {
		return f.eligibleIDs, nil
	}
	var ids []uint
	for _, s := range f.subjects {
		if f.eligible[s.UserID] {
			ids = append(ids, s.UserID)
		}
	}
	return ids, nil
}

func (f *fakeStore) StartTrainingSweepRun(_ context.Context, destination string, _ time.Time) (uint, error) {
	f.sweepStarted = append(f.sweepStarted, destination)
	return 77, f.sweepStartErr
}

func (f *fakeStore) FinishTrainingSweepRun(ctx context.Context, id uint, status string, deletedFiles int, errMsg string, _ time.Time) error {
	f.sweepFinished = append(f.sweepFinished, sweepFinishCall{id: id, status: status, deletedFiles: deletedFiles, errMsg: errMsg, ctxErr: ctx.Err()})
	return f.sweepFinishErr
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) IsTrainingEligible(_ context.Context, userID uint) (bool, error) {
	f.eligCalls = append(f.eligCalls, userID)
	if errs := f.eligErr[userID]; len(errs) > 0 {
		f.eligErr[userID] = errs[1:]
		if errs[0] != nil {
			return false, errs[0]
		}
	}
	if seq := f.eligSeq[userID]; len(seq) > 0 {
		f.eligSeq[userID] = seq[1:]
		return seq[0], nil
	}
	return f.eligible[userID], nil
}

func (f *fakeStore) ListTrainingSubjects(context.Context) ([]repositories.TrainingSubject, error) {
	return f.subjects, f.listSubjectsErr
}

func (f *fakeStore) ListTrainingSessions(_ context.Context, userID uint) ([]repositories.TrainingSession, error) {
	return f.sessions[userID], nil
}

func (f *fakeStore) ListTrainingUtterances(_ context.Context, sessionID uint) ([]repositories.TrainingUtterance, error) {
	return f.utterances[sessionID], nil
}

func (f *fakeStore) ListTrainingMaskingTerms(ctx context.Context) (repositories.TrainingMaskingTerms, error) {
	if f.termsErr != nil {
		if err := f.termsErr(ctx); err != nil {
			return repositories.TrainingMaskingTerms{}, err
		}
	}
	return f.terms, nil
}

func (f *fakeStore) StartTrainingExportRun(_ context.Context, destination string, _ time.Time) (uint, error) {
	f.startedDest = destination
	return 42, f.startErr
}

func (f *fakeStore) FinishTrainingExportRun(ctx context.Context, id uint, status string, students, sessions int, errMsg string, _ time.Time) error {
	f.finished = append(f.finished, finishCall{id: id, status: status, students: students, sessions: sessions, errMsg: errMsg, ctxErr: ctx.Err()})
	return f.finishErr
}

type putCall struct {
	key, contentType string
	body             []byte
}

// errPut は fakeWriter の書き込みの失敗。
var errPut = errors.New("put failed")

// errDB は fakeStore の DB の失敗。
var errDB = errors.New("db down")

// fakeWriter は ObjectWriter の差し替え。failOn を含むキーへの書き込みを失敗させる。
type fakeWriter struct {
	puts      []putCall
	failOn    string
	deletes   []string
	deleteErr error
	// deleteFailOn を含むキーの削除を失敗させる。
	deleteFailOn string
	// stored は今あるオブジェクト（PutObject で増え、DeleteObject で消える）。ListKeys・GetObject が読む。
	stored  map[string][]byte
	listErr error
	getErr  error
	// afterList は ListKeys が一覧を作った後に呼ぶ（一覧とマニフェストの読み込みの間に、ほかの処理が書く場面を作る）。
	afterList func(prefix string)
	lists     []string
}

func (w *fakeWriter) DeleteObject(_ context.Context, key string) error {
	if w.deleteErr != nil {
		return w.deleteErr
	}
	if w.deleteFailOn != "" && strings.Contains(key, w.deleteFailOn) {
		return fmt.Errorf("AccessDenied on resource arn:aws:s3:::train-bucket/%s: delete denied", key)
	}
	w.deletes = append(w.deletes, key)
	delete(w.stored, key)
	return nil
}

func (w *fakeWriter) ListKeys(_ context.Context, prefix string) ([]string, error) {
	if w.listErr != nil {
		return nil, w.listErr
	}
	var keys []string
	for k := range w.stored {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	w.lists = append(w.lists, prefix)
	if w.afterList != nil {
		w.afterList(prefix)
	}
	return keys, nil
}

func (w *fakeWriter) GetObject(_ context.Context, key string) ([]byte, error) {
	if w.getErr != nil {
		return nil, w.getErr
	}
	b, ok := w.stored[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return bytes.Clone(b), nil
}

var _ ObjectStore = (*fakeWriter)(nil)

func (w *fakeWriter) PutObject(_ context.Context, key string, body []byte, contentType string) error {
	if w.failOn != "" && strings.Contains(key, w.failOn) {
		// 本物の S3 の AccessDenied と同じく、エラーにキーを含める（監査ログで伏せることを確かめるため）。
		return fmt.Errorf("AccessDenied on resource arn:aws:s3:::train-bucket/%s: %w", key, errPut)
	}
	w.puts = append(w.puts, putCall{key: key, contentType: contentType, body: bytes.Clone(body)})
	if w.stored == nil {
		w.stored = map[string][]byte{}
	}
	w.stored[key] = bytes.Clone(body)
	return nil
}

func (w *fakeWriter) find(t *testing.T, suffix string) putCall {
	t.Helper()
	for _, p := range w.puts {
		if strings.HasSuffix(p.key, suffix) {
			return p
		}
	}
	t.Fatalf("%q で終わるキーへの書き込みがありません: %v", suffix, w.keys())
	return putCall{}
}

func (w *fakeWriter) keys() []string {
	out := make([]string, len(w.puts))
	for i, p := range w.puts {
		out[i] = p.key
	}
	return out
}

var fixedNow = time.Date(2026, 11, 17, 1, 2, 3, 0, time.UTC)

func newTestJob(t *testing.T, store Store, writer ObjectWriter) *Job {
	t.Helper()
	j, err := NewJob(testConfig(), store, writer)
	if err != nil {
		t.Fatal(err)
	}
	j.now = func() time.Time { return fixedNow }
	j.runSuffix = func() string { return "a1b2c3d4" }
	return j
}

func wantHMAC(kind string, id uint) string {
	mac := hmac.New(sha256.New, testKey)
	fmt.Fprintf(mac, "%s:%d", kind, id)
	return hex.EncodeToString(mac.Sum(nil))
}

func decodeSamples(t *testing.T, body []byte) []Sample {
	t.Helper()
	var out []Sample
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		var s Sample
		if err := json.Unmarshal(sc.Bytes(), &s); err != nil {
			t.Fatalf("JSONL の行が JSON でない: %v: %s", err, sc.Text())
		}
		out = append(out, s)
	}
	return out
}

// leakCase は置き換える項目ごとの、発話に混ぜる個人の情報。
// 項目（maskingRules）を足したら、ここにも例を足す。無ければ TestRun_NoPersonalDataInOutput が落ちる。
var leakCases = map[string]struct {
	utterance string
	leaks     []string
}{
	masking.KeyName:    {"山田太郎と申します。山田と呼ばれています。", []string{"山田", "太郎"}},
	masking.KeyEmail:   {"連絡先は hanako.contact@mail.example.jp です。", []string{"hanako.contact", "mail.example.jp"}},
	masking.KeySchool:  {"東京情報専門学校で学んでいます。", []string{"東京情報専門学校"}},
	masking.KeyCompany: {"株式会社サンプル商事のインターンに参加しました。", []string{"サンプル商事"}},
	masking.KeyPhone:   {"電話は 090-1234-5678 です。", []string{"090-1234-5678", "1234-5678"}},
	masking.KeyAddress: {"住所は東京都千代田区霞が関1-2-3です。", []string{"千代田区", "霞が関1-2-3"}},
	keyKnownEmail:      {"本人のアドレスは taro.yamada@example です。", []string{"taro.yamada"}},
}

func leakFixture() *fakeStore {
	subj := repositories.TrainingSubject{UserID: 7, Name: "山田 太郎", Email: "taro.yamada@example.ac.jp", SchoolName: "東京情報専門学校"}
	utts := []repositories.TrainingUtterance{{SessionID: 100, Role: "ai", Text: "山田さん、自己紹介をお願いします。"}}
	for _, r := range maskingRules() {
		if c, ok := leakCases[r.Key]; ok {
			utts = append(utts, repositories.TrainingUtterance{SessionID: 100, Role: "user", Text: c.utterance})
		}
	}
	return &fakeStore{
		subjects:   []repositories.TrainingSubject{subj},
		eligible:   map[uint]bool{7: true},
		sessions:   map[uint][]repositories.TrainingSession{7: {{SessionID: 100, CompanyName: "株式会社サンプル商事", TemplateVersion: "v1", ScoresJSON: validScores}}},
		utterances: map[uint][]repositories.TrainingUtterance{100: utts},
		terms:      repositories.TrainingMaskingTerms{Schools: []string{"東京情報専門学校"}, Companies: []string{"株式会社サンプル商事"}},
	}
}

// TestRun_NoPersonalDataInOutput は、発話に含まれる全項目（氏名・メール・学校名・企業名・電話番号・住所）が
// 書き出した内容（JSONL とマニフェスト）に残らず、置き換えの記号になっていることを確かめる（受け入れ条件）。
// 項目は maskingRules を回すので、項目を足すとこのテストの対象にもなる。
func TestRun_NoPersonalDataInOutput(t *testing.T) {
	for _, r := range maskingRules() {
		if _, ok := leakCases[r.Key]; !ok {
			t.Fatalf("項目 %q の例が leakCases にありません", r.Key)
		}
	}

	store := leakFixture()
	w := &fakeWriter{}
	if _, err := newTestJob(t, store, w).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var all bytes.Buffer
	for _, p := range w.puts {
		all.Write(p.body)
	}
	out := all.String()

	for _, r := range maskingRules() {
		c := leakCases[r.Key]
		t.Run(r.Key, func(t *testing.T) {
			for _, leak := range c.leaks {
				if strings.Contains(out, leak) {
					t.Errorf("%q が出力に残っています", leak)
				}
			}
			if !strings.Contains(out, r.Token) {
				t.Errorf("置き換えの記号 %s が出力にありません", r.Token)
			}
		})
	}
	// AI の質問の中の氏名も置き換わる。
	samples := decodeSamples(t, w.find(t, ".jsonl").body)
	if got := samples[0].Turns[0].Q; got != "<NAME>さん、自己紹介をお願いします。" {
		t.Errorf("質問 = %q", got)
	}
}

// TestRun_WritesSamplesAndManifest は成功したときの出力の形・ラベル・監査ログを確かめる。
func TestRun_WritesSamplesAndManifest(t *testing.T) {
	store := leakFixture()
	store.sessions[7] = append(store.sessions[7], repositories.TrainingSession{SessionID: 101, TemplateVersion: "v2", ScoresJSON: validScores})
	store.utterances[101] = []repositories.TrainingUtterance{
		{SessionID: 101, Role: "ai", Text: "志望動機を教えてください。"},
		{SessionID: 101, Role: "user", Text: "ものづくりが好きだからです。"},
	}
	w := &fakeWriter{}
	res, err := newTestJob(t, store, w).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	runPrefix := "scorer/private/runs/20261117T010203Z-a1b2c3d4/"
	wantDest := "s3://train-bucket/" + runPrefix
	if res.Destination != wantDest || store.startedDest != wantDest {
		t.Errorf("出力先 = %q（監査ログ %q）, want %q", res.Destination, store.startedDest, wantDest)
	}
	if res.Students != 1 || res.Sessions != 2 || res.SkippedSessions != 0 {
		t.Errorf("Result = %+v", res)
	}

	subject := wantHMAC("user", 7)
	if len(w.puts) != 2 {
		t.Fatalf("書き込み = %v, want 学生のファイルとマニフェストの2つ", w.keys())
	}
	if w.puts[0].key != runPrefix+"subjects/"+subject+".jsonl" || w.puts[0].contentType != "application/x-ndjson" {
		t.Errorf("1つ目の書き込み = %q (%s)", w.puts[0].key, w.puts[0].contentType)
	}
	// マニフェストは最後（これがある回だけが書き出しを終えた回、という約束のため）。
	if w.puts[1].key != runPrefix+"manifest.json" || w.puts[1].contentType != "application/json" {
		t.Errorf("最後の書き込み = %q (%s), want マニフェスト", w.puts[1].key, w.puts[1].contentType)
	}

	samples := decodeSamples(t, w.puts[0].body)
	if len(samples) != 2 {
		t.Fatalf("行数 = %d, want 2", len(samples))
	}
	s := samples[1]
	if s.Subject != subject || s.SampleID != wantHMAC("session", 101) {
		t.Errorf("subject/sample_id = %q / %q", s.Subject, s.SampleID)
	}
	if s.LabelSource != "llm" || s.FormatVersion != FormatVersion || s.TemplateVersion != "v2" {
		t.Errorf("行 = %+v", s)
	}
	wantLabels := map[string]int{"logic": 3, "specificity": 4, "ownership": 2, "communication": 5, "enthusiasm": 0}
	if fmt.Sprint(s.Labels) != fmt.Sprint(wantLabels) {
		t.Errorf("labels = %v, want %v", s.Labels, wantLabels)
	}
	if len(s.Turns) != 1 || s.Turns[0] != (Turn{Q: "志望動機を教えてください。", A: "ものづくりが好きだからです。"}) {
		t.Errorf("turns = %+v", s.Turns)
	}
	// 面接 ID・利用者 ID をそのまま書かない。
	if strings.Contains(string(w.puts[0].body), `"session_id"`) || strings.Contains(string(w.puts[0].body), `"user_id"`) {
		t.Errorf("DB の ID が出力にあります: %s", w.puts[0].body)
	}

	var m Manifest
	if err := json.Unmarshal(w.puts[1].body, &m); err != nil {
		t.Fatal(err)
	}
	if m.LabelSource != "llm" || !strings.Contains(m.LabelNote, "LLM ラベルで学習") {
		t.Errorf("マニフェストにラベルの出どころが無い: %+v", m)
	}
	if m.RunID != "20261117T010203Z-a1b2c3d4" || m.StudentCount != 1 || m.SessionCount != 2 ||
		len(m.Files) != 1 || m.Files[0] != "subjects/"+subject+".jsonl" || !m.ExportedAt.Equal(fixedNow) {
		t.Errorf("マニフェスト = %+v", m)
	}
	// ファイルごとの面接の数（消すときに学生のファイルを読まずに session_count を直すため）。
	if fmt.Sprint(m.FileSessions) != fmt.Sprintf("map[subjects/%s.jsonl:2]", subject) {
		t.Errorf("file_sessions = %v", m.FileSessions)
	}
	if len(m.MaskingTokens) != 6 || len(m.RubricKeys) != 5 {
		t.Errorf("記号 %v / ルーブリック %v", m.MaskingTokens, m.RubricKeys)
	}

	want := []finishCall{{id: 42, status: models.TrainingScorerExportSucceeded, students: 1, sessions: 2}}
	if fmt.Sprint(store.finished) != fmt.Sprint(want) {
		t.Errorf("監査ログ = %+v, want %+v", store.finished, want)
	}
}

// TestRun_SkipsIneligibleAtWriteTime は、一覧を取ったあとに同意を取り消した（判定が false になった）学生を
// 書かないことを確かめる。書く直前の判定を外すと、このテストが落ちる。
func TestRun_SkipsIneligibleAtWriteTime(t *testing.T) {
	store := leakFixture()
	store.eligible[7] = false
	w := &fakeWriter{}
	res, err := newTestJob(t, store, w).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(w.puts) != 1 || !strings.HasSuffix(w.puts[0].key, "manifest.json") {
		t.Fatalf("書き込み = %v, want マニフェストだけ", w.keys())
	}
	if res.Students != 0 || res.Sessions != 0 {
		t.Errorf("Result = %+v", res)
	}
	if fmt.Sprint(store.eligCalls) != "[7]" {
		t.Errorf("判定の呼び出し = %v", store.eligCalls)
	}
	var m Manifest
	if err := json.Unmarshal(w.puts[0].body, &m); err != nil {
		t.Fatal(err)
	}
	if m.Files == nil || len(m.Files) != 0 || m.StudentCount != 0 {
		t.Errorf("マニフェスト = %+v, want 空のファイル一覧", m)
	}
	if store.finished[0].status != models.TrainingScorerExportSucceeded {
		t.Errorf("監査ログ = %+v", store.finished)
	}
}

// TestRun_ZeroSubjects は対象が0人でも、マニフェストと監査ログ（成功・0件）を残すことを確かめる。
func TestRun_ZeroSubjects(t *testing.T) {
	store := &fakeStore{}
	w := &fakeWriter{}
	res, err := newTestJob(t, store, w).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(w.puts) != 1 || res.Students != 0 {
		t.Errorf("書き込み = %v, Result = %+v", w.keys(), res)
	}
	want := []finishCall{{id: 42, status: models.TrainingScorerExportSucceeded}}
	if fmt.Sprint(store.finished) != fmt.Sprint(want) {
		t.Errorf("監査ログ = %+v", store.finished)
	}
}

// TestRun_SkipsUnusableSessions は、ラベルが使えない面接・学生の回答が無い面接を書かず、数えることを確かめる。
func TestRun_SkipsUnusableSessions(t *testing.T) {
	answer := []repositories.TrainingUtterance{{Role: "ai", Text: "質問"}, {Role: "user", Text: "回答"}}
	store := &fakeStore{
		subjects: []repositories.TrainingSubject{{UserID: 1, Name: "佐藤 花子"}, {UserID: 2, Name: "鈴木 一郎"}},
		eligible: map[uint]bool{1: true, 2: true},
		sessions: map[uint][]repositories.TrainingSession{
			1: {
				{SessionID: 10, ScoresJSON: validScores},
				{SessionID: 11, ScoresJSON: `{"logic":3}`}, // 項目が欠けている
				{SessionID: 12, ScoresJSON: `{"logic":6,"specificity":4,"ownership":2,"communication":5,"enthusiasm":0}`}, // 範囲外
				{SessionID: 13, ScoresJSON: `null`},
				{SessionID: 14, ScoresJSON: `not json`},
				{SessionID: 15, ScoresJSON: validScores}, // 学生の回答が無い
			},
			// 2 は面接が無い。ファイルを作らない。
		},
		utterances: map[uint][]repositories.TrainingUtterance{
			10: answer, 11: answer, 12: answer, 13: answer, 14: answer,
			15: {{Role: "ai", Text: "質問だけ"}},
		},
	}
	w := &fakeWriter{}
	res, err := newTestJob(t, store, w).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Students != 1 || res.Sessions != 1 || res.SkippedSessions != 5 {
		t.Errorf("Result = %+v, want 学生1・面接1・書かなかった5", res)
	}
	samples := decodeSamples(t, w.find(t, wantHMAC("user", 1)+".jsonl").body)
	if len(samples) != 1 || samples[0].SampleID != wantHMAC("session", 10) {
		t.Errorf("行 = %+v", samples)
	}
	for _, k := range w.keys() {
		if strings.Contains(k, wantHMAC("user", 2)) {
			t.Errorf("面接の無い学生のファイルがあります: %s", k)
		}
	}
	// 面接が無い学生は判定し直さない（書くものが無いため）。書いた学生は、書く前と書いた後の2回判定する。
	if fmt.Sprint(store.eligCalls) != "[1 1]" {
		t.Errorf("判定の呼び出し = %v", store.eligCalls)
	}
}

// TestRun_FailuresAreAudited は、途中の失敗が監査ログに「失敗」として残り、エラーが返ることを確かめる。
func TestRun_FailuresAreAudited(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*fakeStore, *fakeWriter)
		wantIs    error
		wantMsg   string
		wantCount [2]int
		wantPuts  int
	}{
		{
			name:     "学生のファイルの書き込みに失敗",
			setup:    func(_ *fakeStore, w *fakeWriter) { w.failOn = "/subjects/" },
			wantIs:   errPut,
			wantMsg:  "scorerexport: 書き込みに失敗しました",
			wantPuts: 0,
		},
		{
			name:      "マニフェストの書き込みに失敗",
			setup:     func(_ *fakeStore, w *fakeWriter) { w.failOn = "manifest.json" },
			wantIs:    errPut,
			wantMsg:   "マニフェストの書き込みに失敗しました",
			wantCount: [2]int{1, 1},
			wantPuts:  1,
		},
		{
			name:     "学生の一覧の取得に失敗",
			setup:    func(s *fakeStore, _ *fakeWriter) { s.listSubjectsErr = errDB },
			wantIs:   errDB,
			wantMsg:  "対象の学生を取得できませんでした",
			wantPuts: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, w := leakFixture(), &fakeWriter{}
			tt.setup(store, w)
			_, err := newTestJob(t, store, w).Run(context.Background())
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("err = %v, want %v", err, tt.wantIs)
			}
			if len(w.puts) != tt.wantPuts {
				t.Errorf("書き込み = %v", w.keys())
			}
			if len(store.finished) != 1 {
				t.Fatalf("監査ログ = %+v", store.finished)
			}
			f := store.finished[0]
			// 監査ログのエラーは文字列で保存するので、どの段階の失敗かを文字列で確かめる。
			if f.status != models.TrainingScorerExportFailed || !strings.Contains(f.errMsg, tt.wantMsg) ||
				f.students != tt.wantCount[0] || f.sessions != tt.wantCount[1] {
				t.Errorf("監査ログ = %+v", f)
			}
			// エラーに含まれたキーの subject は伏せる。
			if strings.Contains(f.errMsg, wantHMAC("user", 7)) {
				t.Errorf("監査ログに subject が残っています: %s", f.errMsg)
			}
		})
	}
}

// TestRun_CanceledContextIsAuditedAsFailure は、呼び出し側の ctx が取り消されて失敗した回も、
// 取り消されていない ctx で監査ログに「失敗」を書くことを確かめる。
func TestRun_CanceledContextIsAuditedAsFailure(t *testing.T) {
	store := leakFixture()
	store.termsErr = func(ctx context.Context) error { return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newTestJob(t, store, &fakeWriter{}).Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(store.finished) != 1 || store.finished[0].status != models.TrainingScorerExportFailed || store.finished[0].ctxErr != nil {
		t.Errorf("監査ログ = %+v", store.finished)
	}
}

// TestRun_NoExportWithoutAuditLog は、監査ログを作れなければ何も書き出さないことを確かめる。
func TestRun_NoExportWithoutAuditLog(t *testing.T) {
	store := leakFixture()
	store.startErr = errDB
	w := &fakeWriter{}
	_, err := newTestJob(t, store, w).Run(context.Background())
	if !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
	if len(w.puts) != 0 || len(store.finished) != 0 {
		t.Errorf("書き込み = %v, 監査ログの更新 = %+v", w.keys(), store.finished)
	}
}

// TestRun_FinishAuditErrorIsReturned は、監査ログに結果を書けなかったことを、書き出しが成功しても返すことを確かめる。
func TestRun_FinishAuditErrorIsReturned(t *testing.T) {
	store := leakFixture()
	store.finishErr = errDB
	_, err := newTestJob(t, store, &fakeWriter{}).Run(context.Background())
	if !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestConfigFromEnv(t *testing.T) {
	key32 := string(testKey)
	tests := []struct {
		name       string
		key, uri   string
		wantErr    error
		wantBucket string
		wantPrefix string
	}{
		{name: "鍵と出力先がある", key: key32, uri: "s3://b/scorer/private", wantBucket: "b", wantPrefix: "scorer/private"},
		{name: "プレフィックスの前後の「/」は落とす", key: key32, uri: " s3://b//scorer/ ", wantBucket: "b", wantPrefix: "scorer"},
		{name: "鍵がちょうど32バイト", key: key32, uri: "s3://b/p", wantBucket: "b", wantPrefix: "p"},
		{name: "鍵が31バイト", key: key32[:31], uri: "s3://b/p", wantErr: ErrHMACKeyTooShort},
		{name: "鍵が無い", key: "", uri: "s3://b/p", wantErr: ErrHMACKeyMissing},
		{name: "鍵が空白だけ", key: "   ", uri: "s3://b/p", wantErr: ErrHMACKeyMissing},
		{name: "出力先が無い", key: key32, uri: "", wantErr: ErrDestinationMissing},
		{name: "s3 の URI でない", key: key32, uri: "https://b.s3.amazonaws.com/p", wantErr: ErrDestinationInvalid},
		{name: "バケットだけ", key: key32, uri: "s3://b", wantErr: ErrDestinationInvalid},
		{name: "バケットの直下", key: key32, uri: "s3://b/", wantErr: ErrDestinationInvalid},
		{name: "バケット名が空", key: key32, uri: "s3:///p", wantErr: ErrDestinationInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{EnvHMACKey: tt.key, EnvDestination: tt.uri}
			cfg, err := ConfigFromEnv(func(k string) string { return env[k] })
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if cfg.Bucket != tt.wantBucket || cfg.Prefix != tt.wantPrefix {
				t.Errorf("cfg = %+v", cfg)
			}
		})
	}
}

// TestNewJob_RejectsInvalidConfig は、設定が不正ならジョブを作らない（動かさない）ことを確かめる。
// CLI を通さずにジョブを作る経路でも、鍵と出力先の確認を外せないようにするため。
func TestNewJob_RejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		store   Store
		writer  ObjectWriter
		wantErr error
	}{
		{name: "鍵が無い", cfg: Config{Bucket: "b", Prefix: "p"}, store: &fakeStore{}, writer: &fakeWriter{}, wantErr: ErrHMACKeyMissing},
		{name: "出力先が無い", cfg: Config{HMACKey: testKey}, store: &fakeStore{}, writer: &fakeWriter{}, wantErr: ErrDestinationMissing},
		{name: "プレフィックスが無い", cfg: Config{HMACKey: testKey, Bucket: "b"}, store: &fakeStore{}, writer: &fakeWriter{}, wantErr: ErrDestinationInvalid},
		{name: "store が無い", cfg: testConfig(), writer: &fakeWriter{}},
		{name: "writer が無い", cfg: testConfig(), store: &fakeStore{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, err := NewJob(tt.cfg, tt.store, tt.writer)
			if err == nil || j != nil {
				t.Fatalf("NewJob = %v, %v, want エラー", j, err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildTurns(t *testing.T) {
	m, err := masking.New(maskingRules(), masking.Dictionary{masking.KeyName: {"山田太郎"}})
	if err != nil {
		t.Fatal(err)
	}
	u := func(role, text string) repositories.TrainingUtterance {
		return repositories.TrainingUtterance{Role: role, Text: text}
	}
	tests := []struct {
		name string
		in   []repositories.TrainingUtterance
		want []Turn
	}{
		{name: "発話が無い", in: nil, want: nil},
		{name: "質問と回答", in: []repositories.TrainingUtterance{u("ai", "Q1"), u("user", "A1"), u("ai", "Q2"), u("user", "A2")},
			want: []Turn{{Q: "Q1", A: "A1"}, {Q: "Q2", A: "A2"}}},
		{name: "続いた AI の発話は1つの質問", in: []repositories.TrainingUtterance{u("ai", "前置き"), u("ai", "Q1"), u("user", "A1")},
			want: []Turn{{Q: "前置き\nQ1", A: "A1"}}},
		{name: "続いた学生の発話は1つの回答", in: []repositories.TrainingUtterance{u("ai", "Q1"), u("user", "A1"), u("user", "A1続き")},
			want: []Turn{{Q: "Q1", A: "A1\nA1続き"}}},
		{name: "最初の質問より前の回答は質問を空に", in: []repositories.TrainingUtterance{u("user", "こんにちは"), u("ai", "Q1"), u("user", "A1")},
			want: []Turn{{Q: "", A: "こんにちは"}, {Q: "Q1", A: "A1"}}},
		{name: "回答の無い最後の質問は入れない", in: []repositories.TrainingUtterance{u("ai", "Q1"), u("user", "A1"), u("ai", "ありがとうございました")},
			want: []Turn{{Q: "Q1", A: "A1"}}},
		{name: "空の発話とほかの役割は使わない", in: []repositories.TrainingUtterance{u("ai", "Q1"), u("user", "  "), u("system", "内部"), u("user", "A1")},
			want: []Turn{{Q: "Q1", A: "A1"}}},
		{name: "質問と回答の両方をマスキング", in: []repositories.TrainingUtterance{u("ai", "山田太郎さん"), u("user", "山田太郎です")},
			want: []Turn{{Q: "<NAME>さん", A: "<NAME>です"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTurns(tt.in, m)
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("buildTurns = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestHash は、鍵と種類（利用者・面接）で値が変わり、同じ入力なら同じ値になることを確かめる。
func TestHash(t *testing.T) {
	j := newTestJob(t, &fakeStore{}, &fakeWriter{})
	other := newTestJob(t, &fakeStore{}, &fakeWriter{})
	other.cfg.HMACKey = []byte("ffffffffffffffffffffffffffffffff")

	if got := j.hash("user", 7); got != wantHMAC("user", 7) || len(got) != 64 {
		t.Errorf("hash = %q", got)
	}
	if j.hash("user", 7) == j.hash("session", 7) {
		t.Error("利用者と面接で同じ値になっています")
	}
	if j.hash("user", 7) == other.hash("user", 7) {
		t.Error("鍵が違っても同じ値になっています")
	}
	if j.hash("user", 7) == j.hash("user", 8) {
		t.Error("ID が違っても同じ値になっています")
	}
}

func TestRedactHashes(t *testing.T) {
	h := wantHMAC("user", 7)
	got := redactHashes("denied: train-bucket/runs/x/subjects/" + h + ".jsonl")
	if got != "denied: train-bucket/runs/x/subjects/<subject>.jsonl" {
		t.Errorf("redactHashes = %q", got)
	}
	if got := redactHashes("s3 put: timeout"); got != "s3 put: timeout" {
		t.Errorf("伏せる値の無いエラーが変わった: %q", got)
	}
}

func TestNewMasker_KnownEmail(t *testing.T) {
	tests := []struct {
		name, email, text, want string
	}{
		{name: "アドレス全体", email: "taro@example.ac.jp", text: "taro@example.ac.jp", want: "<EMAIL>"},
		{name: "トップレベルドメインが欠けた書き方", email: "taro.y@example.ac.jp", text: "taro.y@example です", want: "<EMAIL>@example です"},
		{name: "メールが空", email: "", text: "taro です", want: "taro です"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newMasker(repositories.TrainingSubject{Email: tt.email}, repositories.TrainingMaskingTerms{})
			if err != nil {
				t.Fatal(err)
			}
			if got := m.Mask(tt.text); got != tt.want {
				t.Errorf("Mask = %q, want %q", got, tt.want)
			}
		})
	}
}

type maskUserStub struct {
	user *models.User
	err  error
}

func (s maskUserStub) GetUserByID(uint) (*models.User, error) { return s.user, s.err }

type maskTermsStub struct {
	calls int
	terms repositories.TrainingMaskingTerms
	err   error
}

func (s *maskTermsStub) ListTrainingMaskingTerms(context.Context) (repositories.TrainingMaskingTerms, error) {
	s.calls++
	return s.terms, s.err
}

func TestInterviewShadowMask_UsesExportDictionary(t *testing.T) {
	terms := &maskTermsStub{terms: repositories.TrainingMaskingTerms{Companies: []string{"架空製作所"}}}
	masker := NewInterviewShadowMask(maskUserStub{user: &models.User{
		ID: 7, Name: "山田太郎", Email: "taro@example.com", SchoolName: "架空高校",
	}}, terms)
	mask, err := masker.ForUser(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	got := mask("山田太郎です。架空製作所と taro@example.com")
	if strings.Contains(got, "山田") || strings.Contains(got, "架空製作所") || strings.Contains(got, "taro@example.com") {
		t.Fatalf("伏字が残った: %s", got)
	}
	if _, err := masker.ForUser(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if terms.calls != 1 {
		t.Fatalf("学校・企業名の一覧を %d 回読んだ", terms.calls)
	}
}

func TestInterviewShadowMask_MissingUser(t *testing.T) {
	masker := NewInterviewShadowMask(maskUserStub{}, &maskTermsStub{})
	if _, err := masker.ForUser(context.Background(), 1); err == nil {
		t.Fatal("学生がいないのに伏字を作った")
	}
}

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestRunFromEnv_ConfigErrorStopsBeforeDB は、鍵か出力先が無ければ DB にもつながず、出力先も作らないことを確かめる。
func TestRunFromEnv_ConfigErrorStopsBeforeDB(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr error
	}{
		{name: "鍵が無い", env: map[string]string{EnvDestination: "s3://b/p"}, wantErr: ErrHMACKeyMissing},
		{name: "出力先が無い", env: map[string]string{EnvHMACKey: string(testKey)}, wantErr: ErrDestinationMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connected, prepared := false, false
			_, _, err := RunFromEnv(context.Background(), envOf(tt.env),
				func() (Store, error) { connected = true; return &fakeStore{}, nil },
				func(context.Context, string) (ObjectStore, error) { prepared = true; return &fakeWriter{}, nil })
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if connected || prepared {
				t.Errorf("設定の誤りなのに DB（%v）か出力先（%v）を用意しました", connected, prepared)
			}
		})
	}
}

// TestRunFromEnv_Wiring は、設定のバケットで出力先を作り、DB の Store でジョブを動かして監査ログを残すことを確かめる。
func TestRunFromEnv_Wiring(t *testing.T) {
	store, w := leakFixture(), &fakeWriter{}
	var gotBucket string
	res, swept, err := RunFromEnv(context.Background(),
		envOf(map[string]string{EnvHMACKey: string(testKey), EnvDestination: "s3://train-bucket/scorer"}),
		func() (Store, error) { return store, nil },
		func(_ context.Context, bucket string) (ObjectStore, error) { gotBucket = bucket; return w, nil })
	if err != nil {
		t.Fatal(err)
	}
	if gotBucket != "train-bucket" {
		t.Errorf("出力先のバケット = %q", gotBucket)
	}
	if !strings.HasPrefix(res.Destination, "s3://train-bucket/scorer/runs/") || store.startedDest != res.Destination {
		t.Errorf("出力先 = %q（監査ログ %q）", res.Destination, store.startedDest)
	}
	if res.Students != 1 || res.Sessions != 1 {
		t.Errorf("Result = %+v", res)
	}
	if len(store.finished) != 1 || store.finished[0].status != models.TrainingScorerExportSucceeded {
		t.Errorf("監査ログ = %+v", store.finished)
	}
	// 環境変数の鍵で HMAC にしている（鍵の配線が切れると subject が変わる）。
	w.find(t, "/subjects/"+wantHMAC("user", 7)+".jsonl")
	// 書き出しの前に、書き出し済みのデータの削除を行い、監査ログに残す（消すものが無くても残す）。
	if fmt.Sprint(store.sweepStarted) != "[s3://train-bucket/scorer/runs/]" || len(store.sweepFinished) != 1 ||
		store.sweepFinished[0].status != models.TrainingScorerExportSucceeded || swept.DeletedFiles != 0 {
		t.Errorf("削除の監査ログ = %v %+v, 結果 %+v", store.sweepStarted, store.sweepFinished, swept)
	}
}

func TestRunFromEnv_SetupErrors(t *testing.T) {
	env := envOf(map[string]string{EnvHMACKey: string(testKey), EnvDestination: "s3://b/p"})
	dbErr, s3Err := errors.New("db down"), errors.New("no aws config")
	tests := []struct {
		name      string
		connect   func() (Store, error)
		newWriter func(context.Context, string) (ObjectStore, error)
		wantErr   error
	}{
		{
			name:      "DB につながらない",
			connect:   func() (Store, error) { return nil, dbErr },
			newWriter: func(context.Context, string) (ObjectStore, error) { return &fakeWriter{}, nil },
			wantErr:   dbErr,
		},
		{
			name:      "出力先を用意できない",
			connect:   func() (Store, error) { return &fakeStore{}, nil },
			newWriter: func(context.Context, string) (ObjectStore, error) { return nil, s3Err },
			wantErr:   s3Err,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := RunFromEnv(context.Background(), env, tt.connect, tt.newWriter); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestRun_MasksOwnSchoolNotInTerms は、本人が自由記述した学校名が、DB の名称の一覧に無くても置き換わることを確かめる。
func TestRun_MasksOwnSchoolNotInTerms(t *testing.T) {
	store := leakFixture()
	store.terms = repositories.TrainingMaskingTerms{}
	store.subjects[0].SchoolName = "北九州情報ビジネス専門学校"
	store.utterances[100] = []repositories.TrainingUtterance{
		{SessionID: 100, Role: "ai", Text: "学校では何を学んでいますか。"},
		{SessionID: 100, Role: "user", Text: "北九州情報ビジネス専門学校でネットワークを学んでいます。"},
	}
	w := &fakeWriter{}
	if _, err := newTestJob(t, store, w).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	samples := decodeSamples(t, w.find(t, ".jsonl").body)
	if got := samples[0].Turns[0].A; got != "<SCHOOL>でネットワークを学んでいます。" {
		t.Errorf("回答 = %q", got)
	}
}

// TestRun_DeletesWhenRevokedDuringWrite は、書く直前の判定と書き込みの間に同意を取り消した学生のファイルを、
// 書いた後の判定で見つけて消し、件数にもマニフェストにも入れないことを確かめる（#2325 のレビュー）。
func TestRun_DeletesWhenRevokedDuringWrite(t *testing.T) {
	store := leakFixture()
	store.eligSeq = map[uint][]bool{7: {true, false}}
	w := &fakeWriter{}
	res, err := newTestJob(t, store, w).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	subjectKey := "scorer/private/runs/20261117T010203Z-a1b2c3d4/subjects/" + wantHMAC("user", 7) + ".jsonl"
	if fmt.Sprint(w.deletes) != "["+subjectKey+"]" {
		t.Fatalf("消したもの = %v, want [%s]", w.deletes, subjectKey)
	}
	if res.Students != 0 || res.Sessions != 0 {
		t.Errorf("Result = %+v, want 0人・0件", res)
	}
	var m Manifest
	if err := json.Unmarshal(w.find(t, "manifest.json").body, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 0 || m.StudentCount != 0 {
		t.Errorf("マニフェスト = %+v, want ファイルなし", m)
	}
}

// TestRun_FailsWhenDeleteAfterRevokeFails は、取り消した学生のファイルを消せなければ、実行を失敗にして
// 監査ログに残すことを確かめる（取り消し後のデータが残ったまま成功扱いにしない）。
func TestRun_FailsWhenDeleteAfterRevokeFails(t *testing.T) {
	store := leakFixture()
	store.eligSeq = map[uint][]bool{7: {true, false}}
	errDel := errors.New("delete denied")
	w := &fakeWriter{deleteErr: errDel}
	_, err := newTestJob(t, store, w).Run(context.Background())
	if !errors.Is(err, errDel) {
		t.Fatalf("err = %v, want %v", err, errDel)
	}
	if len(store.finished) != 1 || store.finished[0].status != models.TrainingScorerExportFailed {
		t.Fatalf("監査ログ = %+v, want 失敗", store.finished)
	}
}

// TestRun_DeletesWhenRecheckErrors は、書いた後の判定がエラーなら、ファイルを消してから実行を失敗にすることを
// 確かめる（使ってよいか分からないデータを残さない）。
func TestRun_DeletesWhenRecheckErrors(t *testing.T) {
	store := leakFixture()
	store.eligErr = map[uint][]error{7: {nil, errDB}}
	w := &fakeWriter{}
	_, err := newTestJob(t, store, w).Run(context.Background())
	if !errors.Is(err, errDB) {
		t.Fatalf("err = %v, want %v", err, errDB)
	}
	if len(w.deletes) != 1 {
		t.Fatalf("消したもの = %v, want 1件", w.deletes)
	}
}

// TestRandomRunSuffix は、実行 ID の末尾が 8 桁の16進で、呼ぶたびに変わることを確かめる（二重起動での衝突を防ぐ）。
func TestRandomRunSuffix(t *testing.T) {
	a, b := randomRunSuffix(), randomRunSuffix()
	if len(a) != 8 || len(b) != 8 || a == b {
		t.Fatalf("randomRunSuffix = %q, %q", a, b)
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Fatalf("16進でない: %q", a)
	}
}
