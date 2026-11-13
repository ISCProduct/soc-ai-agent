package maintenance

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeDeleter は呼ばれた引数を記録し、用意した結果を順に返す。
// 用意した結果を使い切った後は 0 件を返す。
type fakeDeleter struct {
	results []fakeResult
	cutoffs []time.Time
	limits  []int
}

type fakeResult struct {
	n   int64
	err error
}

func (f *fakeDeleter) DeleteExpired(cutoff time.Time, limit int) (int64, error) {
	f.cutoffs = append(f.cutoffs, cutoff)
	f.limits = append(f.limits, limit)
	i := len(f.cutoffs) - 1
	if i >= len(f.results) {
		return 0, nil
	}
	// 本物と同じく、limit を超えて消さない（上限の後の確認は limit=1）
	return min(f.results[i].n, int64(limit)), f.results[i].err
}

func repeat(r fakeResult, n int) []fakeResult {
	out := make([]fakeResult, n)
	for i := range out {
		out[i] = r
	}
	return out
}

type logRecord map[string]any

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func parseLogs(t *testing.T, buf *bytes.Buffer) []logRecord {
	t.Helper()
	var records []logRecord
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var r logRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("ログを JSON として読めない: %v (%q)", err, line)
		}
		records = append(records, r)
	}
	return records
}

var testNow = time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)

// newTestPurger は待ちを記録だけするようにした ExpiredTokenPurger を返す。
func newTestPurger(pending, refresh, company ExpiredDeleter, logger *slog.Logger) (*ExpiredTokenPurger, *[]time.Duration) {
	p := NewExpiredTokenPurger(pending, refresh, company, logger)
	var sleeps []time.Duration
	p.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	return p, &sleeps
}

var wantTargets = [3]string{"pending_registrations", "user_refresh_tokens", "company_user_refresh_tokens"}

