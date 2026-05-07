package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
)

type CrawlService struct {
	repo        repository.CrawlRepository
	companyRepo repository.CompanyRepository
	popularRepo repository.CompanyPopularityRepository
	aiClient    *openai.Client
	mu          sync.Mutex
}

func NewCrawlService(repo repository.CrawlRepository, companyRepo repository.CompanyRepository, popularRepo repository.CompanyPopularityRepository, aiClient *openai.Client) *CrawlService {
	return &CrawlService{repo: repo, companyRepo: companyRepo, popularRepo: popularRepo, aiClient: aiClient}
}

type CrawlSourcePayload struct {
	Name         string `json:"name"`
	TargetType   string `json:"target_type"`
	SourceType   string `json:"source_type"`
	SourceURL    string `json:"source_url"`
	ScheduleType string `json:"schedule_type"`
	ScheduleDay  *int   `json:"schedule_day"`
	ScheduleTime string `json:"schedule_time"`
	IsActive     *bool  `json:"is_active"`
}

func (s *CrawlService) ListSources() ([]models.CrawlSource, error) {
	return s.repo.ListSources()
}

func (s *CrawlService) ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error) {
	return s.repo.ListRuns(sourceID, limit)
}

func (s *CrawlService) CreateSource(payload CrawlSourcePayload) (*models.CrawlSource, error) {
	if payload.ScheduleDay == nil {
		return nil, errors.New("schedule_day is required")
	}
	source := &models.CrawlSource{
		Name:         strings.TrimSpace(payload.Name),
		TargetType:   strings.TrimSpace(payload.TargetType),
		SourceType:   strings.TrimSpace(payload.SourceType),
		SourceURL:    strings.TrimSpace(payload.SourceURL),
		ScheduleType: strings.TrimSpace(payload.ScheduleType),
		ScheduleDay:  *payload.ScheduleDay,
		ScheduleTime: strings.TrimSpace(payload.ScheduleTime),
		IsActive:     true,
	}
	if payload.IsActive != nil {
		source.IsActive = *payload.IsActive
	}
	if err := validateCrawlSource(source); err != nil {
		return nil, err
	}
	next := computeNextRun(time.Now(), source)
	source.NextRunAt = next
	if err := s.repo.CreateSource(source); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *CrawlService) UpdateSource(id uint, payload CrawlSourcePayload) (*models.CrawlSource, error) {
	source, err := s.repo.GetSource(id)
	if err != nil {
		return nil, err
	}
	if payload.Name != "" {
		source.Name = strings.TrimSpace(payload.Name)
	}
	if payload.TargetType != "" {
		source.TargetType = strings.TrimSpace(payload.TargetType)
	}
	if payload.SourceType != "" {
		source.SourceType = strings.TrimSpace(payload.SourceType)
	}
	if payload.SourceURL != "" {
		source.SourceURL = strings.TrimSpace(payload.SourceURL)
	}
	if payload.ScheduleType != "" {
		source.ScheduleType = strings.TrimSpace(payload.ScheduleType)
	}
	if payload.ScheduleDay != nil {
		source.ScheduleDay = *payload.ScheduleDay
	}
	if payload.ScheduleTime != "" {
		source.ScheduleTime = strings.TrimSpace(payload.ScheduleTime)
	}
	if payload.IsActive != nil {
		source.IsActive = *payload.IsActive
	}
	if err := validateCrawlSource(source); err != nil {
		return nil, err
	}
	source.NextRunAt = computeNextRun(time.Now(), source)
	if err := s.repo.UpdateSource(source); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *CrawlService) RunSource(id uint) (*models.CrawlRun, error) {
	source, err := s.repo.GetSource(id)
	if err != nil {
		return nil, err
	}
	return s.runSource(source)
}

func (s *CrawlService) StartScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.runDueSources()
	}
}

func (s *CrawlService) RunDueSources() {
	s.runDueSources()
}

func (s *CrawlService) runDueSources() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	sources, err := s.repo.ListDueSources(now)
	if err != nil {
		return
	}
	for i := range sources {
		_, _ = s.runSource(&sources[i])
	}
}

