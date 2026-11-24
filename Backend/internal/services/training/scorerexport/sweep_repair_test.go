package scorerexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
)

// #2336 のレビュー: マニフェストの書き直しは排他していないので、削除どうしが同時に走ると
// 「載っているのに無いファイル」が残りうる。消したファイルはキーの一覧に出ないので、照合で直す。

// TestSweep_RepairsLostManifestUpdate は、削除どうしが重なって、先に外したファイルを後の書き直しが戻した
// （ファイルは消えているのにマニフェストに載っている）状態を、次の Sweep が直すことを確かめる。
func TestSweep_RepairsLostManifestUpdate(t *testing.T) {
	store, w := sweepFixture(t)
	stale := w.stored[runsPrefix+"runA/manifest.json"] // 学生 1・2 が載った、削除の前のマニフェスト
	sw := newTestSweeper(t, store, w)
	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 同時に走ったもう一方の削除が、古いマニフェストを読んで書き戻した（lost update）。
	w.stored[runsPrefix+"runA/manifest.json"] = stale

	res, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.RepairedEntries != 1 || res.UpdatedManifests != 1 || res.DeletedFiles != 0 {
		t.Errorf("結果 = %+v, want 照合で1件外す・1マニフェスト・0ファイル", res)
	}
	a := readManifest(t, w, "runA")
	if fmt.Sprint(a.Files) != "["+subjectFile(1)+"]" || a.StudentCount != 1 || a.SessionCount != 3 ||
		fmt.Sprint(a.FileSessions) != "map["+subjectFile(1)+":3]" {
		t.Errorf("runA のマニフェスト = %+v", a)
	}
	if _, ok := w.stored[subjectKey("runA", 1)]; !ok {
		t.Error("同意している学生のファイルを消しました")
	}
}

// TestSweep_RepairsRunFinishedDuringSweep は、書き出しの途中（マニフェストを書く前）の回で Sweep がファイルを消し、
// その後に書き出しが消えたファイルをマニフェストに載せた場合も、次の Sweep が直すことを確かめる。
func TestSweep_RepairsRunFinishedDuringSweep(t *testing.T) {
	store, w := sweepFixture(t)
	sw := newTestSweeper(t, store, w)
	if _, err := sw.Sweep(context.Background()); err != nil { // runB の学生 2 を消す（マニフェストはまだ無い）
		t.Fatal(err)
	}
	// 書き出しが終わり、学生 2（もう消えた）と学生 1 を載せたマニフェストを書いた。
	w.stored[subjectKey("runB", 1)] = []byte("{}\n")
	w.stored[runsPrefix+"runB/manifest.json"] = manifestBody(t, map[uint]int{1: 1, 2: 1})

	res, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b := readManifest(t, w, "runB")
	if res.RepairedEntries != 1 || fmt.Sprint(b.Files) != "["+subjectFile(1)+"]" || b.StudentCount != 1 || b.SessionCount != 1 {
		t.Errorf("結果 = %+v / runB のマニフェスト = %+v", res, b)
	}
}

// TestSweep_ManifestWrittenAfterListingIsFixed は、一覧の時点ではマニフェストが無かった回（書き出しの途中）でも、
// 消す前にマニフェストを読みに行き、一覧の後に書かれたマニフェストからも消すファイルを外すことを確かめる。
func TestSweep_ManifestWrittenAfterListingIsFixed(t *testing.T) {
	store, w := sweepFixture(t)
	w.afterList = func(prefix string) {
		if prefix == runsPrefix {
			w.stored[runsPrefix+"runB/manifest.json"] = manifestBody(t, map[uint]int{2: 1}) // 書き出しが終わった
		}
	}
	res, err := newTestSweeper(t, store, w).Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b := readManifest(t, w, "runB"); len(b.Files) != 0 || b.StudentCount != 0 || b.SessionCount != 0 {
		t.Errorf("runB のマニフェスト = %+v（消したファイルが載っている）", b)
	}
	if res.UpdatedManifests != 3 || res.RepairedEntries != 0 {
		t.Errorf("結果 = %+v, want 3マニフェスト（runA・runB・runC）", res)
	}
}

