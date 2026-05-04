package services

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type MynaviCompanyData struct {
	Name           string
	Industry       string
	EmployeeCount  int
	FoundedYear    int
	Location       string
	WebsiteURL     string
	Description    string
	MainBusiness   string
	Culture        string
	WorkStyle      string
	WelfareDetails string
	AverageAge     float64
	FemaleRatio    float64
}

// ParseMynaviCompanyPagePublic はテスト・CLIから呼び出せる公開ラッパー。
func ParseMynaviCompanyPagePublic(rawHTML string) (*MynaviCompanyData, error) {
	return parseMynaviCompanyPage(rawHTML)
}

// parseMynaviCompanyPage はマイナビ企業詳細ページのHTMLをパースして企業情報を返す。
func parseMynaviCompanyPage(rawHTML string) (*MynaviCompanyData, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, err
	}

	data := &MynaviCompanyData{}

	data.Name = extractCompanyName(doc)

	// 業種セクション（h2直後のul/li）を最優先で解析
	parseIndustrySection(doc, data)
	// th/td 形式（業種以外を強制上書き）
	parseTableRows(doc, data)
	// td.heading + td.sameSize 形式（事業内容などマイナビ独自レイアウト）
	parseLabeledTdRows(doc, data)
	// dl/dt/dd 形式で未取得フィールドを補完
	parseDlRows(doc, data)
	// .companyMessage / .companyIntroduction クラスのテキストを会社概要として取得
	parseCompanyMessage(doc, data)
	// 会社説明テキストセクション
	parseDescriptionSections(doc, data)
	// WebサイトURL の抽出（テーブル解析後に実施）
	if data.WebsiteURL == "" {
		data.WebsiteURL = extractWebsiteURL(doc)
	}

	return data, nil
}

func extractCompanyName(doc *goquery.Document) string {
	candidates := []string{
		".corpName",
		".corp-name",
		".companyName",
		"#corp-name",
		"h1.name",
		".corp-head h1",
		".corp-head__name",
		".company-profile__name",
		"h1",
	}
	for _, sel := range candidates {
		if text := strings.TrimSpace(doc.Find(sel).First().Text()); text != "" {
			return text
		}
	}
	return ""
}

func extractWebsiteURL(doc *goquery.Document) string {
	// テーブルの CSR活動 や ホームページ フィールドから URL を取得
	var found string
	doc.Find("table tr").EachWithBreak(func(_ int, tr *goquery.Selection) bool {
		th := strings.TrimSpace(tr.Find("th").First().Text())
		if mynaviContainsAny(th, "CSR活動", "ホームページ", "HP", "公式サイト", "URL") {
			td := strings.TrimSpace(tr.Find("td").First().Text())
			if strings.HasPrefix(td, "http") {
				found = td
				return false
			}
			// td 内のリンクも確認
			tr.Find("td a").EachWithBreak(func(_ int, a *goquery.Selection) bool {
				if href, exists := a.Attr("href"); exists && strings.HasPrefix(href, "http") {
					found = href
					return false
				}
				return true
			})
			if found != "" {
				return false
			}
		}
		return true
	})
	if found != "" {
		return found
	}
	// リンクテキストから探す
	doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, exists := s.Attr("href")
		if !exists {
			return true
		}
		text := strings.TrimSpace(s.Text())
		if mynaviContainsAny(text, "ホームページ", "公式サイト", "HP") {
			if strings.HasPrefix(href, "http") {
				found = href
				return false
			}
		}
		return true
	})
	return found
}

// parseTableRows は table/th/td 形式のデータを解析する。
func parseTableRows(doc *goquery.Document, data *MynaviCompanyData) {
	doc.Find("table tr").Each(func(_ int, tr *goquery.Selection) {
		th := mynaviNormalizeText(tr.Find("th").First().Text())
		td := mynaviNormalizeText(tr.Find("td").First().Text())
		if th != "" && td != "" {
			assignFieldForce(th, td, data)
		}
	})
}

// parseDlRows は dl/dt/dd 形式のデータを解析して空フィールドを補完する。
func parseDlRows(doc *goquery.Document, data *MynaviCompanyData) {
	doc.Find("dl").Each(func(_ int, dl *goquery.Selection) {
		dl.Find("dt").Each(func(_ int, dt *goquery.Selection) {
			label := mynaviNormalizeText(dt.Text())
			value := mynaviNormalizeText(dt.Next().Text())
			// "本社" の dt/dd は都道府県のみのため Location には使わない
			if label == "本社" {
				return
			}
			assignField(label, value, data)
		})
	})
}

// parseLabeledTdRows は <td class="heading">ラベル</td><td class="sameSize">値</td> パターンを解析する。
func parseLabeledTdRows(doc *goquery.Document, data *MynaviCompanyData) {
	doc.Find("table tr").Each(func(_ int, tr *goquery.Selection) {
		tds := tr.Find("td")
		if tds.Length() < 2 {
			return
		}
		first := tds.Eq(0)
		if _, exists := first.Attr("class"); !exists {
			return
		}
		label := mynaviNormalizeText(first.Text())
		value := mynaviNormalizeText(tds.Eq(1).Text())
		if label != "" && value != "" {
			assignField(label, value, data)
		}
	})
}

// parseCompanyMessage は .companyMessage / .companyIntroduction クラスのテキストを会社概要として設定する。
func parseCompanyMessage(doc *goquery.Document, data *MynaviCompanyData) {
	if data.Description != "" {
		return
	}
	selectors := []string{".companyMessage", ".company-message", ".companyIntroduction", ".corp-introduction"}
	for _, sel := range selectors {
		text := mynaviNormalizeText(doc.Find(sel).First().Text())
		if text != "" && len([]rune(text)) <= 1000 {
			data.Description = text
			return
		}
	}
}

