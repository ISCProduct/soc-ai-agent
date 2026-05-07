package services

import (
	"Backend/internal/models"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
)

// openworkScores OpenWorkから取得した評価スコアを保持する
type openworkScores struct {
	Overall  float64 `json:"overall"`   // 総合評価（5点満点）
	Welfare  float64 `json:"welfare"`   // 待遇面の満足度
	Morale   float64 `json:"morale"`    // 社員の士気
	Openness float64 `json:"openness"`  // 風通しの良さ
	Growth   float64 `json:"growth"`    // 20代成長環境
}

// openworkCompanyData OpenWorkページからパースしたデータ
type openworkCompanyData struct {
	Name          string
	Industry      string
	Scores        openworkScores
	AverageSalary int // 平均年収（万円）
	ReviewCount   int // 口コミ件数（ログのみ）
}

// executeOpenworkCompanyCrawl OpenWork企業ページをスクレイピングしてDB保存する
func (s *CrawlService) executeOpenworkCompanyCrawl(source *models.CrawlSource) error {
	body, charset, err := fetchOpenworkBytes(source.SourceURL)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty content from source_url")
	}

	utf8Body, err := decodeOpenworkToUTF8(body, charset)
	if err != nil {
		log.Printf("[OpenworkCrawl] charset decode failed, using raw bytes: %v", err)
		utf8Body = body
	}

	data, err := parseOpenworkCompanyPage(utf8Body)
	if err != nil {
		return fmt.Errorf("failed to parse openwork page: %w", err)
	}
	if strings.TrimSpace(data.Name) == "" {
		return errors.New("could not extract company name from openwork page")
	}

	log.Printf("[OpenworkCrawl] company=%s overall=%.1f reviews=%d", data.Name, data.Scores.Overall, data.ReviewCount)

	// レートリミット対応
	time.Sleep(2 * time.Second)

	now := time.Now()
	company, err := s.companyRepo.FindByName(data.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	cultureJSON := buildOpenworkCultureJSON(data.Scores)
	welfareText := buildOpenworkWelfareText(data.Scores.Welfare)
	mainBusinessText := buildOpenworkSalaryText(data.AverageSalary)

	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            data.Name,
			Industry:        data.Industry,
			Culture:         cultureJSON,
			WelfareDetails:  welfareText,
			MainBusiness:    mainBusinessText,
			SourceType:      source.SourceType,
			SourceURL:       source.SourceURL,
			SourceFetchedAt: &now,
			IsProvisional:   true,
			DataStatus:      "draft",
		}
		return s.companyRepo.Create(newCompany)
	}

	// 既存企業を更新（空でないフィールドのみ上書き）
	if data.Industry != "" {
		company.Industry = data.Industry
	}
	if cultureJSON != "" {
		company.Culture = cultureJSON
	}
	if welfareText != "" {
		company.WelfareDetails = welfareText
	}
	if mainBusinessText != "" {
		company.MainBusiness = mainBusinessText
	}
	company.SourceType = source.SourceType
	company.SourceURL = source.SourceURL
	company.SourceFetchedAt = &now
	return s.companyRepo.Update(company)
}

// parseOpenworkCompanyPage goqueryでOpenWork企業ページをパースする
func parseOpenworkCompanyPage(htmlBytes []byte) (*openworkCompanyData, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, err
	}

	data := &openworkCompanyData{}

	// 企業名: h1またはtitleからフォールバック
	data.Name = strings.TrimSpace(doc.Find("h1.p-companyTop__name, h1.company-name, h1").First().Text())
	if data.Name == "" {
		title := doc.Find("title").Text()
		// 「会社名 の評判・口コミ | OpenWork」形式
		for _, sep := range []string{" の評判", " の口コミ", "|", "｜"} {
			if idx := strings.Index(title, sep); idx > 0 {
				data.Name = strings.TrimSpace(title[:idx])
				break
			}
		}
	}

	// 業種
	data.Industry = strings.TrimSpace(doc.Find(".p-companyTop__industry, .js-companyIndustry, [class*='industry']").First().Text())

	// 総合評価スコア
	data.Scores.Overall = parseOpenworkScore(doc.Find(
		".p-totalScore__value, .total-score, .js-totalScore, [class*='totalScore'] [class*='value'], [class*='total'] [class*='score']",
	).First().Text())

	// カテゴリスコア: dl/dt/dd または li 形式でパース
	doc.Find("dl.p-scoreList, dl[class*='scoreList'], dl[class*='score-list']").Each(func(_ int, dl *goquery.Selection) {
		dl.Find("dt").Each(func(i int, dt *goquery.Selection) {
			label := strings.TrimSpace(dt.Text())
			val := parseOpenworkScore(strings.TrimSpace(dl.Find("dd").Eq(i).Text()))
			applyOpenworkScore(data, label, val)
		})
	})

	// li形式のスコアリスト
	doc.Find("li[class*='scoreItem'], li[class*='score-item'], .p-scoreItem").Each(func(_ int, li *goquery.Selection) {
		label := strings.TrimSpace(li.Find("[class*='label'], dt, .name").First().Text())
		val := parseOpenworkScore(li.Find("[class*='value'], dd, .score").First().Text())
		if label != "" && val > 0 {
			applyOpenworkScore(data, label, val)
		}
	})

	// 平均年収（数字+「万円」のパターン）
	doc.Find("[class*='salary'], [class*='income'], [class*='average']").Each(func(_ int, el *goquery.Selection) {
		text := strings.TrimSpace(el.Text())
		if strings.Contains(text, "万円") || strings.Contains(text, "年収") {
			if v := parseOpenworkSalary(text); v > 0 && data.AverageSalary == 0 {
				data.AverageSalary = v
			}
		}
	})

	// 口コミ件数（ログ用）
	doc.Find("[class*='review'], [class*='reviewCount'], [class*='クチコミ']").Each(func(_ int, el *goquery.Selection) {
		text := el.Text()
		if strings.Contains(text, "件") {
			if v := parseOpenworkReviewCount(text); v > 0 && data.ReviewCount == 0 {
				data.ReviewCount = v
			}
		}
	})

	return data, nil
}

