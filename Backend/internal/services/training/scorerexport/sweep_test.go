package scorerexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
)

const runsPrefix = "scorer/private/runs/"

func subjectKey(run string, userID uint) string {
	return runsPrefix + run + "/subjects/" + wantHMAC("user", userID) + ".jsonl"
}

func subjectFile(userID uint) string {
	return "subjects/" + wantHMAC("user", userID) + ".jsonl"
}

func manifestBody(t *testing.T, sessions map[uint]int) []byte {
	t.Helper()
	m := Manifest{FormatVersion: FormatVersion, RunID: "r", LabelSource: LabelSource, FileSessions: map[string]int{}}
	for id, n := range sessions {
		m.Files = append(m.Files, subjectFile(id))
		m.FileSessions[subjectFile(id)] = n
		m.StudentCount++
		m.SessionCount += n
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// sweepFixture は書き出し済みの3回ぶんを置く。
//
//	runA（マニフェストあり）: 学生 1（同意あり）・学生 2（取り消し）
//	runB（マニフェストなし。書き出しの途中・途中で落ちた回）: 学生 2
//	runC（マニフェストあり）: 学生 3（退会）と、学生のファイルではないキー
func sweepFixture(t *testing.T) (*fakeStore, *fakeWriter) {
	t.Helper()
	w := &fakeWriter{stored: map[string][]byte{
		subjectKey("runA", 1):             []byte("{}\n{}\n{}\n"),
		subjectKey("runA", 2):             []byte("{}\n{}\n"),
		runsPrefix + "runA/manifest.json": manifestBody(t, map[uint]int{1: 3, 2: 2}),
		subjectKey("runB", 2):             []byte("{}\n"),
		subjectKey("runC", 3):             []byte("{}\n"),
		runsPrefix + "runC/manifest.json": manifestBody(t, map[uint]int{3: 1}),
		runsPrefix + "runC/notes.txt":     []byte("x"),
	}}
	return &fakeStore{eligibleIDs: []uint{1}}, w
}

func newTestSweeper(t *testing.T, store SweepStore, w ObjectStore) *Sweeper {
	t.Helper()
	s, err := NewSweeper(testConfig(), store, w)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return fixedNow }
	return s
}

func readManifest(t *testing.T, w *fakeWriter, run string) Manifest {
	t.Helper()
	var m Manifest
	if err := json.Unmarshal(w.stored[runsPrefix+run+"/manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestSweep_DeletesRevokedAndWithdrawn は、いま学習に使ってよい学生以外（取り消し・退会）のファイルを
// すべての回から消し、マニフェストの files・件数・file_sessions を直すことを確かめる（受け入れ条件
// 「同意をオフにした人 → 次の書き出しまたは削除の処理 → S3 から消える」）。
func TestSweep_DeletesRevokedAndWithdrawn(t *testing.T) {
	store, w := sweepFixture(t)
	res, err := newTestSweeper(t, store, w).Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{subjectKey("runA", 2), subjectKey("runB", 2), subjectKey("runC", 3)} {
		if _, ok := w.stored[k]; ok {
			t.Errorf("%s が残っています", k)
		}
	}
	for _, k := range []string{subjectKey("runA", 1), runsPrefix + "runC/notes.txt"} {
		if _, ok := w.stored[k]; !ok {
			t.Errorf("%s を消しました（消してはいけない）", k)
		}
	}
	if res.DeletedFiles != 3 || res.UpdatedManifests != 2 {
		t.Errorf("結果 = %+v, want 3ファイル・2マニフェスト", res)
	}
	a := readManifest(t, w, "runA")
	if fmt.Sprint(a.Files) != "["+subjectFile(1)+"]" || a.StudentCount != 1 || a.SessionCount != 3 ||
		fmt.Sprint(a.FileSessions) != "map["+subjectFile(1)+":3]" {
		t.Errorf("runA のマニフェスト = %+v", a)
	}
	c := readManifest(t, w, "runC")
	if len(c.Files) != 0 || c.Files == nil || c.StudentCount != 0 || c.SessionCount != 0 {
		t.Errorf("runC のマニフェスト = %+v（files は空の配列のまま）", c)
	}
	// マニフェストの無い回（runB）には、マニフェストを作らない。
	if _, ok := w.stored[runsPrefix+"runB/manifest.json"]; ok {
		t.Error("マニフェストの無い回にマニフェストを作りました")
	}
	want := []sweepFinishCall{{id: 77, status: models.TrainingScorerExportSucceeded, deletedFiles: 3}}
	if fmt.Sprint(store.sweepFinished) != fmt.Sprint(want) || fmt.Sprint(store.sweepStarted) != "[s3://train-bucket/"+runsPrefix+"]" {
		t.Errorf("監査ログ = %v %+v", store.sweepStarted, store.sweepFinished)
	}
}

// TestSweep_ExportedDataOfRevokedStudentDisappears は、書き出し（Job.Run）で書いた学生が同意を取り消すと、
// 次の Sweep でそのファイルが消え、マニフェストからも外れることを、書き出しの出力そのもので確かめる。
func TestSweep_ExportedDataOfRevokedStudentDisappears(t *testing.T) {
	store, w := leakFixture(), &fakeWriter{}
	res, err := newTestJob(t, store, w).Run(context.Background())
	if err != nil || res.Students != 1 {
		t.Fatalf("書き出し: %+v %v", res, err)
	}
	key := "scorer/private/runs/20261117T010203Z-a1b2c3d4/" + subjectFile(7)
	if _, ok := w.stored[key]; !ok {
		t.Fatalf("書き出したファイルがありません: %v", w.keys())
	}

	store.eligible[7] = false // 同意を取り消した
	swept, err := newTestSweeper(t, store, w).Sweep(context.Background())
	if err != nil || swept.DeletedFiles != 1 {
		t.Fatalf("削除: %+v %v", swept, err)
	}
	if _, ok := w.stored[key]; ok {
		t.Error("取り消した学生のファイルが残っています")
	}
	m := readManifest(t, w, "20261117T010203Z-a1b2c3d4")
	if len(m.Files) != 0 || m.StudentCount != 0 || m.SessionCount != 0 || len(m.FileSessions) != 0 {
		t.Errorf("マニフェスト = %+v", m)
	}
}

// TestSweep_ManifestFailureKeepsFiles は、マニフェストを書き直せなかった回のファイルは消さず（載っているのに
// 無いファイルを作らない）、ほかの回は続け、失敗を監査ログに subject を伏せて残すことを確かめる。
func TestSweep_ManifestFailureKeepsFiles(t *testing.T) {
	store, w := sweepFixture(t)
	w.failOn = "runA/manifest.json"
	res, err := newTestSweeper(t, store, w).Sweep(context.Background())
	if !errors.Is(err, errPut) {
		t.Fatalf("err = %v, want %v", err, errPut)
	}
	if _, ok := w.stored[subjectKey("runA", 2)]; !ok {
		t.Error("マニフェストを直せなかった回のファイルを消しました")
	}
	if _, ok := w.stored[subjectKey("runC", 3)]; ok {
		t.Error("ほかの回（runC）を続けていません")
	}
	if res.DeletedFiles != 2 {
		t.Errorf("消した数 = %d, want 2（runB と runC）", res.DeletedFiles)
	}
	f := store.sweepFinished
	if len(f) != 1 || f[0].status != models.TrainingScorerExportFailed || f[0].deletedFiles != 2 || hashPattern.MatchString(f[0].errMsg) {
		t.Errorf("監査ログ = %+v（subject は伏せる）", f)
	}
}

// TestSweep_DeleteFailureIsRetriedNextTime は、ファイルを消せなかったときはエラーにし（マニフェストは直した後）、
// 次の Sweep で消し直せることを確かめる。
func TestSweep_DeleteFailureIsRetriedNextTime(t *testing.T) {
	store, w := sweepFixture(t)
	w.deleteFailOn = "runA/subjects/"
	sw := newTestSweeper(t, store, w)
	if _, err := sw.Sweep(context.Background()); err == nil {
		t.Fatal("消せなかったのにエラーになりません")
	}
	if strings.Contains(store.sweepFinished[0].errMsg, wantHMAC("user", 2)) {
		t.Errorf("監査ログに subject が残っています: %s", store.sweepFinished[0].errMsg)
	}
	if m := readManifest(t, w, "runA"); len(m.Files) != 1 {
		t.Errorf("マニフェストは先に直す: %+v", m)
	}

	w.deleteFailOn = ""
	res, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w.stored[subjectKey("runA", 2)]; ok || res.DeletedFiles != 1 || res.UpdatedManifests != 0 {
		t.Errorf("2回目: 結果 %+v（残っている=%v）", res, ok)
	}
}

// TestSweep_ErrorsDeleteNothing は、対象の学生の一覧・キーの一覧・マニフェストの読み込みに失敗したとき、
// 何も（その回のファイルを）消さずにエラーにし、監査ログに「失敗」を残すことを確かめる。
// 一覧が取れないまま進めると、全員分を消してしまうため。
func TestSweep_ErrorsDeleteNothing(t *testing.T) {
	errList, errGet := errors.New("list denied"), errors.New("get denied")
	tests := []struct {
		name    string
		setup   func(*fakeStore, *fakeWriter)
		wantErr error
	}{
		{name: "学生の一覧", setup: func(s *fakeStore, _ *fakeWriter) { s.eligibleIDsErr = errDB }, wantErr: errDB},
		{name: "キーの一覧", setup: func(_ *fakeStore, w *fakeWriter) { w.listErr = errList }, wantErr: errList},
		{name: "マニフェストの読み込み", setup: func(_ *fakeStore, w *fakeWriter) { w.getErr = errGet }, wantErr: errGet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, w := sweepFixture(t)
			tt.setup(store, w)
			_, err := newTestSweeper(t, store, w).Sweep(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			for _, k := range []string{subjectKey("runA", 2), subjectKey("runC", 3)} {
				if _, ok := w.stored[k]; !ok {
					t.Errorf("%s を消しました", k)
				}
			}
			if len(store.sweepFinished) != 1 || store.sweepFinished[0].status != models.TrainingScorerExportFailed {
				t.Errorf("監査ログ = %+v", store.sweepFinished)
			}
		})
	}
}

// TestSweep_BrokenManifestKeepsFiles は、マニフェストが JSON として読めない回のファイルは消さないことを確かめる。
func TestSweep_BrokenManifestKeepsFiles(t *testing.T) {
	store, w := sweepFixture(t)
	w.stored[runsPrefix+"runA/manifest.json"] = []byte("{")
	if _, err := newTestSweeper(t, store, w).Sweep(context.Background()); err == nil {
		t.Fatal("壊れたマニフェストなのにエラーになりません")
	}
	if _, ok := w.stored[subjectKey("runA", 2)]; !ok {
		t.Error("マニフェストが壊れた回のファイルを消しました")
	}
}

// TestSweep_AuditStartFailureDeletesNothing は、監査ログを作れなければ何も消さないことを確かめる（記録の無い削除を作らない）。
func TestSweep_AuditStartFailureDeletesNothing(t *testing.T) {
	store, w := sweepFixture(t)
	store.sweepStartErr = errDB
	if _, err := newTestSweeper(t, store, w).Sweep(context.Background()); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
	if len(w.deletes) != 0 || len(store.sweepFinished) != 0 {
		t.Errorf("消したもの %v / 監査ログ %+v", w.deletes, store.sweepFinished)
	}
}

// TestSweep_AuditFinishUsesUncancelledContext は、取り消された ctx でも、監査ログの結果を取り消しを外して書くことを確かめる。
func TestSweep_AuditFinishUsesUncancelledContext(t *testing.T) {
	store, w := sweepFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = newTestSweeper(t, store, w).Sweep(ctx)
	if len(store.sweepFinished) != 1 || store.sweepFinished[0].ctxErr != nil {
		t.Errorf("監査ログ = %+v", store.sweepFinished)
	}
}

func TestSweep_AuditFinishFailureIsReturned(t *testing.T) {
	store, w := sweepFixture(t)
	store.sweepFinishErr = errDB
	if _, err := newTestSweeper(t, store, w).Sweep(context.Background()); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestSweep_RequiresStore(t *testing.T) {
	_, w := sweepFixture(t)
	if _, err := newTestSweeper(t, nil, w).Sweep(context.Background()); err == nil {
		t.Fatal("store が無いのに Sweep できました")
	}
	if len(w.deletes) != 0 {
		t.Errorf("消したもの %v", w.deletes)
	}
}

// TestDeleteUser_DeletesOnlyThatStudent は、パージから呼ぶ DeleteUser が、その学生のファイルだけを
// すべての回から消し、マニフェストを直すことを確かめる（受け入れ条件「退会 → パージ → S3 からも消える」）。
// DB を使わない（パージの後は利用者の行が無い）。
func TestDeleteUser_DeletesOnlyThatStudent(t *testing.T) {
	_, w := sweepFixture(t)
	n, err := newTestSweeper(t, nil, w).DeleteUser(context.Background(), 2)
	if err != nil || n != 2 {
		t.Fatalf("DeleteUser = %d, %v; want 2", n, err)
	}
	for _, k := range []string{subjectKey("runA", 2), subjectKey("runB", 2)} {
		if _, ok := w.stored[k]; ok {
			t.Errorf("%s が残っています", k)
		}
	}
	for _, k := range []string{subjectKey("runA", 1), subjectKey("runC", 3)} {
		if _, ok := w.stored[k]; !ok {
			t.Errorf("%s を消しました（ほかの学生）", k)
		}
	}
	if m := readManifest(t, w, "runA"); fmt.Sprint(m.Files) != "["+subjectFile(1)+"]" || m.SessionCount != 3 {
		t.Errorf("runA のマニフェスト = %+v", m)
	}
}

func TestDeleteUser_NothingToDelete(t *testing.T) {
	_, w := sweepFixture(t)
	n, err := newTestSweeper(t, nil, w).DeleteUser(context.Background(), 99)
	if err != nil || n != 0 || len(w.deletes) != 0 || len(w.puts) != 0 {
		t.Fatalf("DeleteUser = %d, %v / 消したもの %v / 書いたもの %v", n, err, w.deletes, w.keys())
	}
}

// TestDeleteUser_ErrorHidesSubject は、DeleteUser のエラーから subject を伏せることを確かめる（パージの監査ログに残るため）。
func TestDeleteUser_ErrorHidesSubject(t *testing.T) {
	_, w := sweepFixture(t)
	w.deleteFailOn = "/subjects/"
	_, err := newTestSweeper(t, nil, w).DeleteUser(context.Background(), 2)
	if err == nil {
		t.Fatal("消せなかったのにエラーになりません")
	}
	if hashPattern.MatchString(err.Error()) {
		t.Errorf("エラーに subject が残っています: %v", err)
	}
}

func TestNewSweeper_Validates(t *testing.T) {
	if _, err := NewSweeper(Config{}, nil, &fakeWriter{}); !errors.Is(err, ErrHMACKeyMissing) {
		t.Errorf("設定なし: %v", err)
	}
	if _, err := NewSweeper(testConfig(), nil, nil); err == nil {
		t.Error("objects なしで作れました")
	}
}

// TestNewUserDataDeleterFromEnv は、サーバーの配線用の Sweeper が、設定が無ければ作らず（nil）、
// 設定の誤りはエラーにし、正しければ設定のバケットで作ることを確かめる。
func TestNewUserDataDeleterFromEnv(t *testing.T) {
	newObjects := func(bucket *string) func(context.Context, string) (ObjectStore, error) {
		return func(_ context.Context, b string) (ObjectStore, error) { *bucket = b; return &fakeWriter{}, nil }
	}
	var bucket string
	s, err := NewUserDataDeleterFromEnv(context.Background(), envOf(map[string]string{EnvHMACKey: " "}), newObjects(&bucket))
	if s != nil || err != nil || bucket != "" {
		t.Errorf("設定なし: %v %v（出力先を用意した=%q）", s, err, bucket)
	}
	if _, err := NewUserDataDeleterFromEnv(context.Background(), envOf(map[string]string{EnvDestination: "s3://b/p"}), newObjects(&bucket)); !errors.Is(err, ErrHMACKeyMissing) {
		t.Errorf("鍵だけ無い: %v", err)
	}
	s, err = NewUserDataDeleterFromEnv(context.Background(),
		envOf(map[string]string{EnvHMACKey: string(testKey), EnvDestination: "s3://train-bucket/scorer"}), newObjects(&bucket))
	if err != nil || s == nil || bucket != "train-bucket" || s.runsPrefix() != "scorer/runs/" {
		t.Errorf("設定あり: %v %v bucket=%q", s, err, bucket)
	}
	s3Err := errors.New("no aws config")
	if _, err := NewUserDataDeleterFromEnv(context.Background(),
		envOf(map[string]string{EnvHMACKey: string(testKey), EnvDestination: "s3://b/p"}),
		func(context.Context, string) (ObjectStore, error) { return nil, s3Err }); !errors.Is(err, s3Err) {
		t.Errorf("出力先を用意できない: %v", err)
	}
}

// TestSweepFromEnv は、-sweep-only の本体が、設定の誤りでは DB にも出力先にも触れず、
// 正しければ削除だけを行う（書き出さない）ことを確かめる。
func TestSweepFromEnv(t *testing.T) {
	connected := false
	_, err := SweepFromEnv(context.Background(), envOf(map[string]string{EnvDestination: "s3://b/p"}),
		func() (Store, error) { connected = true; return &fakeStore{}, nil },
		func(context.Context, string) (ObjectStore, error) { return &fakeWriter{}, nil })
	if !errors.Is(err, ErrHMACKeyMissing) || connected {
		t.Fatalf("設定の誤り: %v（DB に接続した=%v）", err, connected)
	}

	store, w := sweepFixture(t)
	res, err := SweepFromEnv(context.Background(),
		envOf(map[string]string{EnvHMACKey: string(testKey), EnvDestination: "s3://train-bucket/scorer/private"}),
		func() (Store, error) { return store, nil },
		func(context.Context, string) (ObjectStore, error) { return w, nil })
	if err != nil || res.DeletedFiles != 3 {
		t.Fatalf("SweepFromEnv = %+v, %v", res, err)
	}
	if store.startedDest != "" {
		t.Errorf("-sweep-only なのに書き出しました（%s）", store.startedDest)
	}
}

// TestRunFromEnv_SweepFailureStopsExport は、書き出しの前の削除に失敗したら書き出さないことを確かめる
// （取り消した学生のデータが残っている状態を、成功の書き出しで隠さない）。
func TestRunFromEnv_SweepFailureStopsExport(t *testing.T) {
	store, w := leakFixture(), &fakeWriter{listErr: errors.New("list denied")}
	_, _, err := RunFromEnv(context.Background(),
		envOf(map[string]string{EnvHMACKey: string(testKey), EnvDestination: "s3://train-bucket/scorer"}),
		func() (Store, error) { return store, nil },
		func(context.Context, string) (ObjectStore, error) { return w, nil })
	if err == nil {
		t.Fatal("削除に失敗したのにエラーになりません")
	}
	if store.startedDest != "" || len(w.puts) != 0 {
		t.Errorf("削除に失敗したのに書き出しました（監査ログ %q、書いたもの %v）", store.startedDest, w.keys())
	}
}
