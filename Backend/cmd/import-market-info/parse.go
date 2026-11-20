package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// maxEntryBytes は、zip 内の 1 ファイルを読む上限（zip 爆弾対策）。
const maxEntryBytes = 200 << 20

// edinetEntry は、EDINET コードリストの 1 行（必要な列だけ）。
type edinetEntry struct {
	Listing         string // 上場 / 非上場 / 空
	SecCode         string // 5桁（無ければ空）
	CorporateNumber string // 13桁
}

func readZipEntry(zr *zip.Reader, match func(name string) bool) ([]byte, error) {
	for _, f := range zr.File {
		if !match(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes+1))
		if err != nil {
			return nil, err
		}
		if len(b) > maxEntryBytes {
			return nil, fmt.Errorf("%s が大きすぎます", f.Name)
		}
		return b, nil
	}
	return nil, errors.New("zip の中に目的のファイルがありません")
}

// parseEDINETZip は Edinetcode.zip（中身は cp932 の CSV）を、法人番号 → 行 の表にする。
func parseEDINETZip(data []byte) (map[string]edinetEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	b, err := readZipEntry(zr, func(n string) bool { return strings.HasSuffix(strings.ToLower(n), ".csv") })
	if err != nil {
		return nil, err
	}
	return parseEDINETCSV(b)
}

// parseEDINETCSV は cp932 の CSV を解析する。1行目はメタ情報、ヘッダー行は列名で探す。
func parseEDINETCSV(raw []byte) (map[string]edinetEntry, error) {
	utf8b, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), raw)
	if err != nil {
		return nil, fmt.Errorf("cp932 の変換に失敗: %w", err)
	}
	r := csv.NewReader(bytes.NewReader(utf8b))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	iList, iSec, iNum := -1, -1, -1
	out := map[string]edinetEntry{}
	for _, rec := range recs {
		if iNum < 0 { // ヘッダー行を探す（それ以前はメタ情報）
			for i, h := range rec {
				switch strings.TrimSpace(h) {
				case "上場区分":
					iList = i
				case "証券コード":
					iSec = i
				case "提出者法人番号":
					iNum = i
				}
			}
			continue
		}
		if iList < 0 || iSec < 0 || len(rec) <= max(iList, iSec, iNum) {
			continue
		}
		e := edinetEntry{Listing: strings.TrimSpace(rec[iList]), SecCode: strings.TrimSpace(rec[iSec]), CorporateNumber: strings.TrimSpace(rec[iNum])}
		if !validCorpNumber(e.CorporateNumber) {
			continue
		}
		// 同じ法人番号が複数あるときは、上場区分が入っている行を優先する
		if old, ok := out[e.CorporateNumber]; ok && (old.Listing != "" || e.Listing == "") {
			continue
		}
		out[e.CorporateNumber] = e
	}
	if iNum < 0 || iList < 0 || iSec < 0 {
		return nil, errors.New("EDINET の CSV に必要な列（上場区分・証券コード・提出者法人番号）がありません")
	}
	return out, nil
}

var corpNumberRe = regexp.MustCompile(`^[0-9]{13}$`)

func validCorpNumber(s string) bool { return corpNumberRe.MatchString(s) }

// parseJPXXLSX は data_j.xlsx を、コード（4桁） → 市場・商品区分 の表にする。
func parseJPXXLSX(data []byte) (map[string]string, error) {
	rows, err := readXLSX(data)
	if err != nil {
		return nil, err
	}
	iCode, iMarket := -1, -1
	out := map[string]string{}
	for _, row := range rows {
		if iCode < 0 {
			for i, h := range row {
				switch strings.TrimSpace(h) {
				case "コード":
					iCode = i
				case "市場・商品区分":
					iMarket = i
				}
			}
			if iMarket < 0 {
				iCode = -1
			}
			continue
		}
		if len(row) <= max(iCode, iMarket) {
			continue
		}
		if code := strings.TrimSpace(row[iCode]); code != "" {
			out[code] = strings.TrimSpace(row[iMarket])
		}
	}
	if iCode < 0 {
		return nil, errors.New("東証の一覧に必要な列（コード・市場・商品区分）がありません")
	}
	return out, nil
}

type xmlText struct {
	T string `xml:"t"`
	R []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func (s xmlText) text() string {
	if len(s.R) == 0 {
		return s.T
	}
	var sb strings.Builder
	for _, r := range s.R {
		sb.WriteString(r.T)
	}
	return sb.String()
}

// readXLSX は、最初のシート（worksheets/sheet1.xml）を行×列の文字列にする。
func readXLSX(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var shared []string
	if b, err := readZipEntry(zr, func(n string) bool { return n == "xl/sharedStrings.xml" }); err == nil {
		var sst struct {
			SI []xmlText `xml:"si"`
		}
		if err := xml.Unmarshal(b, &sst); err != nil {
			return nil, err
		}
		for _, s := range sst.SI {
			shared = append(shared, s.text())
		}
	}
	b, err := readZipEntry(zr, func(n string) bool { return n == "xl/worksheets/sheet1.xml" })
	if err != nil {
		return nil, err
	}
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Ref string  `xml:"r,attr"`
				T   string  `xml:"t,attr"`
				V   string  `xml:"v"`
				IS  xmlText `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(b, &sheet); err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(sheet.Rows))
	for _, r := range sheet.Rows {
		var row []string
		for _, c := range r.Cells {
			col := colIndex(c.Ref)
			if col < 0 {
				continue
			}
			for len(row) <= col {
				row = append(row, "")
			}
			switch c.T {
			case "s":
				if i, err := strconv.Atoi(c.V); err == nil && i >= 0 && i < len(shared) {
					row[col] = shared[i]
				}
			case "inlineStr":
				row[col] = c.IS.text()
			default: // 数値・str
				row[col] = c.V
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// colIndex は "B2" → 1。
func colIndex(ref string) int {
	n := 0
	i := 0
	for ; i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z'; i++ {
		n = n*26 + int(ref[i]-'A') + 1
	}
	if i == 0 {
		return -1
	}
	return n - 1
}
