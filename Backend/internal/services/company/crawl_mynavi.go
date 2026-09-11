package company

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
)

// executeMynaviCompanyCrawl マイナビ企業ページをgoqueryでパースして企業情報を保存する
func (s *CrawlService) executeMynaviCompanyCrawl(source *models.CrawlSource) error {
	body, charset, err := fetchBytes(source.SourceURL)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty content from source_url")
	}

	// Shift-JISの場合はUTF-8に変換
	utf8Body, err := decodeToUTF8(body, charset)
	if err != nil {
		log.Printf("[MynaviCrawl] charset decode failed, using raw bytes: %v", err)
		utf8Body = body
	}

	extracted, err := parseMynaviCompanyPage(utf8Body)
	if err != nil {
		return fmt.Errorf("failed to parse mynavi page: %w", err)
	}
	if strings.TrimSpace(extracted.Name) == "" {
		return errors.New("could not extract company name from mynavi page")
	}

	now := time.Now()
	// レートリミット対応（次回クロール時への配慮）
	time.Sleep(1 * time.Second)

	company, err := s.companyRepo.FindByName(extracted.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            extracted.Name,
			Description:     extracted.Description,
			Industry:        extracted.Industry,
			EmployeeCount:   extracted.EmployeeCount,
			FoundedYear:     extracted.FoundedYear,
			Location:        extracted.Location,
			WebsiteURL:      extracted.WebsiteURL,
			Culture:         extracted.Culture,
			WelfareDetails:  extracted.WelfareDetails,
			MainBusiness:    extracted.MainBusiness,
			AverageAge:      extracted.AverageAge,
			FemaleRatio:     extracted.FemaleRatio,
			SourceType:      source.SourceType,
			SourceURL:       source.SourceURL,
			SourceFetchedAt: &now,
			IsProvisional:   true,
			DataStatus:      "draft",
		}
		return s.companyRepo.Create(newCompany)
	}

	// 既存企業を更新（空でないフィールドのみ上書き）
	if extracted.Description != "" {
		company.Description = extracted.Description
	}
	if extracted.Industry != "" {
		company.Industry = extracted.Industry
	}
	if extracted.EmployeeCount > 0 {
		company.EmployeeCount = extracted.EmployeeCount
	}
	if extracted.FoundedYear > 0 {
		company.FoundedYear = extracted.FoundedYear
	}
	if extracted.Location != "" {
		company.Location = extracted.Location
	}
	if extracted.WebsiteURL != "" {
		company.WebsiteURL = extracted.WebsiteURL
	}
	if extracted.Culture != "" {
		company.Culture = extracted.Culture
	}
	if extracted.WelfareDetails != "" {
		company.WelfareDetails = extracted.WelfareDetails
	}
	if extracted.MainBusiness != "" {
		company.MainBusiness = extracted.MainBusiness
	}
	if extracted.AverageAge > 0 {
		company.AverageAge = extracted.AverageAge
	}
	if extracted.FemaleRatio > 0 {
		company.FemaleRatio = extracted.FemaleRatio
	}
	company.SourceType = source.SourceType
	company.SourceURL = source.SourceURL
	company.SourceFetchedAt = &now
	return s.companyRepo.Update(company)
}

type mynaviCompanyData struct {
	Name           string
	Industry       string
	FoundedYear    int
	EmployeeCount  int
	Location       string
	Description    string
	MainBusiness   string
	WebsiteURL     string
	WelfareDetails string
	Culture        string
	AverageAge     float64
	FemaleRatio    float64
}