// parseIndustrySection は <h2>業種</h2> の直後にある li テキストを結合して業種を設定する。
func parseIndustrySection(doc *goquery.Document, data *MynaviCompanyData) {
	if data.Industry != "" {
		return
	}
	doc.Find("h2").Each(func(_ int, h *goquery.Selection) {
		if strings.TrimSpace(h.Text()) != "業種" {
			return
		}
		var parts []string
		h.Next().Find("li").Each(func(_ int, li *goquery.Selection) {
			if t := strings.TrimSpace(li.Text()); t != "" {
				parts = append(parts, t)
			}
		})
		if len(parts) > 0 {
			data.Industry = strings.Join(parts, "・")
		}
	})
}

// parseDescriptionSections は見出し直後の短いテキストを補完フィールドとして使う。
func parseDescriptionSections(doc *goquery.Document, data *MynaviCompanyData) {
	doc.Find("h2, h3").Each(func(_ int, h *goquery.Selection) {
		label := strings.TrimSpace(h.Text())
		body := mynaviNormalizeText(h.Next().Text())
		// 長すぎる or 空のテキストはスキップ
		if body == "" || len([]rune(body)) > 300 {
			return
		}
		assignField(label, body, data)
	})
}

// assignFieldForce はフィールドを強制上書きする（th/td 優先処理用）。
func assignFieldForce(label, value string, data *MynaviCompanyData) {
	if value == "" {
		return
	}
	switch {
	case mynaviContainsAny(label, "業種", "業界"):
		if data.Industry == "" {
			data.Industry = value
		}
	case mynaviContainsAny(label, "従業員", "社員数"):
		if n := extractInt(value); n > 0 {
			data.EmployeeCount = n
		}
	case mynaviContainsAny(label, "設立"):
		if y := extractYear(value); y > 0 {
			data.FoundedYear = y
		}
	case mynaviContainsAny(label, "本社所在地", "所在地"):
		data.Location = value
	case mynaviContainsAny(label, "平均年齢"):
		if f := extractFloat(value); f > 0 {
			data.AverageAge = f
		}
	case mynaviContainsAny(label, "女性比率", "女性割合"):
		if f := extractFloat(value); f > 0 {
			data.FemaleRatio = f
		}
	case label == "勤務形態" || label == "勤務地":
		data.WorkStyle = value
	case mynaviContainsAny(label, "福利厚生"):
		data.WelfareDetails = value
	case mynaviContainsAny(label, "事業内容", "主な事業"):
		data.MainBusiness = value
	case mynaviContainsAny(label, "会社概要", "企業概要"):
		data.Description = value
	}
}

// assignField は未設定フィールドのみ補完する（dt/dd や見出しセクション用）。
func assignField(label, value string, data *MynaviCompanyData) {
	if value == "" {
		return
	}
	switch {
	case mynaviContainsAny(label, "業種", "業界"):
		if data.Industry == "" {
			data.Industry = value
		}
	case mynaviContainsAny(label, "従業員", "社員数"):
		if data.EmployeeCount == 0 {
			if n := extractInt(value); n > 0 {
				data.EmployeeCount = n
			}
		}
	case mynaviContainsAny(label, "設立"):
		if data.FoundedYear == 0 {
			if y := extractYear(value); y > 0 {
				data.FoundedYear = y
			}
		}
	case mynaviContainsAny(label, "所在地"):
		if data.Location == "" {
			data.Location = value
		}
	case mynaviContainsAny(label, "平均年齢"):
		if data.AverageAge == 0 {
			data.AverageAge = extractFloat(value)
		}
	case mynaviContainsAny(label, "女性比率", "女性割合"):
		if data.FemaleRatio == 0 {
			data.FemaleRatio = extractFloat(value)
		}
	case label == "勤務形態" || label == "勤務地":
		if data.WorkStyle == "" {
			data.WorkStyle = value
		}
	case mynaviContainsAny(label, "福利厚生"):
		if data.WelfareDetails == "" {
			data.WelfareDetails = value
		}
	case mynaviContainsAny(label, "事業内容", "主な事業"):
		if data.MainBusiness == "" {
			data.MainBusiness = value
		}
	case mynaviContainsAny(label, "会社概要", "企業概要"):
		if data.Description == "" {
			data.Description = value
		}
	}
}

// mynaviContainsAny は s がいずれかのキーワードを含むか判定する。
func mynaviContainsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// mynaviNormalizeText は連続する空白を1つのスペースに正規化する。
func mynaviNormalizeText(s string) string {
	s = strings.TrimSpace(s)
	return reWhitespace.ReplaceAllString(s, " ")
}

var reDigits = regexp.MustCompile(`[\d,]+`)
var reFloat = regexp.MustCompile(`[\d.]+`)
var reYear = regexp.MustCompile(`(19|20)\d{2}`)
var reWhitespace = regexp.MustCompile(`\s+`)
var reWareki = map[string]int{
	"昭和": 1925,
	"平成": 1988,
	"令和": 2018,
}

func extractInt(s string) int {
	clean := strings.ReplaceAll(s, ",", "")
	m := reDigits.FindString(clean)
	v, _ := strconv.Atoi(m)
	return v
}

func extractYear(s string) int {
	// 西暦（4桁）を優先
	if m := reYear.FindString(s); m != "" {
		v, _ := strconv.Atoi(m)
		return v
	}
	// 和暦変換
	for era, base := range reWareki {
		re := regexp.MustCompile(era + `(\d+)年`)
		if m := re.FindStringSubmatch(s); len(m) > 1 {
			n, _ := strconv.Atoi(m[1])
			return base + n
		}
	}
	return 0
}

func extractFloat(s string) float64 {
	m := reFloat.FindString(s)
	if m == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(m, 64)
	return v
}