func (s *CrawlService) runSource(source *models.CrawlSource) (*models.CrawlRun, error) {
	if source == nil {
		return nil, errors.New("source not found")
	}
	run := &models.CrawlRun{
		SourceID:  source.ID,
		Status:    "running",
		Message:   "",
		StartedAt: time.Now(),
	}
	if err := s.repo.CreateRun(run); err != nil {
		return nil, err
	}

	message := ""
	if err := s.executeCrawl(source); err != nil {
		message = err.Error()
		run.Status = "failed"
	} else {
		run.Status = "success"
		message = "completed"
	}
	run.Message = message
	finished := time.Now()
	run.EndedAt = &finished
	_ = s.repo.UpdateRun(run)

	source.LastRunAt = &finished
	source.NextRunAt = computeNextRun(finished, source)
	_ = s.repo.UpdateSource(source)

	return run, nil
}

func (s *CrawlService) executeCrawl(source *models.CrawlSource) error {
	switch source.TargetType {
	case "company":
		return s.executeCompanyCrawl(source)
	case "popular_companies":
		return s.executePopularCompaniesCrawl(source)
	case "job_site_company":
		return s.executeJobSiteCompanyCrawl(source)
	case "job_listing":
		return s.executeJobListingCrawl(source)
	case "openwork_company":
		return s.executeOpenworkCompanyCrawl(source)
	default:
		return fmt.Errorf("unsupported target_type: %s", source.TargetType)
	}
}

func validateCrawlSource(source *models.CrawlSource) error {
	if strings.TrimSpace(source.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(source.TargetType) == "" {
		return errors.New("target_type is required")
	}
	if source.TargetType != "company" && source.TargetType != "popular_companies" && source.TargetType != "job_site_company" && source.TargetType != "job_listing" && source.TargetType != "openwork_company" {
		return errors.New("target_type must be company, popular_companies, job_site_company, job_listing, or openwork_company")
	}
	if source.TargetType == "popular_companies" && strings.TrimSpace(source.SourceURL) == "" {
		return errors.New("source_url is required for popular_companies")
	}
	if source.TargetType == "job_site_company" && strings.TrimSpace(source.SourceURL) == "" {
		return errors.New("source_url is required for job_site_company")
	}
	if source.TargetType == "job_listing" && strings.TrimSpace(source.SourceURL) == "" {
		return errors.New("source_url is required for job_listing")
	}
	if source.TargetType == "openwork_company" && strings.TrimSpace(source.SourceURL) == "" {
		return errors.New("source_url is required for openwork_company")
	}
	if source.ScheduleType != "weekly" && source.ScheduleType != "monthly" {
		return errors.New("schedule_type must be weekly or monthly")
	}
	if source.ScheduleType == "weekly" {
		if source.ScheduleDay < 0 || source.ScheduleDay > 6 {
			return errors.New("schedule_day must be 0-6 for weekly")
		}
	} else {
		if source.ScheduleDay < 1 || source.ScheduleDay > 31 {
			return errors.New("schedule_day must be 1-31 for monthly")
		}
	}
	if source.ScheduleTime == "" || !isValidTime(source.ScheduleTime) {
		return errors.New("schedule_time must be HH:MM")
	}
	return nil
}

func (s *CrawlService) executeCompanyCrawl(source *models.CrawlSource) error {
	if strings.TrimSpace(source.Name) == "" {
		return errors.New("company name is required for company crawl")
	}
	now := time.Now()
	company, err := s.companyRepo.FindByName(source.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            source.Name,
			SourceType:      source.SourceType,
			SourceURL:       source.SourceURL,
			SourceFetchedAt: &now,
			IsProvisional:   true,
			DataStatus:      "draft",
		}
		return s.companyRepo.Create(newCompany)
	}
	company.SourceType = source.SourceType
	company.SourceURL = source.SourceURL
	company.SourceFetchedAt = &now
	return s.companyRepo.Update(company)
}

type popularCompanyExtraction struct {
	Companies []struct {
		Name     string `json:"name"`
		Evidence string `json:"evidence"`
		Summary  string `json:"summary"`
		Rank     *int   `json:"rank,omitempty"`
	} `json:"companies"`
}

