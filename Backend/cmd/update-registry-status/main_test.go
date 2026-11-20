package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

// 偽の国税庁。法人番号ごとの結果を返す（外部通信なし）。
type fakeFinder struct {
	byNumber map[string]*houjinbangou.Corporation
	errs     map[string]error
}

func (f fakeFinder) FindByNumber(_ context.Context, n string) (*houjinbangou.Corporation, error) {
	if err := f.errs[n]; err != nil {
		return nil, err
	}
	return f.byNumber[n], nil
}

// 偽の DB。Apply / Restore の「現在の値が from のままの行だけ更新」を再現する。
type fakeStore struct {
	rows          map[uint]*models.Company
	gotIncludeDrf bool
	gotLimit      int
}

func (s *fakeStore) Targets(includeDrafts bool, limit int) ([]models.Company, error) {
	s.gotIncludeDrf, s.gotLimit = includeDrafts, limit
	var out []models.Company
	for id := uint(1); id <= uint(len(s.rows)); id++ {
		out = append(out, *s.rows[id])
	}
	return out, nil
}

func (s *fakeStore) Apply(rows []backupRow) (int64, error) {
	return s.update(rows, func(r backupRow) (registryState, registryState) { return r.Before, r.After })
}

func (s *fakeStore) Restore(rows []backupRow) (int64, error) {
	return s.update(rows, func(r backupRow) (registryState, registryState) { return r.After, r.Before })
}

func (s *fakeStore) update(rows []backupRow, pick func(backupRow) (registryState, registryState)) (int64, error) {
	var n int64
	for _, r := range rows {
		from, to := pick(r)
		c := s.rows[r.CompanyID]
		cur := stateOf(c)
		if cur.Kind != from.Kind || !ptrEq(cur.ClosedOn, from.ClosedOn) || !timeEq(cur.CheckedAt, from.CheckedAt) {
			continue
		}
		c.CorporateKind = to.Kind
		c.RegistryCheckedAt = to.CheckedAt
		c.RegistryClosedOn = nil
		if to.ClosedOn != nil {
			d, _ := time.Parse(dateLayout, *to.ClosedOn)
			c.RegistryClosedOn = &d
		}
		n++
	}
	return n, nil
}

func timeEq(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

var fixedNow = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }

func corp(name, kind, closeDate string) *houjinbangou.Corporation {
	return &houjinbangou.Corporation{Name: name, Kind: kind, CloseDate: closeDate}
}

func newFixture() (*fakeStore, fakeFinder) {
	st := &fakeStore{rows: map[uint]*models.Company{
		1: {ID: 1, Name: "株式会社テスト商事", CorporateNumber: "1000000000001"},
		2: {ID: 2, Name: "テスト省", CorporateNumber: "2000000000002"},
		3: {ID: 3, Name: "株式会社テスト工業", CorporateNumber: "3000000000003"},
		4: {ID: 4, Name: "株式会社テスト物産", CorporateNumber: "4000000000004"},
		5: {ID: 5, Name: "テスト市", CorporateNumber: "5000000000005"},
		6: {ID: 6, Name: "株式会社テスト通信", CorporateNumber: "6000000000006"},
	}}
	return st, fakeFinder{
		byNumber: map[string]*houjinbangou.Corporation{
			"1000000000001": corp("株式会社テスト商事", "301", ""),
			"2000000000002": corp("テスト省", "101", ""),
			"3000000000003": corp("株式会社テスト工業", "301", "2020-04-01"),
			"4000000000004": corp("株式会社まったく別の会社", "301", ""),
			"5000000000005": corp("テスト市", "201", ""),
		},
		errs: map[string]error{"6000000000006": errors.New("通信エラー")},
	}
}

func TestRun_ConfirmOnlyDoesNotWrite(t *testing.T) {
	st, f := newFixture()
	var out bytes.Buffer
	if err := run(context.Background(), options{}, st, f, &out, fixedNow); err != nil {
		t.Fatal(err)
	}
	for id, c := range st.rows {
		if c.CorporateKind != "" || c.RegistryCheckedAt != nil {
			t.Fatalf("確認のみは書かない: %d", id)
		}
	}
	s := out.String()
	for _, want := range []string{"国の機関 1 社", "地方公共団体 1 社", "登記が閉鎖 1 社", "商号の不一致 1 社", "通信などの失敗 1 社",
		"国税庁法人番号公表サイト（国税庁）", "--include-drafts", "確認のみです"} {
		if !strings.Contains(s, want) {
			t.Errorf("出力に %q が無い:\n%s", want, s)
		}
	}
}

func TestRun_ApplyStoresKindAndClosed_SkipsMismatchAndFailures(t *testing.T) {
	st, f := newFixture()
	backup := filepath.Join(t.TempDir(), "before.json")
	var out bytes.Buffer
	o := options{Apply: true, Backup: backup, IncludeDrafts: true, Limit: 10}
	if err := run(context.Background(), o, st, f, &out, fixedNow); err != nil {
		t.Fatal(err)
	}
	if !st.gotIncludeDrf || st.gotLimit != 10 {
		t.Fatal("--include-drafts と --limit が対象の取得に渡る")
	}
	if c := st.rows[1]; c.CorporateKind != "301" || c.RegistryClosedOn != nil || c.RegistryCheckedAt == nil {
		t.Fatalf("閉鎖なしは、種別と確認日時だけ保存: %+v", c)
	}
	if c := st.rows[2]; c.CorporateKind != "101" {
		t.Fatal("国の機関の種別を保存")
	}
	if c := st.rows[3]; c.RegistryClosedOn == nil || c.RegistryClosedOn.Format(dateLayout) != "2020-04-01" {
		t.Fatal("閉鎖日を保存")
	}
	for _, id := range []uint{4, 6} {
		if c := st.rows[id]; c.CorporateKind != "" || c.RegistryCheckedAt != nil {
			t.Fatalf("商号の不一致・通信の失敗は書かない: %d", id)
		}
	}
	if !strings.Contains(out.String(), "まったく別の会社") {
		t.Fatal("商号の不一致は一覧に出す")
	}
}

