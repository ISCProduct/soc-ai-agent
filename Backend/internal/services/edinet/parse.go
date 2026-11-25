package edinet

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

// Fact は有価証券報告書から取り出した数字1項目。
type Fact struct {
	Field string // company_facts.field
	Value string
	Unit  string
}

// Section は有価証券報告書から取り出した文章1つ。
type Section struct {
	Key   string // company_documents.source_ref の区分
	Title string
	Body  string
}

// AnnualReport は1通の有価証券報告書から取り出した内容。
type AnnualReport struct {
	Facts    []Fact
	Sections []Section
}

const (
	ctxConsolidated = "CurrentYearInstant"
	ctxSingle       = "CurrentYearInstant_NonConsolidatedMember"
	ctxDuration     = "CurrentYearDuration"
	ctxDurationS    = "CurrentYearDuration_NonConsolidatedMember"
	ctxFiling       = "FilingDateInstant"
)

// 平均年齢・勤続年数・年間給与は「提出会社（単体）」の数字しか載らない。持株会社だと本社の数人分になるので、
// 項目名にも single を付け、回答で単体と分かるようにする。
var factSpecs = []struct {
	field, element, context, unit string
}{
	{"employees_consolidated", "NumberOfEmployees", ctxConsolidated, "人"},
	{"employees_single", "NumberOfEmployees", ctxSingle, "人"},
	{"average_age_single", "AverageAgeYearsInformationAboutReportingCompanyInformationAboutEmployees", ctxSingle, "歳"},
	{"average_service_years_single", "AverageLengthOfServiceYearsInformationAboutReportingCompanyInformationAboutEmployees", ctxSingle, "年"},
	{"average_salary_single", "AverageAnnualSalaryInformationAboutReportingCompanyInformationAboutEmployees", ctxSingle, "円"},
}

// 売上高は会計基準・業種で要素名が違う。上から順に試す（2026-11-25 に60件で確認した要素）。
var salesElements = []string{
	"NetSalesSummaryOfBusinessResults",
	"RevenueIFRSSummaryOfBusinessResults",
	"OperatingRevenue1SummaryOfBusinessResults",
	"OperatingRevenue2SummaryOfBusinessResults",
	"NetSalesOfCompletedConstructionContractsSummaryOfBusinessResults",
	"RevenuesUSGAAPSummaryOfBusinessResults",
	"OrdinaryIncomeSummaryOfBusinessResults", // 銀行など
}

var sectionSpecs = []Section{
	{Key: "business", Title: "事業の内容"},
	{Key: "policy", Title: "経営方針、経営環境及び対処すべき課題等"},
	{Key: "employees", Title: "従業員の状況"},
	{Key: "affiliates", Title: "関係会社の状況"},
	{Key: "history", Title: "沿革"},
}

var sectionElements = map[string]string{
	"business":   "DescriptionOfBusinessTextBlock",
	"policy":     "BusinessPolicyBusinessEnvironmentIssuesToAddressEtcTextBlock",
	"employees":  "InformationAboutEmployeesTextBlock",
	"affiliates": "OverviewOfAffiliatedEntitiesTextBlock",
	"history":    "CompanyHistoryTextBlock",
}

// ParseAnnualReportZip は書類取得 API（type=5）の ZIP から有価証券報告書本文の CSV を読み、数字と文章を返す。
// CSV は UTF-16LE・タブ区切りで、列は「要素ID・項目名・コンテキストID・相対年度・連結・個別・期間・時点・ユニットID・単位・値」。
func ParseAnnualReportZip(zipBytes []byte) (*AnnualReport, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("edinet: ZIP を開けません: %w", err)
	}
	var f *zip.File
	for _, zf := range zr.File {
		// 本文は jpcrp030000-asr-*.csv。監査報告書（jpaud-*）は使わない。
		if strings.Contains(zf.Name, "XBRL_TO_CSV/jpcrp") && strings.HasSuffix(zf.Name, ".csv") {
			f = zf
			break
		}
	}
	if f == nil {
		return nil, errors.New("edinet: ZIP に有価証券報告書本文の CSV がありません")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, 100*1024*1024))
	if err != nil {
		return nil, err
	}
	return parseCSV(decodeUTF16LE(raw))
}

func parseCSV(text string) (*AnnualReport, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("edinet: CSV の解析に失敗: %w", err)
	}
	// (要素名, コンテキストID) → 値。同じ組が複数あれば最初を使う。
	values := map[[2]string]string{}
	for i, row := range rows {
		if i == 0 || len(row) < 9 {
			continue
		}
		el := row[0]
		if j := strings.LastIndex(el, ":"); j >= 0 {
			el = el[j+1:]
		}
		key := [2]string{el, row[2]}
		if _, ok := values[key]; !ok {
			values[key] = strings.TrimSpace(row[len(row)-1])
		}
	}
	get := func(el, ctx string) (string, bool) {
		v, ok := values[[2]string{el, ctx}]
		if !ok || v == "" || v == "－" || v == "-" {
			return "", false
		}
		return v, true
	}

	rep := &AnnualReport{}
	for _, s := range factSpecs {
		if v, ok := get(s.element, s.context); ok {
			rep.Facts = append(rep.Facts, Fact{Field: s.field, Value: v, Unit: s.unit})
		}
	}
	for _, el := range salesElements {
		if v, ok := get(el, ctxDuration); ok {
			rep.Facts = append(rep.Facts, Fact{Field: "net_sales_consolidated", Value: v, Unit: "円"})
			break
		}
		if v, ok := get(el, ctxDurationS); ok {
			rep.Facts = append(rep.Facts, Fact{Field: "net_sales_single", Value: v, Unit: "円"})
			break
		}
	}
	for _, s := range sectionSpecs {
		if v, ok := get(sectionElements[s.Key], ctxFiling); ok {
			s.Body = v
			rep.Sections = append(rep.Sections, s)
		}
	}
	return rep, nil
}

// decodeUTF16LE は BOM の有無にかかわらず UTF-16LE を文字列にする。
func decodeUTF16LE(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		b = b[2:]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}