func (s *CrawlService) executePopularCompaniesCrawl(source *models.CrawlSource) error {
	if s.aiClient == nil {
		return errors.New("openai client is required for popular_companies crawl")
	}
	body, err := fetchText(source.SourceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("empty content from source_url")
	}

	extracted, err := s.extractPopularCompanies(source, body)
	if err != nil {
		return err
	}
	if len(extracted.Companies) == 0 {
		return errors.New("no companies extracted from source")
	}

	now := time.Now()
	for _, item := range extracted.Companies {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		company, err := s.companyRepo.FindByName(name)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			newCompany := &models.Company{
				Name:            name,
				SourceType:      source.SourceType,
				SourceURL:       source.SourceURL,
				SourceFetchedAt: &now,
				IsProvisional:   true,
				DataStatus:      "draft",
			}
			if err := s.companyRepo.Create(newCompany); err != nil {
				return err
			}
			company = newCompany
		}

		record := &models.CompanyPopularityRecord{
			CompanyID:    company.ID,
			SourceName:   source.Name,
			SourceURL:    source.SourceURL,
			EvidenceText: strings.TrimSpace(item.Evidence),
			Summary:      strings.TrimSpace(item.Summary),
			Rank:         item.Rank,
			FetchedAt:    now,
		}
		if err := s.popularRepo.Create(record); err != nil {
			return err
		}
	}
	return nil
}

type jobSiteCompanyExtraction struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Industry       string  `json:"industry"`
	EmployeeCount  int     `json:"employee_count"`
	FoundedYear    int     `json:"founded_year"`
	Location       string  `json:"location"`
	WebsiteURL     string  `json:"website_url"`
	Culture        string  `json:"culture"`
	WorkStyle      string  `json:"work_style"`
	WelfareDetails string  `json:"welfare_details"`
	MainBusiness   string  `json:"main_business"`
	AverageAge     float64 `json:"average_age"`
	FemaleRatio    float64 `json:"female_ratio"`
}

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
	case containsAny(label, []string{"業種", "業界"}):
		if data.Industry == "" {
			data.Industry = value
		}
	case containsAny(label, []string{"設立", "創業"}):
		if data.FoundedYear == 0 {
			data.FoundedYear = extractYear(value)
		}
	case containsAny(label, []string{"従業員", "社員数", "人数"}):
		if data.EmployeeCount == 0 {
			data.EmployeeCount = extractInt(value)
		}
	case containsAny(label, []string{"所在地", "本社"}):
		if data.Location == "" {
			data.Location = value
		}
	case containsAny(label, []string{"事業内容", "主要事業", "ビジネス"}):
		if data.MainBusiness == "" {
			data.MainBusiness = value
		}
	case containsAny(label, []string{"企業概要", "会社概要", "概要"}):
		if data.Description == "" {
			data.Description = value
		}
	case containsAny(label, []string{"URL", "ホームページ", "WEB", "Web"}):
		if data.WebsiteURL == "" {
			data.WebsiteURL = value
		}
	case containsAny(label, []string{"福利厚生", "待遇", "諸手当"}):
		if data.WelfareDetails == "" {
			data.WelfareDetails = value
		}
	case containsAny(label, []string{"社風", "企業文化", "カルチャー", "雰囲気"}):
		if data.Culture == "" {
			data.Culture = value
		}
	case containsAny(label, []string{"平均年齢"}):
		if data.AverageAge == 0 {
			data.AverageAge = extractFloat(value)
		}
	case containsAny(label, []string{"女性比率", "女性割合", "女性社員"}):
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

// fetchBytes URLからレスポンスのバイト列とContent-TypeのCharsetを返す
func fetchBytes(url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
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
	charset := extractCharset(ct)
	return body, charset, nil
}

// extractCharset Content-TypeヘッダーからCharsetを抽出する
func extractCharset(contentType string) string {
	lower := strings.ToLower(contentType)
	if strings.Contains(lower, "shift_jis") || strings.Contains(lower, "shift-jis") || strings.Contains(lower, "sjis") {
		return "shift_jis"
	}
	if strings.Contains(lower, "euc-jp") {
		return "euc-jp"
	}
	return "utf-8"
}