// parseMynaviCompanyPage goqueryでマイナビ企業ページをパースする
func parseMynaviCompanyPage(htmlBytes []byte) (*mynaviCompanyData, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, err
	}

	data := &mynaviCompanyData{}

	// 企業名: h1.companyName、またはtitleから抽出
	data.Name = strings.TrimSpace(doc.Find("h1.companyName").First().Text())
	if data.Name == "" {
		data.Name = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	if data.Name == "" {
		// titleタグからフォールバック（「会社名 | マイナビ」形式）
		title := doc.Find("title").Text()
		if idx := strings.Index(title, "|"); idx > 0 {
			data.Name = strings.TrimSpace(title[:idx])
		} else {
			data.Name = strings.TrimSpace(title)
		}
	}

	// 企業情報テーブルをパース（th/td形式）
	doc.Find("table.companyInfoTable tr, table.baseInfo tr").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("th").Text())
		value := strings.TrimSpace(s.Find("td").Text())
		if label == "" || value == "" {
			return
		}
		applyMynaviField(data, label, value)
	})

	// テーブル形式でない場合のdl/dt/ddパターン
	doc.Find("dl").Each(func(_ int, dl *goquery.Selection) {
		dl.Find("dt").Each(func(i int, dt *goquery.Selection) {
			label := strings.TrimSpace(dt.Text())
			dd := dl.Find("dd").Eq(i)
			value := strings.TrimSpace(dd.Text())
			if label != "" && value != "" {
				applyMynaviField(data, label, value)
			}
		})
	})

	// 企業概要テキストのフォールバック
	if data.Description == "" {
		data.Description = strings.TrimSpace(doc.Find(".companyOverview, .corp-outline, .corpOutline").First().Text())
	}
	if data.Description == "" {
		data.Description = strings.TrimSpace(doc.Find("section.overview p, div.overview p").First().Text())
	}

	return data, nil
}

// applyMynaviField ラベル文字列に基づきフィールドを設定する
func applyMynaviField(data *mynaviCompanyData, label, value string) {
	switch {
	case shared.ContainsAny(label, []string{"業種", "業界"}):
		if data.Industry == "" {
			data.Industry = value
		}
	case shared.ContainsAny(label, []string{"設立", "創業"}):
		if data.FoundedYear == 0 {
			data.FoundedYear = extractYear(value)
		}
	case shared.ContainsAny(label, []string{"従業員", "社員数", "人数"}):
		if data.EmployeeCount == 0 {
			data.EmployeeCount = extractInt(value)
		}
	case shared.ContainsAny(label, []string{"所在地", "本社"}):
		if data.Location == "" {
			data.Location = value
		}
	case shared.ContainsAny(label, []string{"事業内容", "主要事業", "ビジネス"}):
		if data.MainBusiness == "" {
			data.MainBusiness = value
		}
	case shared.ContainsAny(label, []string{"企業概要", "会社概要", "概要"}):
		if data.Description == "" {
			data.Description = value
		}
	case shared.ContainsAny(label, []string{"URL", "ホームページ", "WEB", "Web"}):
		if data.WebsiteURL == "" {
			data.WebsiteURL = value
		}
	case shared.ContainsAny(label, []string{"福利厚生", "待遇", "諸手当"}):
		if data.WelfareDetails == "" {
			data.WelfareDetails = value
		}
	case shared.ContainsAny(label, []string{"社風", "企業文化", "カルチャー", "雰囲気"}):
		if data.Culture == "" {
			data.Culture = value
		}
	case shared.ContainsAny(label, []string{"平均年齢"}):
		if data.AverageAge == 0 {
			data.AverageAge = extractFloat(value)
		}
	case shared.ContainsAny(label, []string{"女性比率", "女性割合", "女性社員"}):
		if data.FemaleRatio == 0 {
			data.FemaleRatio = extractFloat(value)
		}
	}
}

// extractYear 文字列から西暦年を抽出する（例: "2005年4月" → 2005）
func extractYear(s string) int {
	re := regexp.MustCompile(`(19|20)\d{2}`)
	m := re.FindString(s)
	if m == "" {
		return 0
	}
	v, _ := strconv.Atoi(m)
	return v
}

// extractInt 文字列から最初の整数を抽出する（例: "1,234名" → 1234）
func extractInt(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	re := regexp.MustCompile(`\d+`)
	m := re.FindString(s)
	if m == "" {
		return 0
	}
	v, _ := strconv.Atoi(m)
	return v
}

// extractFloat 文字列から最初の浮動小数点数を抽出する（例: "32.5歳" → 32.5）
func extractFloat(s string) float64 {
	re := regexp.MustCompile(`\d+(\.\d+)?`)
	m := re.FindString(s)
	if m == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(m, 64)
	return v
}

// Exported type alias and wrappers for testing from external packages.

type MynaviCompanyData = mynaviCompanyData

func ParseMynaviCompanyPage(htmlBytes []byte) (*MynaviCompanyData, error) {
	return parseMynaviCompanyPage(htmlBytes)
}
func ExtractYear(s string) int      { return extractYear(s) }
func ExtractInt(s string) int       { return extractInt(s) }
func ExtractFloat(s string) float64 { return extractFloat(s) }
