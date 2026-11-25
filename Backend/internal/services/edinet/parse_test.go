package edinet

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"unicode/utf16"
)

// testZip は書類取得 API と同じ形（UTF-16LE・タブ区切りの CSV を ZIP に入れたもの）を作る。値は合成データ。
func testZip(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var lines []string
	lines = append(lines, strings.Join([]string{"要素ID", "項目名", "コンテキストID", "相対年度", "連結・個別", "期間・時点", "ユニットID", "単位", "値"}, "\t"))
	for _, r := range rows {
		lines = append(lines, strings.Join([]string{r[0], "", r[1], "", "", "", "", "", r[2]}, "\t"))
	}
	u := utf16.Encode([]rune(strings.Join(lines, "\r\n")))
	raw := []byte{0xFF, 0xFE}
	for _, c := range u {
		raw = append(raw, byte(c), byte(c>>8))
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string][]byte{
		"XBRL_TO_CSV/jpaud-aai-cc-001_E99999-000_2025-03-31_01_2025-06-25.csv":    []byte("監査報告書"),
		"XBRL_TO_CSV/jpcrp030000-asr-001_E99999-000_2025-03-31_01_2025-06-25.csv": raw,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(body)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseAnnualReportZip(t *testing.T) {
	z := testZip(t, [][]string{
		{"jpcrp_cor:NumberOfEmployees", "Prior1YearInstant", "90"},
		{"jpcrp_cor:NumberOfEmployees", "CurrentYearInstant", "110"},
		{"jpcrp_cor:NumberOfEmployees", "CurrentYearInstant_NonConsolidatedMember", "4"},
		{"jpcrp_cor:AverageAgeYearsInformationAboutReportingCompanyInformationAboutEmployees", "CurrentYearInstant_NonConsolidatedMember", "46.2"},
		{"jpcrp_cor:AverageAnnualSalaryInformationAboutReportingCompanyInformationAboutEmployees", "CurrentYearInstant_NonConsolidatedMember", "6467745"},
		{"jpcrp_cor:AverageLengthOfServiceYearsInformationAboutReportingCompanyInformationAboutEmployees", "CurrentYearInstant_NonConsolidatedMember", "－"},
		{"jpcrp_cor:RevenueIFRSSummaryOfBusinessResults", "CurrentYearDuration", "2882052000"},
		{"jpcrp_cor:DescriptionOfBusinessTextBlock", "FilingDateInstant", "３【事業の内容】　当社グループは合成データです。"},
		{"jpcrp_cor:CompanyHistoryTextBlock", "FilingDateInstant", "２【沿革】合成データ"},
	})
	rep, err := ParseAnnualReportZip(z)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rep.Facts {
		got[f.Field] = f.Value + f.Unit
	}
	want := map[string]string{
		"employees_consolidated": "110人",
		"employees_single":       "4人",
		"average_age_single":     "46.2歳",
		"average_salary_single":  "6467745円",
		"net_sales_consolidated": "2882052000円", // IFRS の要素名でも拾う
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["average_service_years_single"]; ok {
		t.Error("「－」は値なしとして扱う")
	}
	if len(rep.Sections) != 2 || rep.Sections[0].Key != "business" || !strings.Contains(rep.Sections[0].Body, "合成データ") {
		t.Errorf("文章の取り出しが違う: %+v", rep.Sections)
	}
}

func TestParseAnnualReportZip_NoBody(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("XBRL_TO_CSV/jpaud-aai-cc-001.csv")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	if _, err := ParseAnnualReportZip(buf.Bytes()); err == nil {
		t.Error("本文の CSV が無ければエラーにする")
	}
}

func TestDocumentIsAnnualReport(t *testing.T) {
	base := Document{DocTypeCode: "120", OrdinanceCode: "010", WithdrawalStatus: "0", CSVFlag: "1", JCN: "1234567890123"}
	if !base.IsAnnualReport() {
		t.Error("一般企業の有報は対象")
	}
	for name, d := range map[string]Document{
		"投資信託":  {DocTypeCode: "120", OrdinanceCode: "030", WithdrawalStatus: "0", CSVFlag: "1", JCN: "1"},
		"取り下げ":  {DocTypeCode: "120", OrdinanceCode: "010", WithdrawalStatus: "1", CSVFlag: "1", JCN: "1"},
		"CSVなし": {DocTypeCode: "120", OrdinanceCode: "010", WithdrawalStatus: "0", CSVFlag: "0", JCN: "1"},
		"臨時報告書": {DocTypeCode: "180", OrdinanceCode: "010", WithdrawalStatus: "0", CSVFlag: "1", JCN: "1"},
	} {
		if d.IsAnnualReport() {
			t.Errorf("%s は対象外", name)
		}
	}
}