// decodeToUTF8 Shift-JIS/EUC-JPをUTF-8に変換する
func decodeToUTF8(b []byte, charset string) ([]byte, error) {
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

func (s *CrawlService) executeJobSiteCompanyCrawl(source *models.CrawlSource) error {
	if s.aiClient == nil {
		return errors.New("openai client is required for job_site_company crawl")
	}
	body, err := fetchText(source.SourceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("empty content from source_url")
	}

	extracted, err := s.extractJobSiteCompany(source, body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(extracted.Name) == "" {
		return errors.New("could not extract company name from source")
	}

	now := time.Now()
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
			WorkStyle:       extracted.WorkStyle,
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
	if extracted.WorkStyle != "" {
		company.WorkStyle = extracted.WorkStyle
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

type jobListingExtraction struct {
	CompanyName string `json:"company_name"`
	Positions   []struct {
		Title           string `json:"title"`
		Description     string `json:"description"`
		EmploymentType  string `json:"employment_type"`
		WorkLocation    string `json:"work_location"`
		RemoteOption    bool   `json:"remote_option"`
		MinSalary       int    `json:"min_salary"`
		MaxSalary       int    `json:"max_salary"`
		RequiredSkills  string `json:"required_skills"`
		PreferredSkills string `json:"preferred_skills"`
	} `json:"positions"`
}

func (s *CrawlService) executeJobListingCrawl(source *models.CrawlSource) error {
	if s.aiClient == nil {
		return errors.New("openai client is required for job_listing crawl")
	}
	body, err := fetchText(source.SourceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("empty content from source_url")
	}

	extracted, err := s.extractJobListings(source, body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(extracted.CompanyName) == "" {
		return errors.New("could not extract company name from source")
	}
	if len(extracted.Positions) == 0 {
		return errors.New("no job positions extracted from source")
	}

	now := time.Now()
	company, err := s.companyRepo.FindByName(extracted.CompanyName)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            extracted.CompanyName,
			SourceType:      source.SourceType,
			SourceURL:       source.SourceURL,
			SourceFetchedAt: &now,
			IsProvisional:   true,
			DataStatus:      "draft",
		}
		if err := s.companyRepo.Create(newCompany); err != nil {
			return err
		}
		company = newCompany
	}

	for _, p := range extracted.Positions {
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}
		existing, err := s.companyRepo.FindJobPositionByCompanyAndTitle(company.ID, title)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			pos := &models.CompanyJobPosition{
				CompanyID:       company.ID,
				Title:           title,
				Description:     p.Description,
				EmploymentType:  p.EmploymentType,
				WorkLocation:    p.WorkLocation,
				RemoteOption:    p.RemoteOption,
				MinSalary:       p.MinSalary,
				MaxSalary:       p.MaxSalary,
				RequiredSkills:  p.RequiredSkills,
				PreferredSkills: p.PreferredSkills,
				IsActive:        true,
			}
			if err := s.companyRepo.CreateJobPosition(pos); err != nil {
				return err
			}
		} else {
			if p.Description != "" {
				existing.Description = p.Description
			}
			if p.EmploymentType != "" {
				existing.EmploymentType = p.EmploymentType
			}
			if p.WorkLocation != "" {
				existing.WorkLocation = p.WorkLocation
			}
			existing.RemoteOption = p.RemoteOption
			if p.MinSalary > 0 {
				existing.MinSalary = p.MinSalary
			}
			if p.MaxSalary > 0 {
				existing.MaxSalary = p.MaxSalary
			}
			if p.RequiredSkills != "" {
				existing.RequiredSkills = p.RequiredSkills
			}
			if p.PreferredSkills != "" {
				existing.PreferredSkills = p.PreferredSkills
			}
			if err := s.companyRepo.UpdateJobPosition(existing); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *CrawlService) extractJobListings(source *models.CrawlSource, rawHTML string) (*jobListingExtraction, error) {
	clean := normalizeHTMLText(rawHTML)
	if len(clean) > 12000 {
		clean = clean[:12000]
	}
	systemPrompt := `You are a data extraction assistant. Extract job listing information from new graduate job site pages. Use only the provided text. Do not infer or guess values not present in the text.`
	userPrompt := fmt.Sprintf(`Extract company name and job positions from the job site page text below.
Return JSON with the following shape:
{
  "company_name": "会社名",
  "positions": [
    {
      "title": "職種名",
      "description": "仕事内容",
      "employment_type": "正社員",
      "work_location": "東京都",
      "remote_option": false,
      "min_salary": 300,
      "max_salary": 500,
      "required_skills": "[\"Java\",\"Spring Boot\"]",
      "preferred_skills": "[\"AWS\"]"
    }
  ]
}
Rules:
- Return 0 for salary fields not found in the text.
- Return "" for string fields not found in the text.
- required_skills and preferred_skills must be JSON arrays serialized as a string (e.g. "[\"Java\"]"), or "" if not found.
- min_salary and max_salary are annual salary in 万円 (integer).
- Do not fabricate data.

Text:
%s`, clean)

	content, err := s.aiClient.ChatCompletionJSON(context.Background(), systemPrompt, userPrompt, 0.2, 1200)
	if err != nil {
		return nil, err
	}
	var parsed jobListingExtraction
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (s *CrawlService) extractJobSiteCompany(source *models.CrawlSource, rawHTML string) (*jobSiteCompanyExtraction, error) {
	clean := normalizeHTMLText(rawHTML)
	if len(clean) > 12000 {
		clean = clean[:12000]
	}
	systemPrompt := `You are a data extraction assistant. Extract company information from new graduate job site pages. Use only the provided text. Do not infer or guess values not present in the text.`
	userPrompt := fmt.Sprintf(`Extract company information from the job site page text below.
Return JSON with the following shape:
{
  "name": "会社名",
  "description": "会社概要",
  "industry": "業界・業種",
  "employee_count": 1000,
  "founded_year": 2000,
  "location": "本社所在地",
  "website_url": "https://...",
  "culture": "企業文化・社風",
  "work_style": "リモート/ハイブリッド/オフィス",
  "welfare_details": "福利厚生",
  "main_business": "主要事業内容",
  "average_age": 32.5,
  "female_ratio": 40.0
}
Rules:
- Return 0 for numeric fields not found in the text.
- Return "" for string fields not found in the text.
- employee_count must be an integer.
- average_age and female_ratio must be floating-point numbers.
- Do not fabricate data.

Text:
%s`, clean)

	content, err := s.aiClient.ChatCompletionJSON(context.Background(), systemPrompt, userPrompt, 0.2, 800)
	if err != nil {
		return nil, err
	}
	var parsed jobSiteCompanyExtraction
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (s *CrawlService) extractPopularCompanies(source *models.CrawlSource, rawHTML string) (*popularCompanyExtraction, error) {
	clean := normalizeHTMLText(rawHTML)
	if len(clean) > 12000 {
		clean = clean[:12000]
	}
	systemPrompt := `You are a data extraction assistant. Use only the provided text. Do not infer or guess.`
	userPrompt := fmt.Sprintf(`Extract popular companies mentioned in the text below.
Return JSON with the following shape:
{
  "companies": [
    {
      "name": "Company Name",
      "evidence": "Exact excerpt from the text",
      "summary": "Why the company is described as popular, based only on the text",
      "rank": 1
    }
  ]
}
Rules:
- If rank is not shown, omit it or set it to null.
- evidence must be a verbatim excerpt from the text.
- summary must be a short, factual sentence based on the evidence only.

Text:
%s`, clean)

	content, err := s.aiClient.ChatCompletionJSON(context.Background(), systemPrompt, userPrompt, 0.2, 800)
	if err != nil {
		return nil, err
	}
	var parsed popularCompanyExtraction
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func fetchText(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func normalizeHTMLText(rawHTML string) string {
	clean := regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</\1>`).ReplaceAllString(rawHTML, " ")
	clean = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = strings.ReplaceAll(clean, "\u00a0", " ")
	clean = regexp.MustCompile(`\s+`).ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

func isValidTime(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}

func computeNextRun(now time.Time, source *models.CrawlSource) *time.Time {
	if source == nil {
		return nil
	}
	hourMin, err := time.Parse("15:04", source.ScheduleTime)
	if err != nil {
		return nil
	}
	hour := hourMin.Hour()
	min := hourMin.Minute()
	loc := now.Location()
	var next time.Time
	if source.ScheduleType == "weekly" {
		target := time.Weekday(source.ScheduleDay)
		days := (int(target) - int(now.Weekday()) + 7) % 7
		next = time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc).AddDate(0, 0, days)
		if !next.After(now) {
			next = next.AddDate(0, 0, 7)
		}
	} else {
		day := source.ScheduleDay
		year, month := now.Year(), now.Month()
		lastDay := lastDayOfMonth(year, month, loc)
		if day > lastDay {
			day = lastDay
		}
		next = time.Date(year, month, day, hour, min, 0, 0, loc)
		if !next.After(now) {
			nextMonth := next.AddDate(0, 1, 0)
			year, month = nextMonth.Year(), nextMonth.Month()
			day = source.ScheduleDay
			lastDay = lastDayOfMonth(year, month, loc)
			if day > lastDay {
				day = lastDay
			}
			next = time.Date(year, month, day, hour, min, 0, 0, loc)
		}
	}
	return &next
}

func lastDayOfMonth(year int, month time.Month, loc *time.Location) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
}