func TestExpiredTokenPurger_Run(t *testing.T) {
	errDB := errors.New("db down")

	cases := []struct {
		name    string
		pending []fakeResult
		refresh []fakeResult
		company []fakeResult
		// 期待する結果（仮登録・学生のリフレッシュトークン・企業ユーザーのリフレッシュトークンの順）
		wantDeleted      [3]int64
		wantLimitReached [3]bool
		wantErr          [3]error
		wantCalls        [3]int
		// 期待するログ（level と target の組。出ない場合は空）
		wantLogs [][2]string
	}{
		{
			name:      "期限切れが無いときは何も消さずログも出さない",
			wantCalls: [3]int{1, 1, 1},
			wantLogs:  nil,
		},
		{
			name:        "1バッチに収まる件数を消して件数をログに出す",
			pending:     []fakeResult{{n: 3}},
			refresh:     []fakeResult{{n: 5}},
			company:     []fakeResult{{n: 7}},
			wantDeleted: [3]int64{3, 5, 7},
			wantCalls:   [3]int{1, 1, 1},
			wantLogs:    [][2]string{{"INFO", "pending_registrations"}, {"INFO", "user_refresh_tokens"}, {"INFO", "company_user_refresh_tokens"}},
		},
		{
			name:        "バッチが上限ちょうどなら次のバッチを流し、上限未満で止める",
			pending:     []fakeResult{{n: purgeBatchSize}, {n: purgeBatchSize}, {n: 10}},
			wantDeleted: [3]int64{2*purgeBatchSize + 10, 0, 0},
			wantCalls:   [3]int{3, 1, 1},
			wantLogs:    [][2]string{{"INFO", "pending_registrations"}},
		},
		{
			name:        "ちょうどバッチサイズの件数なら、次のバッチが0件で止める",
			refresh:     []fakeResult{{n: purgeBatchSize}},
			wantDeleted: [3]int64{0, purgeBatchSize, 0},
			wantCalls:   [3]int{1, 2, 1},
			wantLogs:    [][2]string{{"INFO", "user_refresh_tokens"}},
		},
		{
			// 最後に1件だけ消してみて、消えたら残りがあるとみなす
			name:             "1回の上限まで消してまだ残っていたら、残りを翌日に回し警告を出す",
			pending:          repeat(fakeResult{n: purgeBatchSize}, purgeMaxBatches+5),
			wantDeleted:      [3]int64{purgeBatchSize*purgeMaxBatches + 1, 0, 0},
			wantLimitReached: [3]bool{true, false, false},
			wantCalls:        [3]int{purgeMaxBatches + 1, 1, 1},
			wantLogs:         [][2]string{{"WARN", "pending_registrations"}},
		},
		{
			// #2179 のレビュー：最後のバッチがちょうど上限で残りが0件なら、消し切っているので警告しない
			name:        "1回の上限ちょうどで消し切ったら、警告を出さない",
			company:     repeat(fakeResult{n: purgeBatchSize}, purgeMaxBatches),
			wantDeleted: [3]int64{0, 0, purgeBatchSize * purgeMaxBatches},
			wantCalls:   [3]int{1, 1, purgeMaxBatches + 1},
			wantLogs:    [][2]string{{"INFO", "company_user_refresh_tokens"}},
		},
		{
			name:        "上限の後の確認で失敗したら、エラーとして残す",
			refresh:     append(repeat(fakeResult{n: purgeBatchSize}, purgeMaxBatches), fakeResult{err: errDB}),
			wantDeleted: [3]int64{0, purgeBatchSize * purgeMaxBatches, 0},
			wantErr:     [3]error{nil, errDB, nil},
			wantCalls:   [3]int{1, purgeMaxBatches + 1, 1},
			wantLogs:    [][2]string{{"ERROR", "user_refresh_tokens"}},
		},
		{
			name:        "仮登録の削除が失敗しても、残りの対象の削除は続ける",
			pending:     []fakeResult{{n: purgeBatchSize}, {err: errDB}},
			refresh:     []fakeResult{{n: 2}},
			company:     []fakeResult{{n: 4}},
			wantDeleted: [3]int64{purgeBatchSize, 2, 4},
			wantErr:     [3]error{errDB, nil, nil},
			// 失敗したら同じ対象の次のバッチは流さない
			wantCalls: [3]int{2, 1, 1},
			wantLogs:  [][2]string{{"ERROR", "pending_registrations"}, {"INFO", "user_refresh_tokens"}, {"INFO", "company_user_refresh_tokens"}},
		},
		{
			name:        "企業ユーザーのリフレッシュトークンの削除が失敗したらログに残す",
			pending:     []fakeResult{{n: 1}},
			company:     []fakeResult{{err: errDB}},
			wantDeleted: [3]int64{1, 0, 0},
			wantErr:     [3]error{nil, nil, errDB},
			wantCalls:   [3]int{1, 1, 1},
			wantLogs:    [][2]string{{"INFO", "pending_registrations"}, {"ERROR", "company_user_refresh_tokens"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deleters := [3]*fakeDeleter{{results: tc.pending}, {results: tc.refresh}, {results: tc.company}}
			logger, buf := newTestLogger()
			p, _ := newTestPurger(deleters[0], deleters[1], deleters[2], logger)

			results := p.Run(testNow)

			if len(results) != 3 {
				t.Fatalf("結果の件数 = %d, want 3", len(results))
			}
			for i, r := range results {
				if r.Target != wantTargets[i] {
					t.Errorf("results[%d].Target = %q, want %q", i, r.Target, wantTargets[i])
				}
				if r.Deleted != tc.wantDeleted[i] {
					t.Errorf("%s: Deleted = %d, want %d", r.Target, r.Deleted, tc.wantDeleted[i])
				}
				if r.LimitReached != tc.wantLimitReached[i] {
					t.Errorf("%s: LimitReached = %v, want %v", r.Target, r.LimitReached, tc.wantLimitReached[i])
				}
				// errors.Is(err, nil) は err == nil のときだけ true なので、成功の場合もこれで確かめられる
				if !errors.Is(r.Err, tc.wantErr[i]) {
					t.Errorf("%s: Err = %v, want %v", r.Target, r.Err, tc.wantErr[i])
				}
				if got := len(deleters[i].cutoffs); got != tc.wantCalls[i] {
					t.Errorf("%s: DeleteExpired の呼び出し回数 = %d, want %d", r.Target, got, tc.wantCalls[i])
				}
			}

			records := parseLogs(t, buf)
			var gotLogs [][2]string
			for _, rec := range records {
				level, _ := rec["level"].(string)
				target, _ := rec["target"].(string)
				gotLogs = append(gotLogs, [2]string{level, target})
			}
			if !slices.Equal(gotLogs, tc.wantLogs) {
				t.Errorf("ログ = %v, want %v", gotLogs, tc.wantLogs)
			}
		})
	}
}

// TestExpiredTokenPurger_Cutoff は、各対象に渡す時刻と1バッチの件数を固定する。
// 仮登録は実行時刻ちょうどまでを、リフレッシュトークン（学生・企業ユーザー）は猶予を引いた時刻より前を消す。
func TestExpiredTokenPurger_Cutoff(t *testing.T) {
	pending, refresh, company := &fakeDeleter{}, &fakeDeleter{}, &fakeDeleter{}
	logger, _ := newTestLogger()
	p, _ := newTestPurger(pending, refresh, company, logger)

	p.Run(testNow)

	if len(pending.cutoffs) != 1 || !pending.cutoffs[0].Equal(testNow) {
		t.Errorf("仮登録の cutoff = %v, want [%v]", pending.cutoffs, testNow)
	}
	wantRefresh := testNow.Add(-7 * 24 * time.Hour)
	for name, f := range map[string]*fakeDeleter{"学生": refresh, "企業ユーザー": company} {
		if len(f.cutoffs) != 1 || !f.cutoffs[0].Equal(wantRefresh) {
			t.Errorf("%sのリフレッシュトークンの cutoff = %v, want [%v]", name, f.cutoffs, wantRefresh)
		}
	}
	for _, f := range []*fakeDeleter{pending, refresh, company} {
		if !slices.Equal(f.limits, []int{1000}) {
			t.Errorf("limit = %v, want [1000]", f.limits)
		}
	}
}

// TestExpiredTokenPurger_ProbeAfterLimit は、上限の後の確認では1件だけ消すことを固定する。
func TestExpiredTokenPurger_ProbeAfterLimit(t *testing.T) {
	pending := &fakeDeleter{results: repeat(fakeResult{n: purgeBatchSize}, purgeMaxBatches)}
	logger, _ := newTestLogger()
	p, _ := newTestPurger(pending, &fakeDeleter{}, &fakeDeleter{}, logger)

	p.Run(testNow)

	if got := pending.limits[len(pending.limits)-1]; got != 1 {
		t.Errorf("確認の limit = %d, want 1", got)
	}
}

// TestExpiredTokenPurger_PauseBetweenBatches は、バッチの間にだけ待ちを入れることを固定する（#2179 のレビュー）。
// 1バッチで終わる対象では待たない。上限の後の確認（1件だけ消す）の前にも待つ。
func TestExpiredTokenPurger_PauseBetweenBatches(t *testing.T) {
	cases := []struct {
		name      string
		results   []fakeResult
		wantSleep int
	}{
		{"1バッチで終われば待たない", []fakeResult{{n: 3}}, 0},
		{"3バッチなら間の2回だけ待つ", []fakeResult{{n: purgeBatchSize}, {n: purgeBatchSize}, {n: 1}}, 2},
		{"上限まで流したら、確認の前も待つ", repeat(fakeResult{n: purgeBatchSize}, purgeMaxBatches), purgeMaxBatches},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := newTestLogger()
			p, sleeps := newTestPurger(&fakeDeleter{results: tc.results}, &fakeDeleter{}, &fakeDeleter{}, logger)
			p.Run(testNow)
			if len(*sleeps) != tc.wantSleep {
				t.Fatalf("待ちの回数 = %d, want %d", len(*sleeps), tc.wantSleep)
			}
			for _, d := range *sleeps {
				if d != 100*time.Millisecond {
					t.Errorf("待ち = %v, want 100ms", d)
				}
			}
		})
	}
}

// TestExpiredTokenPurger_LogFields は、ログに件数・対象・エラー以外の項目が出ないことを固定する。
// 仮登録にはメールアドレスが入っているので、行の中身をログに足す変更をここで止める。
func TestExpiredTokenPurger_LogFields(t *testing.T) {
	pending := &fakeDeleter{results: []fakeResult{{n: 1}}}
	refresh := &fakeDeleter{results: []fakeResult{{err: errors.New("db down")}}}
	logger, buf := newTestLogger()
	p, _ := newTestPurger(pending, refresh, &fakeDeleter{}, logger)

	p.Run(testNow)

	allowed := []string{"time", "level", "msg", "target", "deleted", "error"}
	records := parseLogs(t, buf)
	if len(records) != 2 {
		t.Fatalf("ログの件数 = %d, want 2", len(records))
	}
	for _, rec := range records {
		for k := range rec {
			if !slices.Contains(allowed, k) {
				t.Errorf("想定外のログ項目 %q（%v）", k, rec)
			}
		}
		if got, ok := rec["deleted"].(float64); !ok {
			t.Errorf("deleted が無い: %v", rec)
		} else if rec["level"] == "INFO" && got != 1 {
			t.Errorf("deleted = %v, want 1", got)
		}
	}
}