// applyOpenworkScore ラベルに基づいてスコアフィールドに値を設定する
func applyOpenworkScore(data *openworkCompanyData, label string, val float64) {
	if val <= 0 {
		return
	}
	switch {
	case strings.Contains(label, "総合"):
		if data.Scores.Overall == 0 {
			data.Scores.Overall = val
		}
	case strings.Contains(label, "待遇") || strings.Contains(label, "報酬") || strings.Contains(label, "給与"):
		if data.Scores.Welfare == 0 {
			data.Scores.Welfare = val
		}
	case strings.Contains(label, "士気") || strings.Contains(label, "モチベ"):
		if data.Scores.Morale == 0 {
			data.Scores.Morale = val
		}
	case strings.Contains(label, "風通") || strings.Contains(label, "openness"):
		if data.Scores.Openness == 0 {
			data.Scores.Openness = val
		}
	case strings.Contains(label, "20代") || strings.Contains(label, "成長") || strings.Contains(label, "若手"):
		if data.Scores.Growth == 0 {
			data.Scores.Growth = val
		}
	}
}

// parseOpenworkScore テキストからスコア（0-5の浮動小数点）を抽出する
func parseOpenworkScore(s string) float64 {
	re := regexp.MustCompile(`[0-4]\.\d+|[0-5]`)
	m := re.FindString(strings.TrimSpace(s))
	if m == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(m, 64)
	if v < 0 || v > 5 {
		return 0
	}
	return v
}

// parseOpenworkSalary テキストから平均年収（万円）を抽出する
func parseOpenworkSalary(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	re := regexp.MustCompile(`(\d+)\s*万円`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		// 「XXX万円」形式でない場合、単純に数字を探す（例：500）
		re2 := regexp.MustCompile(`\d{3,4}`)
		n := re2.FindString(s)
		if n == "" {
			return 0
		}
		v, _ := strconv.Atoi(n)
		return v
	}
	v, _ := strconv.Atoi(m[1])
	return v
}

// parseOpenworkReviewCount テキストから口コミ件数を抽出する
func parseOpenworkReviewCount(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	re := regexp.MustCompile(`(\d+)\s*件`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	v, _ := strconv.Atoi(m[1])
	return v
}

// buildOpenworkCultureJSON スコアをJSON文字列に変換してCultureフィールドに格納する
func buildOpenworkCultureJSON(scores openworkScores) string {
	if scores.Overall == 0 && scores.Morale == 0 && scores.Openness == 0 && scores.Growth == 0 {
		return ""
	}
	b, err := json.Marshal(map[string]float64{
		"overall_score":  scores.Overall,
		"morale_score":   scores.Morale,
		"openness_score": scores.Openness,
		"growth_score":   scores.Growth,
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// buildOpenworkWelfareText 待遇スコアを文字列に変換する
func buildOpenworkWelfareText(welfareScore float64) string {
	if welfareScore <= 0 {
		return ""
	}
	return fmt.Sprintf("待遇満足度スコア: %.1f/5.0（OpenWork）", welfareScore)
}

// buildOpenworkSalaryText 平均年収を文字列に変換する
func buildOpenworkSalaryText(salary int) string {
	if salary <= 0 {
		return ""
	}
	return fmt.Sprintf("平均年収: %d万円（OpenWork）", salary)
}

// fetchOpenworkBytes URLからレスポンスのバイト列とcharsetを返す
func fetchOpenworkBytes(url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
	req.Header.Set("Accept-Language", "ja,en;q=0.9")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fetch failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	charset := extractOpenworkCharset(ct)
	return body, charset, nil
}

// extractOpenworkCharset Content-TypeヘッダーからCharsetを抽出する
func extractOpenworkCharset(contentType string) string {
	lower := strings.ToLower(contentType)
	if strings.Contains(lower, "shift_jis") || strings.Contains(lower, "shift-jis") || strings.Contains(lower, "sjis") {
		return "shift_jis"
	}
	if strings.Contains(lower, "euc-jp") {
		return "euc-jp"
	}
	return "utf-8"
}

// decodeOpenworkToUTF8 Shift-JIS/EUC-JPをUTF-8に変換する
func decodeOpenworkToUTF8(b []byte, charset string) ([]byte, error) {
	switch charset {
	case "shift_jis":
		decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), b)
		return decoded, err
	case "euc-jp":
		decoded, _, err := transform.Bytes(japanese.EUCJP.NewDecoder(), b)
		return decoded, err
	default:
		return b, nil
	}
}
