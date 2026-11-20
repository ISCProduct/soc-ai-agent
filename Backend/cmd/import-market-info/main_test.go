package main

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, b := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write(b)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func cp932(t *testing.T, s string) []byte {
	t.Helper()
	b, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const edinetCSV = "ダウンロード実行日,2026年11月20日,件数,5件\n" +
	"ＥＤＩＮＥＴコード,提出者種別,上場区分,連結の有無,資本金,決算日,提出者名,提出者名（英字）,提出者名（ヨミ）,所在地,提出者業種,証券コード,提出者法人番号\n" +
	"E00001,内国法人,上場,有,100,3月31日,テスト甲,,,東京,電気機器,11110,1000000000001\n" +
	"E00002,内国法人,上場,有,100,3月31日,テスト乙,,,東京,電気機器,22220,2000000000002\n" +
	"E00003,内国法人,非上場,無,100,3月31日,テスト丙,,,東京,サービス業,,3000000000003\n" +
	"E00004,内国法人,,無,100,3月31日,テスト丁,,,東京,サービス業,,4000000000004\n"

func TestParseEDINET(t *testing.T) {
	m, err := parseEDINETZip(makeZip(t, map[string][]byte{"EdinetcodeDlInfo.csv": cp932(t, edinetCSV)}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 4 {
		t.Fatalf("len=%d", len(m))
	}
	if e := m["1000000000001"]; e.Listing != "上場" || e.SecCode != "11110" {
		t.Fatalf("%+v", e)
	}
	if m["3000000000003"].Listing != "非上場" || m["4000000000004"].Listing != "" {
		t.Fatalf("上場区分: %+v", m)
	}
	if _, err := parseEDINETCSV(cp932(t, "メタ\n列,不足\n")); err == nil {
		t.Fatal("必要な列が無いのにエラーにならない")
	}
}

func sheetXML() string {
	return `<?xml version="1.0"?><worksheet><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c><c r="D1" t="s"><v>3</v></c></row>
<row r="2"><c r="A2"><v>20261120</v></c><c r="B2"><v>1111</v></c><c r="C2" t="s"><v>4</v></c><c r="D2" t="s"><v>5</v></c></row>
<row r="3"><c r="A3"><v>20261120</v></c><c r="B3" t="s"><v>6</v></c><c r="C3" t="s"><v>4</v></c><c r="D3" t="s"><v>7</v></c></row>
<row r="4"><c r="B4"><v>3333</v></c><c r="D4" t="s"><v>8</v></c></row>
</sheetData></worksheet>`
}

func TestParseJPXXLSX(t *testing.T) {
	sst := `<?xml version="1.0"?><sst>` +
		`<si><t>日付</t></si><si><t>コード</t></si><si><t>銘柄名</t></si><si><t>市場・商品区分</t></si>` +
		`<si><t>テスト</t></si><si><t>プライム（内国株式）</t></si><si><t>130A</t></si>` +
		`<si><r><t>グロース</t></r><r><t>（内国株式）</t></r></si><si><t>ETF・ETN</t></si></sst>`
	x := makeZip(t, map[string][]byte{"xl/sharedStrings.xml": []byte(sst), "xl/worksheets/sheet1.xml": []byte(sheetXML())})
	m, err := parseJPXXLSX(x)
	if err != nil {
		t.Fatal(err)
	}
	if m["1111"] != "プライム（内国株式）" || m["130A"] != "グロース（内国株式）" || m["3333"] != "ETF・ETN" {
		t.Fatalf("%v", m)
	}
}

func TestDecide(t *testing.T) {
	edinet := map[string]edinetEntry{
		"1000000000001": {Listing: "上場", SecCode: "11110"},
		"2000000000002": {Listing: "上場", SecCode: "22220"},
		"3000000000003": {Listing: "非上場"},
		"4000000000004": {Listing: ""},
	}
	jpx := map[string]string{"1111": "スタンダード（内国株式）", "5555": "ETF・ETN"}
	listedStd := marketValue{MarketType: "standard", IsListed: true, StockCode: "1111"}
	tests := []struct {
		name     string
		c        companyRef
		wantKind string
		want     *marketValue
	}{
		{"上場+東証結合", companyRef{CorporateNumber: "1000000000001"}, kindNew, &listedStd},
		{"上場+東証なし", companyRef{CorporateNumber: "2000000000002"}, kindNew, &marketValue{IsListed: true, StockCode: "2222"}},
		{"非上場", companyRef{CorporateNumber: "3000000000003"}, kindNew, &marketValue{MarketType: "unlisted"}},
		{"上場区分が空", companyRef{CorporateNumber: "4000000000004"}, kindUnchecked, nil},
		{"EDINETに無い", companyRef{CorporateNumber: "9999999999999"}, kindUnchecked, nil},
		{"法人番号が空", companyRef{}, kindUnchecked, nil},
		{"既存と同じ", companyRef{CorporateNumber: "1000000000001", Existing: &listedStd}, kindSame, &listedStd},
		{"既存と差分", companyRef{CorporateNumber: "1000000000001", Existing: &marketValue{MarketType: "prime", IsListed: true}}, kindDiff, &listedStd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := decide(tt.c, edinet, jpx)
			if d.Kind != tt.wantKind {
				t.Fatalf("kind=%s want %s", d.Kind, tt.wantKind)
			}
			if (d.After == nil) != (tt.want == nil) || (tt.want != nil && *d.After != *tt.want) {
				t.Fatalf("after=%+v want %+v", d.After, tt.want)
			}
		})
	}
}

// 既存の値は --overwrite が無ければ書き込み対象にならず、あれば差分だけが対象になる。
func TestPrintReportOverwrite(t *testing.T) {
	old := &marketValue{MarketType: "prime", IsListed: true}
	nw := &marketValue{MarketType: "standard", IsListed: true}
	ds := []decision{
		{CompanyID: 1, Kind: kindNew, After: nw},
		{CompanyID: 2, Kind: kindDiff, Before: old, After: nw},
		{CompanyID: 3, Kind: kindSame, Before: nw, After: nw},
		{CompanyID: 4, Kind: kindUnchecked},
	}
	if w := printReport(ds, false); len(w) != 1 || w[0].CompanyID != 1 {
		t.Fatalf("overwrite なし: %+v", w)
	}
	if w := printReport(ds, true); len(w) != 2 || w[1].CompanyID != 2 || w[1].Before == nil {
		t.Fatalf("overwrite あり: %+v", w)
	}
}

func TestBackupAndRestoreAction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b.json")
	rows := []backupRow{{CompanyID: 1, After: marketValue{IsListed: true}}}
	if err := writeBackup(path, rows); err != nil {
		t.Fatal(err)
	}
	if err := writeBackup(path, rows); err == nil {
		t.Fatal("既存のファイルを上書きした")
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `"company_id": 1`) {
		t.Fatalf("%s", b)
	}
	after := marketValue{MarketType: "growth", IsListed: true, StockCode: "1111"}
	before := &marketValue{MarketType: "prime", IsListed: true}
	tests := []struct {
		name string
		cur  *marketValue
		r    backupRow
		want string
	}{
		{"新規は行を消す", &after, backupRow{After: after}, "delete"},
		{"更新は元の値へ", &after, backupRow{Before: before, After: after}, "update"},
		{"その後に変わった値は触らない", &marketValue{MarketType: "prime"}, backupRow{After: after}, "skip"},
		{"行が無ければ何もしない", nil, backupRow{After: after}, "skip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := restoreAction(tt.cur, tt.r); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}

func TestWriteAllowed(t *testing.T) {
	tests := []struct {
		name      string
		allowProd bool
		host, env string
		wantErr   bool
	}{
		{"localhost", false, "localhost", "development", false},
		{"127.0.0.1", false, "127.0.0.1", "", false},
		{"本番の環境変数", false, "127.0.0.1", "production", true},
		{"ローカルでないホスト", false, "db.example.internal", "", true},
		{"--allow-production なら許す", true, "db.example.internal", "production", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeAllowed(tt.allowProd, tt.host, tt.env); (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestFindXLSXLinkAndDownload(t *testing.T) {
	l, err := findXLSXLink("https://example.test/a/b/01.html", []byte(`<a href="/x/y/data_j.xlsx">x</a>`))
	if err != nil || l != "https://example.test/x/y/data_j.xlsx" {
		t.Fatalf("%q %v", l, err)
	}
	if _, err := findXLSXLink("https://example.test/", []byte("none")); err == nil {
		t.Fatal("リンクが無いのにエラーにならない")
	}
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	b, err := download(context.Background(), srv.URL)
	if err != nil || string(b) != "ok" || ua != userAgent {
		t.Fatalf("%q %q %v", b, ua, err)
	}
}