func TestRun_ExistingValues(t *testing.T) {
	st, f := newFixture()
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 確認済みで同じ値 → 書かない（確認日時を変えない）
	st.rows[1].CorporateKind, st.rows[1].RegistryCheckedAt = "301", &old
	// 確認済みだが、国税庁では閉鎖に変わった → 書き直す
	st.rows[3].CorporateKind, st.rows[3].RegistryCheckedAt = "301", &old
	var out bytes.Buffer
	o := options{Apply: true, Backup: filepath.Join(t.TempDir(), "b.json")}
	if err := run(context.Background(), o, st, f, &out, fixedNow); err != nil {
		t.Fatal(err)
	}
	if !st.rows[1].RegistryCheckedAt.Equal(old) {
		t.Fatal("値が同じなら確認日時を書き換えない")
	}
	if st.rows[3].RegistryClosedOn == nil || st.rows[3].RegistryCheckedAt.Equal(old) {
		t.Fatal("閉鎖に変わった企業は書き直す")
	}
	if !strings.Contains(out.String(), "変更なし 1 社") || !strings.Contains(out.String(), "値が変わった企業 1 社") {
		t.Fatalf("内訳に出る:\n%s", out.String())
	}
}

func TestBackupRestore_RoundTripAndSkipsEditedRows(t *testing.T) {
	st, f := newFixture()
	backup := filepath.Join(t.TempDir(), "before.json")
	var out bytes.Buffer
	if err := run(context.Background(), options{Apply: true, Backup: backup}, st, f, &out, fixedNow); err != nil {
		t.Fatal(err)
	}
	// 書いた後に、人が企業 2 の種別を直した → 復元は触らない
	st.rows[2].CorporateKind = "399"

	out.Reset()
	if err := run(context.Background(), options{Restore: backup}, st, f, &out, fixedNow); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{1, 3, 5} {
		c := st.rows[id]
		if c.CorporateKind != "" || c.RegistryClosedOn != nil || c.RegistryCheckedAt != nil {
			t.Fatalf("書く前（未確認）に戻る: %d %+v", id, c)
		}
	}
	if st.rows[2].CorporateKind != "399" {
		t.Fatal("書いた後に変わった値は戻さない")
	}
}

func TestWriteBackup_NeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b.json")
	if err := writeBackup(path, []backupRow{{CompanyID: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := writeBackup(path, nil); err == nil {
		t.Fatal("既存のバックアップは上書きしない")
	}
	b, _ := os.ReadFile(path) //nolint:gosec // テストの一時ファイル
	if !strings.Contains(string(b), `"company_id": 1`) {
		t.Fatal("元の内容が残る")
	}
}

func TestRun_ApplyDoesNotOverwriteExistingBackup(t *testing.T) {
	st, f := newFixture()
	backup := filepath.Join(t.TempDir(), "b.json")
	_ = os.WriteFile(backup, []byte("keep"), 0o600)
	err := run(context.Background(), options{Apply: true, Backup: backup}, st, f, &bytes.Buffer{}, fixedNow)
	if err == nil {
		t.Fatal("既存のバックアップがあれば止まる")
	}
	if st.rows[1].CorporateKind != "" {
		t.Fatal("バックアップに失敗したら DB には書かない")
	}
}

func TestCheckPreconditions(t *testing.T) {
	tests := []struct {
		name    string
		o       options
		host    string
		env     string
		appID   bool
		wantErr string
	}{
		{"確認のみ", options{}, "db.example.internal", "production", true, ""},
		{"本番への apply は拒否", options{Apply: true, Backup: "b"}, "db.example.internal", "", true, "--allow-production"},
		{"本番環境変数での apply は拒否", options{Apply: true, Backup: "b"}, "localhost", "production", true, "--allow-production"},
		{"本番への restore も拒否", options{Restore: "b"}, "db.example.internal", "", false, "--allow-production"},
		{"許可すれば通る", options{Apply: true, Backup: "b", AllowProd: true}, "db.example.internal", "", true, ""},
		{"apply に backup が無い", options{Apply: true}, "localhost", "", true, "--backup"},
		{"ID 未設定", options{}, "localhost", "", false, "HOUJIN_BANGOU_APP_ID"},
		{"restore は ID 不要", options{Restore: "b"}, "localhost", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPreconditions(tt.o, tt.host, tt.env, tt.appID)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("エラーなしのはず: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("want %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestToState_UnparsableCloseDateAndMissingKind(t *testing.T) {
	if _, err := toState(corp("x", "301", "令和2年"), fixedNow()); err == nil {
		t.Fatal("閉鎖日を解釈できなければエラー")
	}
	if _, err := toState(corp("x", "", ""), fixedNow()); err == nil {
		t.Fatal("法人種別がなければエラー")
	}
}