// TestSweep_KeepsFileWrittenAfterListing は、最初のキーの一覧の後に書かれた（一覧には無いが実在する）ファイルを、
// 照合でマニフェストから外さないことを確かめる。無いものがあれば、その回を一覧し直して確かめる。
func TestSweep_KeepsFileWrittenAfterListing(t *testing.T) {
	store, w := sweepFixture(t)
	store.eligibleIDs = []uint{1, 5}
	w.afterList = func(prefix string) {
		if prefix != runsPrefix {
			return
		}
		w.stored[subjectKey("runA", 5)] = []byte("{}\n")
		w.stored[runsPrefix+"runA/manifest.json"] = manifestBody(t, map[uint]int{1: 3, 2: 2, 5: 1})
	}
	res, err := newTestSweeper(t, store, w).Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.RepairedEntries != 0 {
		t.Errorf("照合で外した数 = %d, want 0（実在するファイル）", res.RepairedEntries)
	}
	a := readManifest(t, w, "runA")
	if !slices.Contains(a.Files, subjectFile(5)) || slices.Contains(a.Files, subjectFile(2)) || a.StudentCount != 2 || a.SessionCount != 4 {
		t.Errorf("runA のマニフェスト = %+v", a)
	}
	if !slices.Contains(w.lists, runsPrefix+"runA/") {
		t.Errorf("一覧し直していません: %v", w.lists)
	}
}

// TestSweep_RelistFailureKeepsFiles は、照合のための一覧し直しに失敗したら、その回のファイルを消さずに失敗にすることを確かめる。
func TestSweep_RelistFailureKeepsFiles(t *testing.T) {
	store, w := sweepFixture(t)
	delete(w.stored, subjectKey("runA", 1)) // マニフェストに載っているのに無い
	errList := errors.New("list denied")
	w.afterList = func(prefix string) {
		if prefix == runsPrefix {
			w.listErr = errList
		}
	}
	if _, err := newTestSweeper(t, store, w).Sweep(context.Background()); !errors.Is(err, errList) {
		t.Fatalf("err = %v, want %v", err, errList)
	}
	if _, ok := w.stored[subjectKey("runA", 2)]; !ok {
		t.Error("照合できなかった回のファイルを消しました")
	}
	if a := readManifest(t, w, "runA"); len(a.Files) != 2 {
		t.Errorf("照合できなかった回のマニフェストを書き直しました: %+v", a)
	}
}

// TestSweep_OldManifestWithoutFileSessions は、file_sessions の無い（この仕組みより前の）マニフェストでは、
// ファイルと student_count は直し、session_count は直さずに残し（引く数が分からない）、結果で知らせることを確かめる。
func TestSweep_OldManifestWithoutFileSessions(t *testing.T) {
	store, w := sweepFixture(t)
	var raw map[string]any
	if err := json.Unmarshal(w.stored[runsPrefix+"runA/manifest.json"], &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "file_sessions")
	old, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	w.stored[runsPrefix+"runA/manifest.json"] = old

	res, err := newTestSweeper(t, store, w).Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.UncountedManifests != 1 {
		t.Errorf("件数を直せなかったマニフェスト = %d, want 1", res.UncountedManifests)
	}
	a := readManifest(t, w, "runA")
	if fmt.Sprint(a.Files) != "["+subjectFile(1)+"]" || a.StudentCount != 1 || a.SessionCount != 5 || a.FileSessions != nil {
		t.Errorf("runA のマニフェスト = %+v（session_count は 5 のまま、file_sessions は作らない）", a)
	}
	if c := readManifest(t, w, "runC"); c.SessionCount != 0 {
		t.Errorf("file_sessions のある回は直す: %+v", c)
	}
}
