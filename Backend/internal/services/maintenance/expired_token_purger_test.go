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
	return f.results[i].n, f.results[i].err
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

func TestExpiredTokenPurger_Run(t *testing.T) {
	errDB := errors.New("db down")

	cases := []struct {
		name    string
		pending []fakeResult
		refresh []fakeResult
		// 期待する結果（仮登録・リフレッシュトークンの順）
		wantDeleted      [2]int64
		wantLimitReached [2]bool
		wantErr          [2]error
		wantCalls        [2]int
		// 期待するログ（level と target の組。出ない場合は空）
		wantLogs [][2]string
	}{
		{
			name:        "期限切れが無いときは何も消さずログも出さない",
			wantDeleted: [2]int64{0, 0},
			wantCalls:   [2]int{1, 1},
			wantLogs:    nil,
		},
		{
			name:        "1バッチに収まる件数を消して件数をログに出す",
			pending:     []fakeResult{{n: 3}},
			refresh:     []fakeResult{{n: 5}},
			wantDeleted: [2]int64{3, 5},
			wantCalls:   [2]int{1, 1},
			wantLogs:    [][2]string{{"INFO", "pending_registrations"}, {"INFO", "user_refresh_tokens"}},
		},
		{
			name:        "バッチが上限ちょうどなら次のバッチを流し、上限未満で止める",
			pending:     []fakeResult{{n: purgeBatchSize}, {n: purgeBatchSize}, {n: 10}},
			wantDeleted: [2]int64{2*purgeBatchSize + 10, 0},
			wantCalls:   [2]int{3, 1},
			wantLogs:    [][2]string{{"INFO", "pending_registrations"}},
		},
		{
			name:        "ちょうどバッチサイズの件数なら、次のバッチが0件で止める",
			refresh:     []fakeResult{{n: purgeBatchSize}},
			wantDeleted: [2]int64{0, purgeBatchSize},
			wantCalls:   [2]int{1, 2},
			wantLogs:    [][2]string{{"INFO", "user_refresh_tokens"}},
		},
		{
			name:             "1回の上限まで消したら残りを翌日に回し警告を出す",
			pending:          repeat(fakeResult{n: purgeBatchSize}, purgeMaxBatches+5),
			wantDeleted:      [2]int64{purgeBatchSize * purgeMaxBatches, 0},
			wantLimitReached: [2]bool{true, false},
			wantCalls:        [2]int{purgeMaxBatches, 1},
			wantLogs:         [][2]string{{"WARN", "pending_registrations"}},
		},
		{
			name:        "仮登録の削除が失敗してもリフレッシュトークンの削除は続ける",
			pending:     []fakeResult{{n: purgeBatchSize}, {err: errDB}},
			refresh:     []fakeResult{{n: 2}},
			wantDeleted: [2]int64{purgeBatchSize, 2},
			wantErr:     [2]error{errDB, nil},
			// 失敗したら同じ対象の次のバッチは流さない
			wantCalls: [2]int{2, 1},
			wantLogs:  [][2]string{{"ERROR", "pending_registrations"}, {"INFO", "user_refresh_tokens"}},
		},
		{
			name:        "リフレッシュトークンの削除が失敗したらログに残す",
			pending:     []fakeResult{{n: 1}},
			refresh:     []fakeResult{{err: errDB}},
			wantDeleted: [2]int64{1, 0},
			wantErr:     [2]error{nil, errDB},
			wantCalls:   [2]int{1, 1},
			wantLogs:    [][2]string{{"INFO", "pending_registrations"}, {"ERROR", "user_refresh_tokens"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pending := &fakeDeleter{results: tc.pending}
			refresh := &fakeDeleter{results: tc.refresh}
			logger, buf := newTestLogger()

			results := NewExpiredTokenPurger(pending, refresh, logger).Run(testNow)

			if len(results) != 2 {
				t.Fatalf("結果の件数 = %d, want 2", len(results))
			}
			wantTargets := [2]string{"pending_registrations", "user_refresh_tokens"}
			deleters := [2]*fakeDeleter{pending, refresh}
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
// 仮登録は実行時刻ちょうどまでを、リフレッシュトークンは猶予を引いた時刻より前を消す。
func TestExpiredTokenPurger_Cutoff(t *testing.T) {
	pending := &fakeDeleter{}
	refresh := &fakeDeleter{}
	logger, _ := newTestLogger()

	NewExpiredTokenPurger(pending, refresh, logger).Run(testNow)

	if len(pending.cutoffs) != 1 || !pending.cutoffs[0].Equal(testNow) {
		t.Errorf("仮登録の cutoff = %v, want [%v]", pending.cutoffs, testNow)
	}
	wantRefresh := testNow.Add(-7 * 24 * time.Hour)
	if len(refresh.cutoffs) != 1 || !refresh.cutoffs[0].Equal(wantRefresh) {
		t.Errorf("リフレッシュトークンの cutoff = %v, want [%v]", refresh.cutoffs, wantRefresh)
	}
	for _, f := range []*fakeDeleter{pending, refresh} {
		if !slices.Equal(f.limits, []int{1000}) {
			t.Errorf("limit = %v, want [1000]", f.limits)
		}
	}
}

// TestExpiredTokenPurger_LogFields は、ログに件数・対象・エラー以外の項目が出ないことを固定する。
// 仮登録にはメールアドレスが入っているので、行の中身をログに足す変更をここで止める。
func TestExpiredTokenPurger_LogFields(t *testing.T) {
	pending := &fakeDeleter{results: []fakeResult{{n: 1}}}
	refresh := &fakeDeleter{results: []fakeResult{{err: errors.New("db down")}}}
	logger, buf := newTestLogger()

	NewExpiredTokenPurger(pending, refresh, logger).Run(testNow)

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
